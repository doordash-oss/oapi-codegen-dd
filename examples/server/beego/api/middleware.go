// Package api This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to add your middleware logic.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
//
// Beego uses FilterFunc for middleware: func(ctx *context.Context)
// Filters are added via InsertFilter with positions: BeforeStatic, BeforeRouter, BeforeExec, AfterExec, FinishRouter
// See: https://beego.wiki/docs/mvc/controller/filter/
//
// This file shows how to write custom middleware using Beego's FilterFunc.
package api

import (
	"log"

	beecontext "github.com/beego/beego/v2/server/web/context"
)

// ExampleMiddleware demonstrates a custom Beego FilterFunc.
// It logs before each request.
func ExampleMiddleware() func(*beecontext.Context) {
	return func(ctx *beecontext.Context) {
		log.Printf("custom middleware: %s %s", ctx.Request.Method, ctx.Request.URL.Path)
	}
}
