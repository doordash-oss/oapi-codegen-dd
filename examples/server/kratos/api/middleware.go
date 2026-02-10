// Package api This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to add your middleware logic.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
package api

import (
	"net/http"
)

// Kratos has built-in middleware for: recovery, logging, tracing, metrics, validation.
// Configure them via http.Middleware() option when creating the server.
//
// This file is for your custom HTTP filters only.

// ExampleMiddleware demonstrates a custom HTTP filter for Kratos.
// Replace this with your own middleware logic (authentication, CORS, etc.).
// Kratos uses http.FilterFunc which wraps standard http.Handler.
func ExampleMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add custom logic here
			next.ServeHTTP(w, r)
		})
	}
}
