// Package main - This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to customize your server setup.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	handler "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/iris/api"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/middleware/logger"
	"github.com/kataras/iris/v12/middleware/recover"
	"github.com/kataras/iris/v12/middleware/requestid"
)

func main() {
	// Create Iris application
	app := iris.New()

	// Add Iris built-in middleware
	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(logger.New())

	// Add middleware from generated scaffold
	app.Use(handler.CORSMiddleware())
	app.Use(handler.ExampleMiddleware())

	// Create your service implementation
	svc := handler.NewService()

	// Register routes
	handler.NewRouter(app, svc)

	// Start server
	port := "8080"
	go func() {
		log.Printf("Starting server on :%s", port)
		if err := app.Listen(":" + port); err != nil {
			log.Printf("Server stopped: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
}
