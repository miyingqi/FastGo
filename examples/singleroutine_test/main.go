package main

import (
	"FastGo"
	"fmt"
	"time"
)

func main() {
	app := FastGo.NewFastGo()

	// 简单的路由测试
	app.Router().GET("/hello", func(c *FastGo.Context) {
		c.SendString(200, "Hello World!")
	})

	// JSON 响应测试
	app.Router().GET("/api/users", func(c *FastGo.Context) {
		users := FastGo.FJ{
			"users": []FastGo.FJ{
				{"id": 1, "name": "Alice"},
				{"id": 2, "name": "Bob"},
			},
			"timestamp": time.Now().Unix(),
		}
		c.SendJson(200, users)
	})

	// 测试中间件
	app.Use(FastGo.HandlerFunc(func(c *FastGo.Context) {
		fmt.Printf("Middleware: %s %s\n", c.Method(), c.Path())
		c.Next()
	}))

	fmt.Println("Single routine FastGo server starting on :8080")
	fmt.Println("Endpoints:")
	fmt.Println("  GET /hello        - Simple text response")
	fmt.Println("  GET /api/users    - JSON API response")

	app.Run(":8080")
}
