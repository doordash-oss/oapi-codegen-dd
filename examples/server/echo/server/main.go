// Package main - This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to customize your server setup.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	handler "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/echo/api"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// Create Echo instance
	e := echo.New()

	// Add Echo built-in middleware
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogMethod: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			log.Printf("%s %s %d", v.Method, v.URI, v.Status)
			return nil
		},
	}))
	e.Use(middleware.CORS())
	e.Use(middleware.ContextTimeoutWithConfig(middleware.ContextTimeoutConfig{
		Timeout: 30 * time.Second,
	}))

	// Add custom middleware from generated scaffold
	e.Use(handler.ExampleMiddleware())

	// Create your service implementation
	svc := handler.NewService()

	// Register routes
	handler.NewRouter(e, svc)

	// Start server
	port := "8080"
	go func() {
		log.Printf("Starting server on :%s", port)
		if err := e.Start(":" + port); err != nil {
			log.Printf("Server stopped: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
}
