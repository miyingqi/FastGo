package main

import (
	"github.com/miyingqi/FastGo"
)

func main() {
	app := FastGo.NewFastGo(":8080")
	CorsConfig := FastGo.NewCors()
	CorsConfig.
		SetAllowOrigins("*").
		SetAllowMethods("GET", "POST", "OPTIONS")
	app.AddMiddleware(CorsConfig)
	app.POST("/api/auth/send-verification-code", func(ctx *FastGo.Context) {
		ctx.SendJson(200, FastGo.FJ{"code": "200"})
	})
	err := app.Run()
	if err != nil {
		panic(err)
	}
}
