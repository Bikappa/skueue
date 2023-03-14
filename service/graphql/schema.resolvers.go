package graphql

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.
// Code generated by github.com/99designs/gqlgen version v0.17.26

import (
	"context"

	"github.com/bikappa/arduino-jobs-manager/service/graphql/server"
	"github.com/bikappa/arduino-jobs-manager/service/jobservice/types"
)

// StartCompilation is the resolver for the startCompilation field.
func (r *mutationResolver) StartCompilation(ctx context.Context, compilationParameters types.CompilationParameters) (*types.EnqueuedCompilationJob, error) {
	userId := server.ContextIdentity(ctx)
	compilationParameters.UserID = userId
	return r.compilationService.EnqueueJob(ctx, compilationParameters)
}

// Compilation is the resolver for the compilation field.
func (r *queryResolver) Compilation(ctx context.Context, compilationID string) (*types.EnqueuedCompilationJob, error) {
	userId := server.ContextIdentity(ctx)
	return r.compilationService.GetJob(ctx, userId, compilationID)
}

// CompilationUpdate is the resolver for the compilationUpdate field.
func (r *subscriptionResolver) CompilationUpdate(ctx context.Context, compilationID string) (<-chan *types.JobUpdateMessage, error) {
	userId := server.ContextIdentity(ctx)

	return r.compilationService.UpdatesChannel(ctx, userId, compilationID)
}

// CompilationLog is the resolver for the compilationLog field.
func (r *subscriptionResolver) CompilationLog(ctx context.Context, compilationID string) (<-chan *types.LogLineMessage, error) {
	userId := server.ContextIdentity(ctx)
	return r.compilationService.LogsChannel(ctx, userId, compilationID)
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
