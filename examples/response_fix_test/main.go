package main

import (
	"FastGo"
	"fmt"
	"net/http"
	"time"
)

func main() {
	app := FastGo.NewFastGo()

	// 测试基本的 JSON 响应
	app.Router().GET("/users", func(c *FastGo.Context) {
		users := FastGo.FJ{
			"users": []FastGo.FJ{
				{"id": 1, "name": "Alice"},
				{"id": 2, "name": "Bob"},
				{"id": 3, "name": "Charlie"},
			},
			"total":     3,
			"timestamp": time.Now().Unix(),
		}
		c.SendJson(200, users)
	})

	// 测试 HTML 响应
	app.Router().GET("/html", func(c *FastGo.Context) {
		html := `
<!DOCTYPE html>
<html>
<head>
    <title>Test Page</title>
</head>
<body>
    <h1>Hello World!</h1>
    <p>This is a test page to verify the fix for WriteHeader conflicts.</p>
</body>
</html>`
		c.SendHtml(200, html)
	})

	// 测试字符串响应
	app.Router().GET("/text", func(c *FastGo.Context) {
		c.SendString(200, "This is a plain text response")
	})

	// 测试重定向
	app.Router().GET("/redirect", func(c *FastGo.Context) {
		c.Redirect(http.StatusFound, "/users")
	})

	// 测试并发写入
	app.Router().GET("/concurrent", func(c *FastGo.Context) {
		// 这应该会被所有权检查拦截
		go func() {
			c.SendString(200, "This should be blocked")
		}()

		// 主协程的正常响应
		c.SendString(200, "Main response processed correctly")
	})

	fmt.Println("Server starting on :8080")
	fmt.Println("Test endpoints:")
	fmt.Println("  GET /users      - JSON response test")
	fmt.Println("  GET /html       - HTML response test")
	fmt.Println("  GET /text       - Plain text response test")
	fmt.Println("  GET /redirect   - Redirect test")
	fmt.Println("  GET /concurrent - Concurrent access test")

	app.Run(":8080")
}
