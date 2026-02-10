// Package api This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to add your middleware logic.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
//
// Hertz provides built-in middleware for common tasks:
// - Recovery: Built into server.Default()
// - Logging: Use hlog package
// - Tracing: OpenTelemetry integration available
// - CORS: Can be added via middleware
//
// This file shows how to write custom middleware using app.HandlerFunc.
package api

import (
	"context"
	"log"

	"github.com/cloudwego/hertz/pkg/app"
)

// ExampleMiddleware demonstrates a custom app.HandlerFunc middleware.
// It logs before and after each request.
func ExampleMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		log.Printf("before: %s %s", string(c.Method()), string(c.Path()))
		c.Next(ctx)
		log.Printf("after: %s %s status=%d", string(c.Method()), string(c.Path()), c.Response.StatusCode())
	}
}
