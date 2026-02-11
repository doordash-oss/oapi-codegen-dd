// Package api This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to add your middleware logic.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
//
// Iris provides many built-in middleware: recover.New(), requestid.New(), logger.New(), etc.
// See: https://github.com/kataras/iris/tree/main/middleware
//
// This file shows how to write custom middleware using iris.Handler.
package api

import (
	"log"

	"github.com/kataras/iris/v12"
)

// ExampleMiddleware demonstrates a custom iris.Handler middleware.
// It logs before and after each request.
func ExampleMiddleware() iris.Handler {
	return func(ctx iris.Context) {
		log.Printf("before: %s %s", ctx.Method(), ctx.Path())
		ctx.Next()
		log.Printf("after: %s %s status=%d", ctx.Method(), ctx.Path(), ctx.GetStatusCode())
	}
}

// CORSMiddleware adds CORS headers to responses.
// Note: For production, consider using github.com/iris-contrib/middleware/cors
func CORSMiddleware() iris.Handler {
	return func(ctx iris.Context) {
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		ctx.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if ctx.Method() == "OPTIONS" {
			ctx.StatusCode(iris.StatusNoContent)
			return
		}
		ctx.Next()
	}
}
