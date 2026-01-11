package main

import (
	"github.com/miyingqi/Fast-Go"
)

func main() {
	app := &FastGo.App{}
	route := FastGo.NewRouter()
	route.PUT("/1", func(c *FastGo.Context) {

	})
	route.GET("/idn/k", func(c *FastGo.Context) {

		c.SendString(200, "hello world")
	})
	route.GET("/idn/:id", func(c *FastGo.Context) {

	})
	route.GET("/idn/:id/:id/*h", func(c *FastGo.Context) {

	})
	app.UseRouter(route)
	err := app.Run(":8080")
	if err != nil {
		panic(err)
	}
}
