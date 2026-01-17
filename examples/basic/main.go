package main

import (
	"fmt"

	"github.com/miyingqi/FastGo"
)

func main() {
	app := FastGo.NewFastGo(":8080")

	// 使用路由组功能
	apiV1 := app.Router().Group("/api/v1")
	{
		apiV1.GET("/users", func(c *FastGo.Context) {
			c.SendString(200, "GET /api/v1/users")
		})
		apiV1.POST("/users", func(c *FastGo.Context) {
			c.SendString(200, "POST /api/v1/users")
		})
		apiV1.GET("/users/:id", func(c *FastGo.Context) {
			id := c.Params.ByName("id")
			c.SendString(200, fmt.Sprintf("GET /api/v1/users/%s", id))
		})
		apiV1.PUT("/users/:id", func(c *FastGo.Context) {
			id := c.Params.ByName("id")
			c.SendString(200, fmt.Sprintf("PUT /api/v1/users/%s", id))
		})
		apiV1.DELETE("/users/:id", func(c *FastGo.Context) {
			id := c.Params.ByName("id")
			c.SendString(200, fmt.Sprintf("DELETE /api/v1/users/%s", id))
		})
	}

	admin := app.Router().Group("/admin")
	{
		// 为admin组添加中间件
		admin.Use(func(c *FastGo.Context) {
			fmt.Println("Admin middleware executed")
			c.SetHeader("X-Middleware", "admin")
			c.Next()
		})

		admin.GET("/dashboard", func(c *FastGo.Context) {
			c.SendString(200, "Admin Dashboard")
		})
		admin.GET("/users", func(c *FastGo.Context) {
			c.SendString(200, "Admin Users Management")
		})
	}

	// 根路由
	app.Router().GET("/", func(c *FastGo.Context) {
		c.SendString(200, "Hello, FastGo!")
	})

	// 启动服务器
	fmt.Println("Server is starting on :8080")
	if err := app.Run(); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
