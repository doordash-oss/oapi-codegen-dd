// Package main - This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to customize your server setup.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
package main

import (
	"log"
	"time"

	handler "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/fasthttp/api"
	"github.com/valyala/fasthttp"
)

func main() {
	// Create your service implementation
	svc := handler.NewService()

	// Create the handler with middleware
	h := handler.Handler(svc,
		handler.WithMiddleware(handler.RecoveryMiddleware()),
		handler.WithMiddleware(handler.RequestIDMiddleware()),
		handler.WithMiddleware(handler.LoggingMiddleware()),
		handler.WithMiddleware(handler.CORSMiddleware()),
		handler.WithMiddleware(handler.TimeoutMiddleware(30*time.Second)),
		handler.WithMiddleware(handler.ExampleMiddleware()),
	)

	// Create fasthttp server
	server := &fasthttp.Server{
		Handler:      h,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Printf("Starting fasthttp server on :%d", 8080)
	if err := server.ListenAndServe(":8080"); err != nil {
		log.Fatal(err)
	}
}
