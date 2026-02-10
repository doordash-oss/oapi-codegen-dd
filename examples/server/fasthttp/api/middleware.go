// Package api This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to add your middleware logic.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
package api

import (
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
)

// RecoveryMiddleware recovers from panics and returns a 500 error.
func RecoveryMiddleware() func(fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("panic recovered: %v", r)
					ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
				}
			}()
			next(ctx)
		}
	}
}

// RequestIDMiddleware adds a unique request ID to each request.
func RequestIDMiddleware() func(fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			requestID := string(ctx.Request.Header.Peek("X-Request-ID"))
			if requestID == "" {
				requestID = uuid.New().String()
			}
			ctx.Response.Header.Set("X-Request-ID", requestID)
			next(ctx)
		}
	}
}

// LoggingMiddleware logs request method, path, and response status.
func LoggingMiddleware() func(fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			start := time.Now()
			next(ctx)
			log.Printf("%s %s %d %v",
				string(ctx.Method()),
				string(ctx.Path()),
				ctx.Response.StatusCode(),
				time.Since(start),
			)
		}
	}
}

// CORSMiddleware adds CORS headers to responses.
func CORSMiddleware() func(fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
			ctx.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			ctx.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if string(ctx.Method()) == "OPTIONS" {
				ctx.SetStatusCode(fasthttp.StatusNoContent)
				return
			}
			next(ctx)
		}
	}
}

// TimeoutMiddleware adds a timeout to requests.
func TimeoutMiddleware(timeout time.Duration) func(fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return fasthttp.TimeoutHandler(next, timeout, "Request timeout")
	}
}

// ExampleMiddleware demonstrates a custom middleware.
// Replace this with your own middleware logic (authentication, rate limiting, etc.).
func ExampleMiddleware() func(fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			// Add custom logic here
			next(ctx)
		}
	}
}
