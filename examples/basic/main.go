package main

import (
	"fmt"
	"time"

	"github.com/miyingqi/FastGo"
	_ "github.com/miyingqi/FastGo/examples/basic/docs"
	"github.com/miyingqi/FastGoMid"
)

// GetUsers 获取所有用户
// @Summary 获取所有用户
// @Description 获取系统中的所有用户列表
// @Tags 用户管理
// @Accept json
// @Produce json
// @Success 200 {string} string "成功"
// @Router /users [get]
func GetUsers(c *FastGo.Context) {
	time.Sleep(3 * time.Second)
	c.SendString(200, "GET /users")
}

// CreateUser 创建新用户
// @Summary 创建新用户
// @Description 创建一个新的用户
// @Tags 用户管理
// @Accept json
// @Produce json
// @Success 200 {string} string "成功"
// @Router /users [post]
func CreateUser(c *FastGo.Context) {
	c.SendString(200, "POST /users")
}

// GetUserByID 根据ID获取用户
// @Summary 根据ID获取用户
// @Description 根据用户ID获取用户详细信息
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param id path string true "用户ID"
// @Success 200 {string} string "成功"
// @Router /users/{id} [get]
func GetUserByID(c *FastGo.Context) {
	id := c.GetPathParam("id")
	c.SendString(200, fmt.Sprintf("GET /users/%s", id))
}

// UpdateUser 更新用户信息
// @Summary 更新用户信息
// @Description 根据用户ID更新用户信息
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param id path string true "用户ID"
// @Success 200 {string} string "成功"
// @Router /users/{id} [put]
func UpdateUser(c *FastGo.Context) {
	id := c.GetPathParam("id")
	c.SendString(200, fmt.Sprintf("PUT /users/%s", id))
}

// DeleteUser 删除用户
// @Summary 删除用户
// @Description 根据用户ID删除用户
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param id path string true "用户ID"
// @Success 200 {string} string "成功"
// @Router /users/{id} [delete]
func DeleteUser(c *FastGo.Context) {
	id := c.GetPathParam("id")
	c.SendString(200, fmt.Sprintf("DELETE /users/%s", id))
}

// AdminDashboard 管理员仪表板
// @Summary 管理员仪表板
// @Description 获取管理员仪表板信息
// @Tags 管理员
// @Accept json
// @Produce json
// @Success 200 {string} string "成功"
// @Router /admin/dashboard [get]
func AdminDashboard(c *FastGo.Context) {
	c.SendString(200, "Admin Dashboard")
}

// AdminUsers 管理用户
// @Summary 管理用户
// @Description 管理系统中的用户
// @Tags 管理员
// @Accept json
// @Produce json
// @Success 200 {string} string "成功"
// @Router /admin/users [get]
func AdminUsers(c *FastGo.Context) {
	c.SendString(200, "Admin Users Management")
}

// APIPing API健康检查
// @Summary API健康检查
// @Description 检查API服务是否正常运行
// @Tags API v1
// @Accept json
// @Produce json
// @Success 200 {string} string "成功"
// @Router /api/v1/ping [get]
func APIPing(c *FastGo.Context) {
	c.SendString(200, "API v1 Ping")
}

// APIStatus 获取服务器状态
// @Summary 获取服务器状态
// @Description 获取服务器当前状态信息
// @Tags API v1
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string "成功"
// @Router /api/v1/status [get]
func APIStatus(c *FastGo.Context) {
	c.SendJson(200, FastGo.FJ{
		"status": "ok",
		"server": "FastGo",
	})
}

// GetUsersV2 获取用户列表
// @Summary 获取用户列表
// @Description 获取API v2版本的用户列表
// @Tags API v2 - 用户
// @Accept json
// @Produce json
// @Success 200 {string} string "成功"
// @Router /api/v2/users/list [get]
func GetUsersV2(c *FastGo.Context) {
	c.SendString(200, "API v2 - Users List")
}

// CreateUserV2 创建用户
// @Summary 创建用户
// @Description 在API v2版本中创建新用户
// @Tags API v2 - 用户
// @Accept json
// @Produce json
// @Success 200 {string} string "成功"
// @Router /api/v2/users/create [post]
func CreateUserV2(c *FastGo.Context) {
	c.SendString(200, "API v2 - Create User")
}

// GetUserPermissions 获取用户权限
// @Summary 获取用户权限
// @Description 获取用户的管理权限信息
// @Tags API v2 - 用户管理
// @Accept json
// @Produce json
// @Success 200 {string} string "成功"
// @Router /api/v2/users/admin/permissions [get]
func GetUserPermissions(c *FastGo.Context) {
	c.SendString(200, "API v2 - Users Admin Permissions")
}

// SetUserRoles 设置用户角色
// @Summary 设置用户角色
// @Description 为用户设置角色
// @Tags API v2 - 用户管理
// @Accept json
// @Produce json
// @Success 200 {string} string "成功"
// @Router /api/v2/users/admin/roles [post]
func SetUserRoles(c *FastGo.Context) {
	c.SendString(200, "API v2 - Set User Roles")
}

// GetProductsV2 获取产品列表
// @Summary 获取产品列表
// @Description 获取API v2版本的产品列表
// @Tags API v2 - 产品
// @Accept json
// @Produce json
// @Success 200 {string} string "成功"
// @Router /api/v2/products/list [get]
func GetProductsV2(c *FastGo.Context) {
	c.SendString(200, "API v2 - Products List")
}

// CreateProductV2 创建产品
// @Summary 创建产品
// @Description 在API v2版本中创建新产品
// @Tags API v2 - 产品
// @Accept json
// @Produce json
// @Success 200 {string} string "成功"
// @Router /api/v2/products/create [post]
func CreateProductV2(c *FastGo.Context) {
	c.SendString(200, "API v2 - Create Product")
}

// GetDashboardSettings 获取仪表板设置
// @Summary 获取仪表板设置
// @Description 获取用户仪表板的设置信息
// @Tags 用户面板
// @Accept json
// @Produce json
// @Success 200 {string} string "成功"
// @Router /user/dashboard/settings [get]
func GetDashboardSettings(c *FastGo.Context) {
	c.SendString(200, "User Dashboard Settings")
}

// GetProfileSettings 获取个人资料设置
// @Summary 获取个人资料设置
// @Description 获取用户的个人资料设置
// @Tags 用户面板
// @Accept json
// @Produce json
// @Success 200 {string} string "成功"
// @Router /user/dashboard/settings/profile [get]
func GetProfileSettings(c *FastGo.Context) {
	c.SendString(200, "User Profile Settings")
}

// UpdateProfile 更新个人资料
// @Summary 更新个人资料
// @Description 更新用户的个人资料
// @Tags 用户面板
// @Accept json
// @Produce json
// @Success 200 {string} string "成功"
// @Router /user/dashboard/settings/profile [put]
func UpdateProfile(c *FastGo.Context) {
	c.SendString(200, "Update User Profile")
}

// @title FastGo API
// @version 1.0
// @description 这是一个使用FastGo框架构建的API示例
// @host localhost:8888
// @BasePath /
func main() {
	// 创建带Swagger支持的应用
	app := FastGo.NewFastGo()
	// 加载 Swagger 文档

	// 创建独立的路由器
	userRouter := FastGo.NewRouter()
	userGroup := userRouter.Group("/users")
	{
		userGroup.GET("", GetUsers)
		userGroup.POST("", CreateUser)
		userGroup.GET("/:id", GetUserByID)
		userGroup.PUT("/:id", UpdateUser)
		userGroup.DELETE("/:id", DeleteUser)
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

		adminGroup.GET("/dashboard", AdminDashboard)
		adminGroup.GET("/users", AdminUsers)
	}

	// 添加其他路由
	apiGroup := app.Group("/api/v1")
	{
		apiGroup.GET("/ping", APIPing)
		apiGroup.GET("/status", APIStatus)
	}

	// 演示嵌套分组功能
	apiV2Group := app.Group("/api")
	{
		v2Group := apiV2Group.Group("/v2") // 嵌套分组
		{
			usersV2Group := v2Group.Group("/users") // 更深层嵌套
			{
				usersV2Group.GET("/list", GetUsersV2)
				usersV2Group.POST("/create", CreateUserV2)

				// 更深层的嵌套
				adminV2Group := usersV2Group.Group("/admin") // 三层嵌套
				{
					adminV2Group.GET("/permissions", GetUserPermissions)
					adminV2Group.POST("/roles", SetUserRoles)
				}
			}

			// 同级的其他分组
			productsV2Group := v2Group.Group("/products")
			{
				productsV2Group.GET("/list", GetProductsV2)
				productsV2Group.POST("/create", CreateProductV2)
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
				settingsGroup.GET("", GetDashboardSettings)

				profileGroup := settingsGroup.Group("/profile") // 三层嵌套
				{
					profileGroup.GET("", GetProfileSettings)
					profileGroup.PUT("", UpdateProfile)
				}
			}
		}
	}
	app.Use(Cors())
	app.Use(Swagger())
	app.Run("192.168.0.102:8888")
}
func Swagger() FastGo.Middleware {
	swagger := FastGoMid.NewSwaggerMid()
	swagger.LoadSwaggerDoc("examples/basic/docs/swagger.json")
	return swagger
}
func Cors() FastGo.Middleware {
	cors := FastGoMid.NewCors()
	cors.SetAllowMethods("GET, POST, PUT, DELETE, OPTIONS").
		SetAllowOrigins("*").
		SetAllowHeaders("Content-Type, Authorization")
	return cors
}
