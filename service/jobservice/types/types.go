package types

type File struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

type LibraryDescription struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Sketch struct {
	Name     string          `json:"name"`
	Files    []File          `json:"files"`
	Ino      File            `json:"ino"`
	Metadata *SketchMetadata `json:"metadata"`
}

type SketchMetadata struct {
	IncludedLibs []LibraryDescription `json:"includedLibs"`
}

type MessageType string

const (
	LogLineMessageType      MessageType = "compilation_log_line"
	StatusUpdateMessageType MessageType = "compilation_status_update"
)

type ResourceType string

const (
	CompilationJobResourceType ResourceType = "CompilationJob"
)

type BaseMessage struct {
	ResourceType ResourceType
	MessageType  MessageType `json:"type"`
}

type JobUpdateMessage struct {
	BaseMessage
	CompilationJob *EnqueuedCompilationJob `json:"compilationJob"`
}

func NewJobUpdateMessage(job *EnqueuedCompilationJob) *JobUpdateMessage {
	return &JobUpdateMessage{
		BaseMessage: BaseMessage{
			ResourceType: CompilationJobResourceType,
			MessageType:  StatusUpdateMessageType,
		},
		CompilationJob: job,
	}
}

type LogLineMessage struct {
	BaseMessage
	Text string `json:"text"`
}

func NewLogLineMessage(compilationId string, text string) *LogLineMessage {
	return &LogLineMessage{
		BaseMessage: BaseMessage{
			ResourceType: CompilationJobResourceType,
			MessageType:  LogLineMessageType,
		},
		Text: text,
	}
}

type EnqueuedCompilationJob struct {
	ResourceType ResourceType `json:"resourceType"`
	ID           string       `json:"id"`
	QueuedAt     string       `json:"queuedAt"`
	CompletedAt  *string      `json:"completedAt"`
	Status       string       `json:"status"`
	ExpiresAt    *string      `json:"expiresAt"`
}

type CompilationParameters struct {
	FQBN    string
	UserID  string `json:"userId"`
	Sketch  Sketch `json:"sketch"`
	Ota     *bool  `json:"ota"`
	Verbose *bool  `json:"verbose"`
}
