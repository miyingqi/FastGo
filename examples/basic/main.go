package main

import (
	"github.com/miyingqi/FastGo"
)

func main() {
	loger := FastGo.NewAsyncLoggerSP(FastGo.DEBUG)
	loger.SetLevel(FastGo.DEBUG)
	loger.Debug("Hello World")
	app := FastGo.NewFastGo("0.0.0.0:443")
	CorsConfig := FastGo.NewCors()
	CorsConfig.
		SetAllowOrigins("*").
		SetAllowMethods("GET", "POST", "OPTIONS")
	app.AddMiddleware(CorsConfig)
	app.POST("/api/auth/send-verification-code", func(ctx *FastGo.Context) {
		ctx.SendJson(200, FastGo.FJ{"code": "200"})
	})
	app.GET("/api/file/*file", func(ctx *FastGo.Context) {
		param := ctx.GetParam("file")
		ctx.SendString(200, param)
	})
	app.GET("/api/file/:file", func(ctx *FastGo.Context) {
		param := ctx.GetParam("file")
		ctx.SendString(200, param)
	})
	app.SetTLS("E:\\GO\\src\\FastGo\\config\\certificate.crt", "E:\\GO\\src\\FastGo\\config\\private.key")
	err := app.Run()
	if err != nil {
		panic(err)
	}
}
