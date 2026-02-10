// Package main - This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to customize your server setup.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
package main

import (
	"log"
	"time"

	handler "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/kratos/api"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
)

func main() {
	// Create Kratos HTTP server with built-in middleware
	// Kratos has built-in: recovery, logging, tracing, metrics, validation
	httpSrv := http.NewServer(
		http.Address(":8080"),
		http.Timeout(30*time.Second),
		http.Middleware(
			recovery.Recovery(),
			logging.Server(nil),
		),
	)

	// Create your service implementation
	svc := handler.NewService()

	// Register routes with custom middleware from the generated scaffold
	handler.RegisterRoutes(httpSrv, svc,
		handler.WithMiddleware(handler.ExampleMiddleware()),
	)

	// Create and run Kratos app
	app := kratos.New(
		kratos.Name("service"),
		kratos.Server(httpSrv),
	)

	log.Printf("Starting Kratos server on :%d", 8080)
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
