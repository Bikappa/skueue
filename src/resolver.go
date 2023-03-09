package src

import (
	"github.com/bikappa/arduino-jobs-manager/src/jobservice"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	compilationService jobservice.CompilationService
}

func NewResolver(compilationService jobservice.CompilationService) (*Resolver, error) {
	return &Resolver{
		compilationService: compilationService,
	}, nil
}
