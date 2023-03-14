// Code generated by github.com/beyondan/gqlgenc, DO NOT EDIT.

package client

import (
	"context"

	"github.com/beyondan/gqlgenc/client"
	"github.com/beyondan/gqlgenc/client/transport"
)

type Client struct {
	Client *client.Client
}

// INPUT_OBJECT: CompilationParameters
type CompilationParameters struct {
	Fqbn    string "json:\"fqbn\""
	Sketch  Sketch "json:\"sketch\""
	Ota     *bool  "json:\"ota\""
	Verbose *bool  "json:\"verbose\""
}

// INPUT_OBJECT: File
type File struct {
	Name string "json:\"name\""
	Data string "json:\"data\""
}

// INPUT_OBJECT: LibraryDescription
type LibraryDescription struct {
	Name    string "json:\"name\""
	Version string "json:\"version\""
}

// INPUT_OBJECT: Sketch
type Sketch struct {
	Name     string          "json:\"name\""
	Files    []File          "json:\"files\""
	Ino      File            "json:\"ino\""
	Metadata *SketchMetadata "json:\"metadata\""
}

// INPUT_OBJECT: SketchMetadata
type SketchMetadata struct {
	IncludedLibs []LibraryDescription "json:\"includedLibs\""
}

// OPERATION: compilationLogSubscription
type CompilationLogSubscription struct {
	CompilationLog *CompilationLogSubscription_CompilationLog "json:\"compilationLog\""
}

// OPERATION: compilationLogSubscription.compilationLog
type CompilationLogSubscription_CompilationLog struct {
	ResourceType string "json:\"resourceType\""
	MessageType  string "json:\"messageType\""
	Text         string "json:\"text\""
}

// OPERATION: compilationUpdateSubscription
type CompilationUpdateSubscription struct {
	CompilationUpdate *CompilationUpdateSubscription_CompilationUpdate "json:\"compilationUpdate\""
}

// OPERATION: compilationUpdateSubscription.compilationUpdate
type CompilationUpdateSubscription_CompilationUpdate struct {
	ResourceType   string  "json:\"resourceType\""
	MessageType    string  "json:\"messageType\""
	CompilationJob JobInfo "json:\"compilationJob\""
}

// OPERATION: getCompilation
type GetCompilation struct {
	Compilation *JobInfo "json:\"compilation\""
}

// OBJECT: jobInfo
type JobInfo struct {
	ResourceType string  "json:\"resourceType\""
	ID           string  "json:\"id\""
	QueuedAt     string  "json:\"queuedAt\""
	Status       string  "json:\"status\""
	CompletedAt  *string "json:\"completedAt\""
	ExpiresAt    *string "json:\"expiresAt\""
}

// OPERATION: startCompilation
type StartCompilation struct {
	StartCompilation *JobInfo "json:\"startCompilation\""
}

// Pointer helpers
func SketchMetadataPtr(v SketchMetadata) *SketchMetadata {
	return &v
}
func BoolPtr(v bool) *bool {
	return &v
}

const StartCompilationDocument = `mutation startCompilation ($compilationParameters: CompilationParameters!) {
	startCompilation(compilationParameters: $compilationParameters) {
		... jobInfo
	}
}
fragment jobInfo on EnqueuedCompilationJob {
	resourceType
	id
	queuedAt
	status
	completedAt
	expiresAt
}
`

func (Ξc *Client) StartCompilation(ctх context.Context, compilationParameters CompilationParameters) (*StartCompilation, transport.OperationResponse, error) {
	Ξvars := map[string]interface{}{
		"compilationParameters": compilationParameters,
	}

	{
		var data StartCompilation
		res, err := Ξc.Client.Mutation(ctх, "startCompilation", StartCompilationDocument, Ξvars, &data)
		if err != nil {
			return nil, transport.OperationResponse{}, err
		}

		return &data, res, nil
	}
}

const GetCompilationDocument = `query getCompilation ($compilationId: String!) {
	compilation(compilationId: $compilationId) {
		... jobInfo
	}
}
fragment jobInfo on EnqueuedCompilationJob {
	resourceType
	id
	queuedAt
	status
	completedAt
	expiresAt
}
`

func (Ξc *Client) GetCompilation(ctх context.Context, compilationID string) (*GetCompilation, transport.OperationResponse, error) {
	Ξvars := map[string]interface{}{
		"compilationId": compilationID,
	}

	{
		var data GetCompilation
		res, err := Ξc.Client.Query(ctх, "getCompilation", GetCompilationDocument, Ξvars, &data)
		if err != nil {
			return nil, transport.OperationResponse{}, err
		}

		return &data, res, nil
	}
}

const CompilationLogSubscriptionDocument = `subscription compilationLogSubscription ($compilationId: String!) {
	compilationLog(compilationId: $compilationId) {
		resourceType
		messageType
		text
	}
}
`

type MessageCompilationLogSubscription struct {
	Data       *CompilationLogSubscription
	Error      error
	Extensions transport.RawExtensions
}

func (Ξc *Client) CompilationLogSubscription(ctх context.Context, compilationID string) (<-chan MessageCompilationLogSubscription, func()) {
	Ξvars := map[string]interface{}{
		"compilationId": compilationID,
	}

	{
		res := Ξc.Client.Subscription(ctх, "compilationLogSubscription", CompilationLogSubscriptionDocument, Ξvars)

		ch := make(chan MessageCompilationLogSubscription)

		go func() {
			for res.Next() {
				opres := res.Get()

				var msg MessageCompilationLogSubscription
				if len(opres.Errors) > 0 {
					msg.Error = opres.Errors
				}

				err := opres.UnmarshalData(&msg.Data)
				if err != nil && msg.Error == nil {
					msg.Error = err
				}

				msg.Extensions = opres.Extensions

				ch <- msg
			}

			if err := res.Err(); err != nil {
				ch <- MessageCompilationLogSubscription{
					Error: err,
				}
			}
			close(ch)
		}()

		return ch, res.Close
	}
}

const CompilationUpdateSubscriptionDocument = `subscription compilationUpdateSubscription ($compilationId: String!) {
	compilationUpdate(compilationId: $compilationId) {
		resourceType
		messageType
		compilationJob {
			... jobInfo
		}
	}
}
fragment jobInfo on EnqueuedCompilationJob {
	resourceType
	id
	queuedAt
	status
	completedAt
	expiresAt
}
`

type MessageCompilationUpdateSubscription struct {
	Data       *CompilationUpdateSubscription
	Error      error
	Extensions transport.RawExtensions
}

func (Ξc *Client) CompilationUpdateSubscription(ctх context.Context, compilationID string) (<-chan MessageCompilationUpdateSubscription, func()) {
	Ξvars := map[string]interface{}{
		"compilationId": compilationID,
	}

	{
		res := Ξc.Client.Subscription(ctх, "compilationUpdateSubscription", CompilationUpdateSubscriptionDocument, Ξvars)

		ch := make(chan MessageCompilationUpdateSubscription)

		go func() {
			for res.Next() {
				opres := res.Get()

				var msg MessageCompilationUpdateSubscription
				if len(opres.Errors) > 0 {
					msg.Error = opres.Errors
				}

				err := opres.UnmarshalData(&msg.Data)
				if err != nil && msg.Error == nil {
					msg.Error = err
				}

				msg.Extensions = opres.Extensions

				ch <- msg
			}

			if err := res.Err(); err != nil {
				ch <- MessageCompilationUpdateSubscription{
					Error: err,
				}
			}
			close(ch)
		}()

		return ch, res.Close
	}
}