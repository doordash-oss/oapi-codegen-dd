// Package api This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to add your middleware logic.
// To regenerate, delete this file or set output.scaffold-once-overwrite: true in config.
//
// Gin provides many built-in middleware: gin.Logger(), gin.Recovery(),
// gin.BasicAuth(), etc.
// See: https://gin-gonic.com/docs/examples/custom-middleware/
//
// This file shows how to write custom middleware using gin.HandlerFunc.
package api

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// ExampleMiddleware demonstrates a custom gin.HandlerFunc middleware.
// It logs before and after each request.
func ExampleMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		log.Printf("before: %s %s", c.Request.Method, c.Request.URL.Path)

		c.Next()

		duration := time.Since(start)
		log.Printf("after: %s %s status=%d duration=%v", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), duration)
	}
}
