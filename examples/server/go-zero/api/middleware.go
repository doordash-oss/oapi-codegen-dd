// Package api This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to add your middleware logic.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
package api

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest"
)

// go-zero has built-in middleware for: recovery, logging, timeout, tracing,
// circuit breaker, rate limiting, load shedding, and more.
// Configure them via rest.RestConf or rest.WithXxx() options.
//
// This file is for your custom middleware only.

// ExampleMiddleware demonstrates a custom middleware for go-zero.
// Replace this with your own middleware logic (authentication, metrics, etc.).
func ExampleMiddleware() rest.Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Add custom logic here
			next(w, r)
		}
	}
}
