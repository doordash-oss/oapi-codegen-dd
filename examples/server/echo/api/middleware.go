// Package api This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to add your middleware logic.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
//
// Echo provides many built-in middleware: middleware.Logger(), middleware.Recover(),
// middleware.RequestID(), middleware.CORS(), middleware.Timeout(), etc.
// See: https://echo.labstack.com/docs/middleware
//
// This file shows how to write custom middleware using echo.MiddlewareFunc.
package api

import (
	"log"

	"github.com/labstack/echo/v4"
)

// ExampleMiddleware demonstrates a custom echo.MiddlewareFunc.
// It logs before and after each request.
func ExampleMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			log.Printf("before: %s %s", c.Request().Method, c.Request().URL.Path)
			err := next(c)
			log.Printf("after: %s %s status=%d", c.Request().Method, c.Request().URL.Path, c.Response().Status)
			return err
		}
	}
}
