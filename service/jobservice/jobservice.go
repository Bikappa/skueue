package jobservice

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/bikappa/arduino-jobs-manager/service/jobservice/types"
	"github.com/google/uuid"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	typedbatchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type Environment struct {
	ID string
}

type JobVolume struct {
	SubPath   string
	ClaimName string
}

type CompilationService interface {
	EnqueueJob(context.Context, types.CompilationParameters) (*types.EnqueuedCompilationJob, error)
	GetJob(ctx context.Context, userId string, jobId string) (*types.EnqueuedCompilationJob, error)
	GetTarball(ctx context.Context, userId string, jobId string) (string, io.Reader, error)
	UpdatesChannel(ctx context.Context, userId string, jobId string) (<-chan *types.JobUpdateMessage, error)
	LogsChannel(ctx context.Context, userId string, jobId string) (<-chan *types.LogLineMessage, error)
}

type K8SJobServiceJobConfiguration struct {
	ImageTag                string
	TTLSecondsAfterFinished int32
	BuildVolume             JobVolume
}

type K8SJobServiceConfiguration struct {
	K8SRestClient   rest.Config
	K8SNamespace    string
	ServiceMetadata ServiceMetadata
	Jobs            K8SJobServiceJobConfiguration
	Environment     Environment

	// directory where compilation outputs can be find
	// one subdir per compilation id
	CompilationsOutputBaseDir string
}

type K8SCompilationService struct {
	config K8SJobServiceConfiguration

	k8sClientSet *kubernetes.Clientset

	// used to subscribe to job updates
	informerFactory informers.SharedInformerFactory
}

var _ CompilationService = (*K8SCompilationService)(nil)

func NewK8SCompilationService(config K8SJobServiceConfiguration) (*K8SCompilationService, error) {
	k8sClientSet, err := kubernetes.NewForConfig(&config.K8SRestClient)
	if err != nil {
		return &K8SCompilationService{}, err
	}
	factory := informers.NewSharedInformerFactoryWithOptions(k8sClientSet, time.Second*5, informers.WithNamespace(config.K8SNamespace))

	return &K8SCompilationService{
		config:          config,
		k8sClientSet:    k8sClientSet,
		informerFactory: factory,
	}, nil
}

func (s *K8SCompilationService) StartFactory(ctx context.Context) {
	go s.informerFactory.Start(ctx.Done())
}

func (s *K8SCompilationService) jobsClient() typedbatchv1.JobInterface {
	return s.k8sClientSet.BatchV1().Jobs(s.config.K8SNamespace)
}
func (s *K8SCompilationService) podsClient() typedcorev1.PodInterface {
	return s.k8sClientSet.CoreV1().Pods(s.config.K8SNamespace)
}

func (s *K8SCompilationService) configMapClient() typedcorev1.ConfigMapInterface {
	return s.k8sClientSet.CoreV1().ConfigMaps(s.config.K8SNamespace)
}

func (s *K8SCompilationService) EnqueueJob(ctx context.Context, params types.CompilationParameters) (*types.EnqueuedCompilationJob, error) {

	buildVolume := s.config.Jobs.BuildVolume
	jobId := uuid.New().String()
	files := params.Sketch.Files
	files = append(files, params.Sketch.Ino)

	libFullPaths := []string{}
	if params.Sketch.Metadata != nil {
		for _, il := range params.Sketch.Metadata.IncludedLibs {
			libFullPaths = append(libFullPaths, path.Join("/opt/arduino/directories-user/libraries", il.Name))
		}
	}
	
	compilationJob := CompilationOptions{
		ID:           jobId,
		SketchName:   params.Sketch.Name,
		Namespace:    s.config.K8SNamespace,
		Image:        s.config.Jobs.ImageTag,
		TTLSeconds:   s.config.Jobs.TTLSecondsAfterFinished,
		Service:      &s.config.ServiceMetadata,
		OutputVolume: &buildVolume,
		FQBN:         params.FQBN,
		UserID:       params.UserID,
		Files:        files,
		Libs:         libFullPaths,
		Environment:  &s.config.Environment,
	}
	if params.Verbose != nil {
		compilationJob.Verbose = *params.Verbose
	}
	configurationMap, err := compilationJob.CreateK8SConfigMap()
	if err != nil {
		return nil, err
	}
	job, err := compilationJob.CreateK8SJob()
	if err != nil {
		return nil, err
	}
	_, err = s.configMapClient().Create(ctx, configurationMap, v1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	job, err = s.jobsClient().Create(ctx, job, v1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return s.NewEnqueuedJobFromK8SJob(job), nil
}

func (s *K8SCompilationService) GetJob(ctx context.Context, userId string, jobId string) (*types.EnqueuedCompilationJob, error) {
	job, err := s.getUserJob(ctx, userId, jobId)
	if err != nil {
		return nil, err
	}

	return s.NewEnqueuedJobFromK8SJob(job), nil
}

func (s *K8SCompilationService) GetTarball(ctx context.Context, userId string, jobId string) (string, io.Reader, error) {
	job, err := s.getUserJob(ctx, userId, jobId)
	if err != nil {
		return "", nil, err
	}
	status := getJobStatus(job)
	if status != "succeeded" {
		return "", nil, fmt.Errorf("cannot get artefacts of unsuccessul or unfinished job")
	}
	// get the sketch name from the job annotations
	annotations := labels.Set(job.Annotations)
	sketchName := annotations.Get("sketchName")
	if sketchName == "" {
		fmt.Println("no sketch name")
	}
	compilationOutputPath := filepath.Join(s.config.CompilationsOutputBaseDir, jobId)
	entries, err := os.ReadDir(compilationOutputPath)
	if err != nil {
		return "", nil, fmt.Errorf("could not read output directory: %w", err)
	}

	var files []string

	filesWhitelist := map[string]interface{}{
		fmt.Sprintf("%s.ino.eep", sketchName):                 0,
		fmt.Sprintf("%s.ino.elf", sketchName):                 0,
		fmt.Sprintf("%s.ino.with_bootloader.bin", sketchName): 0,
		fmt.Sprintf("%s.ino.with_bootloader.hex", sketchName): 0,
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		_, isWhitelisted := filesWhitelist[e.Name()]
		if isWhitelisted {
			files = append(files, path.Join(compilationOutputPath, e.Name()))
		}
	}
	buf := bytes.NewBuffer([]byte{})
	tarW := tar.NewWriter(buf)
	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			// TODO: return what we can instead
			return "", nil, err
		}
		hdr := &tar.Header{
			Name: path.Base(file),
			Mode: 0600,
			Size: int64(len(body)),
		}
		if err := tarW.WriteHeader(hdr); err != nil {
			return "", nil, err
		}
		if _, err := tarW.Write(body); err != nil {
			return "", nil, err
		}
	}
	if err := tarW.Close(); err != nil {
		return "", nil, err
	}

	return fmt.Sprintf("%s.tar", sketchName), buf, nil
}

func (s *K8SCompilationService) getUserJob(ctx context.Context, userId string, jobId string) (*batchv1.Job, error) {
	res, err := s.jobsClient().List(ctx, v1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{
			"metadata.name": jobId,
		}).String(),
		Limit: 1,
	})
	if err != nil {
		return nil, err
	}
	if len(res.Items) == 0 {
		return nil, fmt.Errorf("not found")
	}

	if fields.Set(res.Items[0].Annotations).Get("userId") != userId {
		return nil, fmt.Errorf("not found")
	}
	return &res.Items[0], nil
}

func (s *K8SCompilationService) UpdatesChannel(ctx context.Context, userId string, jobId string) (<-chan *types.JobUpdateMessage, error) {

	// check if job exist before all
	// TODO: maybe use jobInformer.Lister()
	job, err := s.getUserJob(ctx, userId, jobId)

	if err != nil {
		return nil, err
	}
	jobInformer := s.informerFactory.Batch().V1().Jobs()
	outCh := make(chan *types.JobUpdateMessage)
	jobFinished := make(chan interface{})

	s.StartFactory(context.Background())
	if !cache.WaitForCacheSync(ctx.Done(), jobInformer.Informer().HasSynced) {
		return nil, fmt.Errorf("internal server error")
	}

	jobEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			filteredHandler(jobId)(obj)(func(j *batchv1.Job) {
				outCh <- types.NewJobUpdateMessage(s.NewEnqueuedJobFromK8SJob(j))
			})
		},
		UpdateFunc: func(_, obj interface{}) {
			filteredHandler(jobId)(obj)(func(j *batchv1.Job) {
				outCh <- types.NewJobUpdateMessage(s.NewEnqueuedJobFromK8SJob(j))
				if isFinished(j) {
					jobFinished <- true
				}
			})
		},
		DeleteFunc: func(obj interface{}) {
			filteredHandler(jobId)(obj)(func(j *batchv1.Job) {
				jobFinished <- true
			})
		},
	}

	handlerRegistration, err := jobInformer.Informer().AddEventHandler(jobEventHandler)
	if err != nil {
		return nil, err
	}

	go func() {
		// write first update
		select {
		case outCh <- types.NewJobUpdateMessage(s.NewEnqueuedJobFromK8SJob(job)):
		case <-ctx.Done():
		}
	}()

	go func() {
		// write first update
		outCh <- types.NewJobUpdateMessage(s.NewEnqueuedJobFromK8SJob(job))

		select {
		case <-jobFinished:
		case <-ctx.Done():
		}
		jobInformer.Informer().RemoveEventHandler(handlerRegistration)
		close(outCh)
	}()

	return outCh, nil
}

func (s *K8SCompilationService) LogsChannel(ctx context.Context, userId string, jobId string) (<-chan *types.LogLineMessage, error) {
	// check if job exists before all
	job, err := s.getUserJob(ctx, userId, jobId)
	if err != nil {
		return nil, err
	}
	podInformer := s.informerFactory.Core().V1().Pods()
	outCh := make(chan *types.LogLineMessage)
	logsFinished := make(chan interface{})

	s.StartFactory(context.Background())
	if !cache.WaitForCacheSync(ctx.Done(), podInformer.Informer().HasSynced) {
		return nil, fmt.Errorf("internal server error")
	}

	if isFinished(job) {
		// no update expected get the logs from the pod and close
		pods, err := podInformer.Lister().List(labels.SelectorFromSet(labels.Set{
			"job-name": jobId,
		}))
		if err != nil {
			return nil, err
		}

		if len(pods) == 0 {
			// should happen, if there's no pod then there's no job in first place
			return nil, fmt.Errorf("corresponding pod not found for finished job '%s'", jobId)
		}

		go func() {
			s.readLogs(ctx, pods[0], func(line string) {
				outCh <- types.NewLogLineMessage(jobId, line)
			})
			logsFinished <- true
		}()
		return outCh, nil
	}
	logsStarted := make(chan interface{})
	var handlerRegistration cache.ResourceEventHandlerRegistration
	podEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {},
		UpdateFunc: func(_, obj interface{}) {
			pod := obj.(*corev1.Pod)
			fmt.Println("Update pod event", pod.Status.Phase, pod.Labels, jobId)

			if pod.Labels["job-name"] != jobId {
				return
			}
			if pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodUnknown {
				return
			}
			logsStarted <- true
			go func() {
				s.readLogs(ctx, pod, func(line string) {
					select {
					case <-ctx.Done():
					case outCh <- types.NewLogLineMessage(jobId, line):
					}
				})
				logsFinished <- true
			}()
		},
		DeleteFunc: func(obj interface{}) {
			logsFinished <- true
		},
	}
	handlerRegistration, err = podInformer.Informer().AddEventHandler(&podEventHandler)
	if err != nil {
		return nil, err
	}

	go func() {
		done := false
		for !done {
			select {
			case <-logsStarted:
				podInformer.Informer().RemoveEventHandler(handlerRegistration)
			case <-logsFinished:
				done = true
			case <-ctx.Done():
				done = true
			}
		}
		close(outCh)
	}()

	return outCh, nil
}

type K8SJob struct {
	*batchv1.Job
}

func (s *K8SCompilationService) NewEnqueuedJobFromK8SJob(k8sJob *batchv1.Job) *types.EnqueuedCompilationJob {
	enqueued := types.EnqueuedCompilationJob{
		ResourceType: types.CompilationJobResourceType,
		ID:           k8sJob.Labels["compilationId"],
		Status:       getJobStatus(k8sJob),
		QueuedAt:     k8sJob.GetCreationTimestamp().Time.UTC().Format(time.RFC3339),
	}

	if k8sJob.Status.CompletionTime != nil {
		completionTime := k8sJob.Status.CompletionTime
		completedAt := completionTime.Format(time.RFC3339)
		enqueued.CompletedAt = &completedAt

		expiresAt := completionTime.Add(
			time.Second * time.Duration(s.config.Jobs.TTLSecondsAfterFinished).Abs(),
		).Format(time.RFC3339)
		enqueued.ExpiresAt = &expiresAt
	}

	return &enqueued
}

func (s *K8SCompilationService) readLogs(ctx context.Context, pod *corev1.Pod, onLine func(string)) error {
	req := s.podsClient().GetLogs(pod.Name, &corev1.PodLogOptions{
		Follow:    true,
		SinceTime: &pod.CreationTimestamp,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		onLine(scanner.Text())
	}
	return scanner.Err()
}

func getJobStatus(job *batchv1.Job) string {
	var status string

	if job.Status.Succeeded > 0 {
		status = "succeeded"
	} else if job.Status.Active > 0 || job.Status.UncountedTerminatedPods != nil &&
		len(job.Status.UncountedTerminatedPods.Succeeded)+len(job.Status.UncountedTerminatedPods.Failed) > 0 {
		status = "running"
	} else if job.Status.Failed > 0 {
		status = "failed"
	} else {
		status = "pending"
	}
	return status
}

func filteredHandler(jobId string) func(obj interface{}) func(filtered func(*batchv1.Job)) {
	return func(obj interface{}) func(filtered func(*batchv1.Job)) {
		job := obj.(*batchv1.Job)

		if job.Labels["compilationId"] != jobId {
			return func(filtered func(*batchv1.Job)) {}
		}
		return func(filtered func(*batchv1.Job)) {
			filtered(job)
		}
	}
}

func isFinished(j *batchv1.Job) bool {
	for _, c := range j.Status.Conditions {
		if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
