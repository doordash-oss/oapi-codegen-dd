// Package main - This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to customize your server setup.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
package main

import (
	"log"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"

	handler "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/goframe/api"
)

func main() {
	// Create your service implementation
	svc := handler.NewService()

	// Create GoFrame server
	s := ghttp.GetServer()

	// Apply middleware
	// GoFrame has built-in middleware for recovery, logging, and tracing
	// Add custom middleware
	s.Use(handler.ExampleMiddleware())

	// Register routes
	handler.NewRouter(s, svc)

	// Configure server
	port := 8080
	timeout := 30 * time.Second

	s.SetPort(port)
	s.SetReadTimeout(timeout)
	s.SetWriteTimeout(timeout)
	s.SetIdleTimeout(2 * timeout)

	log.Printf("Starting GoFrame server on :%d", port)
	s.Run()
}
