// Package main - This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to customize your server setup.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
package main

import (
	"context"
	"log"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	handler "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/hertz/api"
)

func main() {
	// Create your service implementation
	svc := handler.NewService()

	// Create Hertz server with options
	port := 8080
	timeout := 30 * time.Second

	h := server.Default(
		server.WithHostPorts(":8080"),
		server.WithReadTimeout(timeout),
		server.WithWriteTimeout(timeout),
		server.WithIdleTimeout(2*timeout),
	)

	// Apply middleware
	// Hertz Default() includes recovery middleware
	// Add request ID middleware
	h.Use(requestIDMiddleware())
	// Add logging middleware
	h.Use(loggingMiddleware())
	// Add CORS middleware
	h.Use(corsMiddleware())
	// Add timeout middleware
	h.Use(timeoutMiddleware(timeout))
	// Add custom middleware
	h.Use(handler.ExampleMiddleware())

	// Register routes
	handler.NewRouter(h, svc)

	log.Printf("Starting Hertz server on :%d", port)
	h.Spin()
}

// requestIDMiddleware adds a unique request ID to each request.
func requestIDMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		requestID := c.Request.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Response.Header.Set("X-Request-ID", requestID)
		c.Next(ctx)
	}
}

func generateRequestID() string {
	return time.Now().Format("20060102150405.000000")
}

// loggingMiddleware logs request method, path, and response status.
func loggingMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()
		c.Next(ctx)
		hlog.Infof("%s %s %d %v", string(c.Method()), string(c.Path()), c.Response.StatusCode(), time.Since(start))
	}
}

// corsMiddleware handles Cross-Origin Resource Sharing.
func corsMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.Response.Header.Set("Access-Control-Allow-Origin", "*")
		c.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		c.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")

		if string(c.Method()) == "OPTIONS" {
			c.AbortWithStatus(consts.StatusNoContent)
			return
		}
		c.Next(ctx)
	}
}

// timeoutMiddleware adds request timeout handling.
func timeoutMiddleware(timeout time.Duration) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		c.Next(ctx)
	}
}
