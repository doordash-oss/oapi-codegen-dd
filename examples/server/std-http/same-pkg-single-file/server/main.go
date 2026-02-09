// Package main - This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to customize your server setup.
// To regenerate, delete this file or set generate.overwrite-scaffolded: true in config.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	handler "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/stdhttp/same-pkg-single-file/api"
)

func main() {
	// Create your handler implementation
	h := handler.NewHandler()

	// Create router with all available middleware
	router := handler.NewRouter(h,
		handler.WithMiddleware(handler.RecoveryMiddleware),
		handler.WithMiddleware(handler.RequestIDMiddleware),
		handler.WithMiddleware(handler.LoggingMiddleware(log.Printf)),
		handler.WithMiddleware(handler.CORSMiddleware(handler.DefaultCORSConfig())),
		handler.WithMiddleware(handler.TimeoutMiddleware(30*time.Second)),
	)

	// Configure server
	port := 8080
	addr := fmt.Sprintf(":%d", port)

	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting server on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
}
