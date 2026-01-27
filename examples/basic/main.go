package main

import (
	"fmt"

	"github.com/miyingqi/FastGo"
)

func main() {
	app := FastGo.NewFastGo()

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

	// 演示嵌套分组功能
	apiV2Group := app.Group("/api")
	{
		v2Group := apiV2Group.Group("/v2") // 嵌套分组
		{
			usersV2Group := v2Group.Group("/users") // 更深层嵌套
			{
				usersV2Group.GET("/list", func(c *FastGo.Context) {
					c.SendString(200, "API v2 - Users List")
				})

				usersV2Group.POST("/create", func(c *FastGo.Context) {
					c.SendString(200, "API v2 - Create User")
				})

				// 更深层的嵌套
				adminV2Group := usersV2Group.Group("/admin") // 三层嵌套
				{
					adminV2Group.GET("/permissions", func(c *FastGo.Context) {
						c.SendString(200, "API v2 - Users Admin Permissions")
					})

					adminV2Group.POST("/roles", func(c *FastGo.Context) {
						c.SendString(200, "API v2 - Set User Roles")
					})
				}
			}

			// 同级的其他分组
			productsV2Group := v2Group.Group("/products")
			{
				productsV2Group.GET("/list", func(c *FastGo.Context) {
					c.SendString(200, "API v2 - Products List")
				})

				productsV2Group.POST("/create", func(c *FastGo.Context) {
					c.SendString(200, "API v2 - Create Product")
				})
			}
		}
	}

	// 另一个嵌套示例 - 用户面板
	userPanelGroup := app.Group("/user")
	{
		dashboardGroup := userPanelGroup.Group("/dashboard") // 嵌套
		{
			settingsGroup := dashboardGroup.Group("/settings") // 再次嵌套
			{
				settingsGroup.GET("", func(c *FastGo.Context) {
					c.SendString(200, "User Dashboard Settings")
				})

				profileGroup := settingsGroup.Group("/profile") // 三层嵌套
				{
					profileGroup.GET("", func(c *FastGo.Context) {
						c.SendString(200, "User Profile Settings")
					})

					profileGroup.PUT("", func(c *FastGo.Context) {
						c.SendString(200, "Update User Profile")
					})
				}
			}
		}
	}

	if err := app.Run(":8080"); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
