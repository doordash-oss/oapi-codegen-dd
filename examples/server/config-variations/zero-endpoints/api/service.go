// Package api This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to add your business logic.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
package api

// Service implements the ServiceInterface.
// Add your dependencies here (database, clients, etc.)
type Service struct {
}

// NewService creates a new Service.
func NewService() *Service {
	return &Service{}
}

// Ensure Service implements ServiceInterface.
var _ ServiceInterface = (*Service)(nil)
