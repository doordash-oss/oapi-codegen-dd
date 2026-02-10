// Package api This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to add your middleware logic.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
//
// GoFrame provides built-in middleware for common tasks:
// - Recovery: Built into the framework
// - Logging: Built into the framework (ghttp.MiddlewareHandlerResponse)
// - Tracing: Built into the framework (OpenTelemetry integration)
// - CORS: Use ghttp.MiddlewareCORS
//
// This file shows how to write custom middleware using ghttp.HandlerFunc.
package api

import (
	"log"

	"github.com/gogf/gf/v2/net/ghttp"
)

// ExampleMiddleware demonstrates a custom ghttp.HandlerFunc middleware.
// It logs before and after each request.
func ExampleMiddleware() ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		log.Printf("before: %s %s", r.Method, r.URL.Path)
		r.Middleware.Next()
		log.Printf("after: %s %s status=%d", r.Method, r.URL.Path, r.Response.Status)
	}
}
