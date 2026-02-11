// Package main - This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to customize your server setup.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	handler "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/fiber/api"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

func main() {
	// Create Fiber app
	app := fiber.New(fiber.Config{
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  2 * 30 * time.Second,
	})

	// Add Fiber built-in middleware
	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(logger.New())

	// Add custom middleware from generated scaffold
	app.Use(handler.ExampleMiddleware())

	// Create your service implementation
	svc := handler.NewService()

	// Register routes
	handler.NewRouter(app, svc)

	// Configure server
	port := 8080
	addr := fmt.Sprintf(":%d", port)

	// Start server in goroutine
	go func() {
		log.Printf("Starting server on %s", addr)
		if err := app.Listen(addr); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	_ = app.Shutdown()
}
