// Package main - This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to customize your server setup.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	handler "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/echo-v5/api"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create Echo instance
	e := echo.New()

	// Add Echo built-in middleware
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogMethod: true,
		LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
			log.Printf("%s %s %d", v.Method, v.URI, v.Status)
			return nil
		},
	}))
	e.Use(middleware.CORS("*"))
	e.Use(middleware.ContextTimeoutWithConfig(middleware.ContextTimeoutConfig{
		Timeout: 30 * time.Second,
	}))

	// Add custom middleware from generated scaffold
	e.Use(handler.ExampleMiddleware())

	// Create your service implementation
	svc := handler.NewService()

	// Register routes
	handler.NewRouter(e, svc)

	// Start server with graceful shutdown
	sc := echo.StartConfig{
		HideBanner: true,
		Address:    ":8080",
		BeforeServeFunc: func(s *http.Server) error {
			s.ReadTimeout = 5 * time.Second
			s.WriteTimeout = 30 * time.Second
			s.IdleTimeout = 120 * time.Second
			return nil
		},
	}
	log.Printf("Starting server on :%d", 8080)
	if err := sc.Start(ctx, e); err != nil {
		log.Printf("Server stopped: %v", err)
	}
}
