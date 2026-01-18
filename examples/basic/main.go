package main

import (
	"fmt"

	"github.com/miyingqi/FastGo"
)

func main() {
	app := FastGo.NewFastGo(":8080")

	// 创建独立的路由器
	userRouter := FastGo.NewRouter()
	userGroup := userRouter.Group("/users")
	{
		userGroup.GET("", func(c *FastGo.Context) {
			c.SendString(200, "GET /users")
		})
		userGroup.POST("", func(c *FastGo.Context) {
			c.SendString(200, "POST /users")
		})
		userGroup.GET("/:id", func(c *FastGo.Context) {
			id := c.Params.ByName("id")
			c.SendString(200, fmt.Sprintf("GET /users/%s", id))
		})
		userGroup.PUT("/:id", func(c *FastGo.Context) {
			id := c.Params.ByName("id")
			c.SendString(200, fmt.Sprintf("PUT /users/%s", id))
		})
		userGroup.DELETE("/:id", func(c *FastGo.Context) {
			id := c.Params.ByName("id")
			c.SendString(200, fmt.Sprintf("DELETE /users/%s", id))
		})
	}

	// 使用AddRouter将独立路由器合并到主应用
	app.AddRouter(userRouter)

	// 使用Group方法创建路由组
	adminGroup := app.Group("/admin")
	{
		// 为admin组添加中间件
		adminGroup.Use(func(c *FastGo.Context) {
			fmt.Println("Admin middleware executed")
			c.SetHeader("X-Middleware", "admin")
			c.Next()
		})

		adminGroup.GET("/dashboard", func(c *FastGo.Context) {
			c.SendString(200, "Admin Dashboard")
		})
		adminGroup.GET("/users", func(c *FastGo.Context) {
			c.SendString(200, "Admin Users Management")
		})
	}

	// 添加其他路由
	apiGroup := app.Group("/api/v1")
	{
		apiGroup.GET("/ping", func(c *FastGo.Context) {
			c.SendString(200, "API v1 Ping")
		})
		apiGroup.GET("/status", func(c *FastGo.Context) {
			c.SendJson(200, FastGo.FJ{
				"status": "ok",
				"server": "FastGo",
			})
		})
	}

	if err := app.Run(); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
