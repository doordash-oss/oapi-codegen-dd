// Package main - This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to customize your server setup.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
package main

import (
	"log"
	"time"

	handler "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/go-zero/api"
	"github.com/zeromicro/go-zero/rest"
)

func main() {
	// Configure go-zero server
	// go-zero has built-in: recovery, logging, timeout, tracing, circuit breaker,
	// rate limiting, load shedding. Configure via RestConf or rest.WithXxx() options.
	c := rest.RestConf{
		Host:    "0.0.0.0",
		Port:    8080,
		Timeout: 30 * int64(time.Second/time.Millisecond), // in milliseconds
	}

	server := rest.MustNewServer(c,
		rest.WithCors("*"),
	)
	defer server.Stop()

	// Add custom middleware from the generated scaffold
	server.Use(handler.ExampleMiddleware())

	// Create your service implementation
	svc := handler.NewService()

	// Register routes
	handler.RegisterRoutes(server, svc)

	log.Printf("Starting go-zero server on :%d", 8080)
	server.Start()
}
