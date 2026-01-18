package main

import (
	"FastGo"
	"fmt"
)

func main() {
	app := FastGo.NewFastGo(":8080")

	// 创建基础API路由组
	api := app.Group("/api")
	{
		v1 := api.Group("/v1")
		{
			users := v1.Group("/users")
			{
				users.GET("/list", func(c *FastGo.Context) {
					c.SendString(200, "API v1 - Users List")
				})

				users.POST("/create", func(c *FastGo.Context) {
					c.SendString(200, "API v1 - Create User")
				})

				// 更深层的嵌套
				admin := users.Group("/admin")
				{
					admin.GET("/permissions", func(c *FastGo.Context) {
						c.SendString(200, "API v1 - Users Admin Permissions")
					})
				}
			}

			products := v1.Group("/products")
			{
				products.GET("/list", func(c *FastGo.Context) {
					c.SendString(200, "API v1 - Products List")
				})
			}
		}

		v2 := api.Group("/v2")
		{
			users := v2.Group("/users")
			{
				users.GET("/list", func(c *FastGo.Context) {
					c.SendString(200, "API v2 - Users List")
				})
			}
		}
	}

	// 另一个独立的路由组
	admin := app.Group("/admin")
	{
		dashboard := admin.Group("/dashboard")
		{
			dashboard.GET("", func(c *FastGo.Context) {
				c.SendString(200, "Admin Dashboard")
			})

			settings := dashboard.Group("/settings")
			{
				settings.GET("", func(c *FastGo.Context) {
					c.SendString(200, "Admin Settings")
				})
			}
		}
	}

	fmt.Println("服务器启动在 :8080 端口")
	fmt.Println("可访问以下路径:")
	fmt.Println("  GET  /api/v1/users/list")
	fmt.Println("  POST /api/v1/users/create")
	fmt.Println("  GET  /api/v1/users/admin/permissions")
	fmt.Println("  GET  /api/v1/products/list")
	fmt.Println("  GET  /api/v2/users/list")
	fmt.Println("  GET  /admin/dashboard")
	fmt.Println("  GET  /admin/dashboard/settings")

	// 注意：在实际应用中，我们不会调用 app.Run() 因为这会启动HTTP服务器
	// 这里只是为了展示嵌套分组功能
}
