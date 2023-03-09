package jobservice

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/bikappa/arduino-jobs-manager/src/jobservice/types"
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

type EnqueuedCompilationJob struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	QueuedAt    string `json:"queuedAt"`
	CompletedAt string `json:"completedAt"`
	Status      string `json:"status"`
}

type CompilationParams struct {
	FQBN   string
	UserID string
	Sketch types.Sketch
}

type CompilationService interface {
	EnqueueJob(context.Context, CompilationParams) (EnqueuedCompilationJob, error)
	GetJob(ctx context.Context, userId string, jobId string) (EnqueuedCompilationJob, error)
	GetTarball(ctx context.Context, userId string, jobId string) (string, io.Reader, error)
	UpdatesChannel(ctx context.Context, userId string, jobId string) (<-chan *JobUpdateEvent, error)
	LogsChannel(ctx context.Context, userId string, jobId string) (<-chan *LogEvent, error)
}

type K8SJobServiceJobConfiguration struct {
	ImageTag                string
	TTLSecondsAfterFinished int32
	LibraryVolume           JobVolume
	BuildVolume             JobVolume
}

type K8SJobServiceConfiguration struct {
	K8SRestClient   rest.Config
	K8SNamespace    string
	ServiceMetadata ServiceMetadata
	Jobs            K8SJobServiceJobConfiguration
	Environment     Environment
}

type K8SCompilationService struct {
	config       K8SJobServiceConfiguration
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

func (s *K8SCompilationService) EnqueueJob(ctx context.Context, params CompilationParams) (EnqueuedCompilationJob, error) {

	buildVolume := s.config.Jobs.BuildVolume
	jobId := uuid.New().String()
	buildVolume.SubPath = filepath.Join(buildVolume.SubPath, jobId)
	files := params.Sketch.Files
	files = append(files, params.Sketch.Ino)
	compilationJob := CompilationOptions{
		ID:            jobId,
		SketchName:    params.Sketch.Name,
		Namespace:     s.config.K8SNamespace,
		Image:         s.config.Jobs.ImageTag,
		TTLSeconds:    s.config.Jobs.TTLSecondsAfterFinished,
		Service:       &s.config.ServiceMetadata,
		LibraryVolume: &s.config.Jobs.LibraryVolume,
		OutputVolume:  &buildVolume,
		FQBN:          params.FQBN,
		UserID:        params.UserID,
		Files:         files,
		Environment:   &s.config.Environment,
	}
	configurationMap, err := compilationJob.CreateK8SConfigMap()
	if err != nil {
		return EnqueuedCompilationJob{}, err
	}
	job, err := compilationJob.CreateK8SJob()
	if err != nil {
		return EnqueuedCompilationJob{}, err
	}
	_, err = s.configMapClient().Create(ctx, configurationMap, v1.CreateOptions{})
	if err != nil {
		return EnqueuedCompilationJob{}, err
	}
	job, err = s.jobsClient().Create(ctx, job, v1.CreateOptions{})
	if err != nil {
		return EnqueuedCompilationJob{}, err
	}

	return NewEnqueuedJobFromK8SJob(job), nil
}

func (s *K8SCompilationService) GetJob(ctx context.Context, userId string, jobId string) (EnqueuedCompilationJob, error) {
	job, err := s.getUserJob(ctx, userId, jobId)
	if err != nil {
		return EnqueuedCompilationJob{}, err
	}

	return NewEnqueuedJobFromK8SJob(job), nil
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
		return "", nil, fmt.Errorf("internal server error")
	}

	compilationOutputPath := "/Users/Bikappa/workspace/testdata/Blink/build"
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
		body, err := ioutil.ReadFile(file)
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

func (s *K8SCompilationService) UpdatesChannel(ctx context.Context, userId string, jobId string) (<-chan *JobUpdateEvent, error) {

	// check if job exist before all
	_, err := s.getUserJob(ctx, userId, jobId)

	if err != nil {
		return nil, err
	}
	jobInformer := s.informerFactory.Batch().V1().Jobs()
	outCh := make(chan *JobUpdateEvent)
	jobFinished := make(chan interface{})

	s.StartFactory(context.Background())
	if !cache.WaitForCacheSync(ctx.Done(), jobInformer.Informer().HasSynced) {
		return nil, fmt.Errorf("internal server error")
	}

	jobEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			filteredHandler(jobId)(obj)(func(j *batchv1.Job) {
				outCh <- NewJobUpdateEvent(NewEnqueuedJobFromK8SJob(j))
			})
		},
		UpdateFunc: func(_, obj interface{}) {
			filteredHandler(jobId)(obj)(func(j *batchv1.Job) {
				outCh <- NewJobUpdateEvent(NewEnqueuedJobFromK8SJob(j))
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
		select {
		case <-jobFinished:
		case <-ctx.Done():
		}
		jobInformer.Informer().RemoveEventHandler(handlerRegistration)
		close(outCh)
	}()

	return outCh, nil
}

func (s *K8SCompilationService) LogsChannel(ctx context.Context, userId string, jobId string) (<-chan *LogEvent, error) {
	// check if job exists before all
	_, err := s.getUserJob(ctx, userId, jobId)
	if err != nil {
		return nil, err
	}
	podInformer := s.informerFactory.Core().V1().Pods()
	outCh := make(chan *LogEvent)
	logsFinished := make(chan interface{})

	s.StartFactory(context.Background())
	fmt.Println("waiting cache sync")
	if !cache.WaitForCacheSync(ctx.Done(), podInformer.Informer().HasSynced) {
		return nil, fmt.Errorf("internal server error")
	}
	fmt.Println("cache synced")

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
				s.readLogs(ctx, pod.Name, func(line string) {
					outCh <- NewLogEvent(jobId, line)
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

type WatchEventType string

const (
	LogWatchEventType         WatchEventType = "log"
	PhaseChangeWatchEventType WatchEventType = "phase_change"
)

type BaseWatchEvent struct {
	Type          WatchEventType `json:"type"`
	CompilationID string         `json:"compilationId"`
}

type JobUpdateEvent struct {
	BaseWatchEvent
	Job EnqueuedCompilationJob `json:"job"`
}

func NewJobUpdateEvent(job EnqueuedCompilationJob) *JobUpdateEvent {
	return &JobUpdateEvent{
		BaseWatchEvent: BaseWatchEvent{
			CompilationID: job.ID,
			Type:          PhaseChangeWatchEventType,
		},
		Job: job,
	}
}

type LogEvent struct {
	BaseWatchEvent
	Text string `json:"text"`
}

func NewLogEvent(compilationId string, text string) *LogEvent {
	return &LogEvent{
		BaseWatchEvent: BaseWatchEvent{
			CompilationID: compilationId,
			Type:          LogWatchEventType,
		},
		Text: text,
	}
}

type K8SJob struct {
	*batchv1.Job
}

func NewEnqueuedJobFromK8SJob(k8sJob *batchv1.Job) EnqueuedCompilationJob {
	enqueued := EnqueuedCompilationJob{
		Type:   "CompilationJob",
		ID:     k8sJob.Labels["compilationId"],
		Status: getJobStatus(k8sJob),

		QueuedAt: k8sJob.GetCreationTimestamp().Time.UTC().Format(time.RFC3339),
	}

	if k8sJob.Status.CompletionTime != nil {
		enqueued.CompletedAt = k8sJob.Status.CompletionTime.Format(time.RFC3339)
	}

	return enqueued
}

func (s *K8SCompilationService) readLogs(ctx context.Context, podName string, onLine func(string)) error {
	fmt.Println("starting log watch")

	req := s.podsClient().GetLogs(podName, &corev1.PodLogOptions{
		Follow: true,
	})
	stream, err := req.Stream(ctx)
	if err != nil {
		return err
	}

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
