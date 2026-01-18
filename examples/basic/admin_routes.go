package main

import (
	"fmt"

	FastGo "github.com/miyingqi/FastGo"
)

// CreateAdminRoutes 创建管理员相关的路由
func CreateAdminRoutes() *FastGo.Router {
	router := FastGo.NewRouter()

	adminGroup := router.Group("/admin")
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
		adminGroup.GET("/settings", func(c *FastGo.Context) {
			c.SendString(200, "Admin Settings")
		})
	}

	return router
}
