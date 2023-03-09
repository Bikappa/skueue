package src

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"

	"github.com/bikappa/arduino-jobs-manager/src/jobservice"
	"github.com/bikappa/arduino-jobs-manager/src/jobservice/types"
	"github.com/bikappa/arduino-jobs-manager/src/model"
	"github.com/bikappa/arduino-jobs-manager/src/server"
)

func (r *mutationResolver) StartCompilation(ctx context.Context, fqbn string, sketch types.Sketch, ota *bool, verbose *bool) (*model.CompilationStatus, error) {
	userId := server.ContextIdentity(ctx)
	enqueuedJob, err := r.compilationService.EnqueueJob(ctx, jobservice.CompilationParams{
		FQBN:   fqbn,
		UserID: userId,
		Sketch: sketch,
	})

	if err != nil {
		return nil, err
	}
	return &model.CompilationStatus{
		ID: &enqueuedJob.ID,
	}, nil
}

func (r *queryResolver) Compilation(ctx context.Context, compilationID string) (*model.CompilationStatus, error) {
	userId := server.ContextIdentity(ctx)

	job, err := r.compilationService.GetJob(ctx, userId, compilationID)
	if err != nil {
		return nil, err
	}
	return &model.CompilationStatus{
		ID: &job.ID,
	}, nil
}

func (r *subscriptionResolver) CompilationUpdate(ctx context.Context, compilationID string) (<-chan *model.CompilationUpdate, error) {
	userId := server.ContextIdentity(ctx)

	eventsCh, err := r.compilationService.UpdatesChannel(ctx, userId, compilationID)
	if err != nil {
		return nil, err
	}
	updatesCh := make(chan *model.CompilationUpdate)

	go func() {
		defer close(updatesCh)
		for e := range eventsCh {
			updatesCh <- &model.CompilationUpdate{
				CompilationID: compilationID,
				State:         e.Job.Status,
			}
		}
	}()
	return updatesCh, nil
}

func (r *subscriptionResolver) CompilationLog(ctx context.Context, compilationID string) (<-chan *model.CompilationLog, error) {
	userId := server.ContextIdentity(ctx)
	logsChannel, err := r.compilationService.LogsChannel(ctx, userId, compilationID)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	externalLogsChannel := make(chan *model.CompilationLog)
	go func() {
		defer close(externalLogsChannel)
		for e := range logsChannel {
			externalLogsChannel <- &model.CompilationLog{
				CompilationID: compilationID,
				Text:          e.Text,
			}
		}
	}()
	return externalLogsChannel, nil
}

// Mutation returns server.MutationResolver implementation.
func (r *Resolver) Mutation() server.MutationResolver { return &mutationResolver{r} }

// Query returns server.QueryResolver implementation.
func (r *Resolver) Query() server.QueryResolver { return &queryResolver{r} }

// Subscription returns server.SubscriptionResolver implementation.
func (r *Resolver) Subscription() server.SubscriptionResolver { return &subscriptionResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type subscriptionResolver struct{ *Resolver }
