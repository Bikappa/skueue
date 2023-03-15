package jobservice

import (
	"bytes"
	"html/template"

	_ "embed"

	"github.com/Masterminds/sprig/v3"
	"github.com/bikappa/arduino-jobs-manager/service/jobservice/types"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sTypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

//go:embed templates/compilation-job.helm
var compilationJobTemplateSource string

var compilationJobTemplate *template.Template = template.Must(
	template.New("compilationJob").Funcs(sprig.FuncMap()).Parse(compilationJobTemplateSource))

type Compilation struct {
	FQBN   string
	UserID string
}

type ServiceMetadata struct {
	ID      string
	Version string
}

type CompilationOptions struct {
	ID           string
	SketchName   string
	Namespace    string
	Image        string
	Environment  *Environment
	OutputVolume *JobVolume
	FQBN         string
	UserID       string
	Service      *ServiceMetadata
	TTLSeconds   int32
	Files        []types.File
	Libs         []string
	Verbose      bool
}

func (j *CompilationOptions) CreateK8SJob() (*batchv1.Job, error) {
	rendered := bytes.NewBuffer([]byte{})
	err := compilationJobTemplate.Execute(rendered, j)
	if err != nil {
		return nil, err
	}
	job := &batchv1.Job{}
	err = yaml.Unmarshal(rendered.Bytes(), job)
	job.UID = k8sTypes.UID(j.ID)
	return job, err
}

func (j *CompilationOptions) CreateK8SConfigMap() (*corev1.ConfigMap, error) {
	data := map[string]string{}
	for _, f := range j.Files {
		data[f.Name] = f.Data
	}

	//TODO: make it from a template too
	return &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name: j.ID,
			Labels: map[string]string{
				"serviceId":      j.Service.ID,
				"serviceVersion": j.Service.Version,
				"compilationId":  j.ID,
			},
		},
		Data: data,
	}, nil
}
