# FastGo - 轻量级Go Web框架

FastGo是一个轻量级的Go语言Web框架，专注于提供高性能和简洁的API。它提供了路由、中间件、嵌套分组等特性，让开发者能够快速构建Web应用程序。

## 特性

- **高性能路由**: 基于Trie树的路由算法，支持参数路由和通配符路由
- **路由分组**: 支持嵌套路由分组，便于组织复杂的路由结构
- **中间件系统**: 支持全局中间件和路由组中间件
- **优雅关闭**: 支持服务器优雅关闭，确保正在处理的请求能够完成
- **上下文管理**: 提供丰富的请求和响应处理方法
- **异步日志**: 内置异步日志系统，支持多种日志级别和彩色输出
- **上下文池**: 使用上下文池提高性能，减少GC压力

## 安装

```bash
go mod init your-project-name
go get github.com/miyingqi/FastGo
```

## 快速开始

```go
package main

import (
	"fmt"
	"github.com/miyingqi/FastGo"
)

func main() {
	// 创建FastGo实例，监听8080端口
	app := FastGo.NewFastGo(":8080")

	// 注册简单的路由
	app.Router().GET("/", func(c *FastGo.Context) {
		c.SendString(200, "Hello, FastGo!")
	})

	// 启动服务器
	if err := app.Run(); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
```

## 路由分组

FastGo支持强大的路由分组功能，包括嵌套分组：

```go
// 创建路由组
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

// 嵌套分组示例
v2Group := app.Group("/api")
{
    usersGroup := v2Group.Group("/v2").Group("/users")
    {
        usersGroup.GET("/list", func(c *FastGo.Context) {
            c.SendString(200, "API v2 - Users List")
        })
        
        usersGroup.POST("/create", func(c *FastGo.Context) {
            c.SendString(200, "API v2 - Create User")
        })
    }
}
```

## 中间件

FastGo支持灵活的中间件系统：

```go
// 全局中间件
app.Use(func(c *FastGo.Context) {
    fmt.Println("Global middleware executed")
    c.Next()
})

// 路由组中间件
adminGroup := app.Group("/admin")
adminGroup.Use(func(c *FastGo.Context) {
    fmt.Println("Admin middleware executed")
    c.SetHeader("X-Middleware", "admin")
    c.Next()
})

adminGroup.GET("/dashboard", func(c *FastGo.Context) {
    c.SendString(200, "Admin Dashboard")
})
```

## 参数路由

支持参数路由，形如`:id`或`:name`：

```go
userGroup := app.Group("/users")
{
    userGroup.GET("/:id", func(c *FastGo.Context) {
        id := c.Params.ByName("id")
        c.SendString(200, fmt.Sprintf("GET /users/%s", id))
    })
    
    userGroup.PUT("/:id", func(c *FastGo.Context) {
        id := c.Params.ByName("id")
        c.SendString(200, fmt.Sprintf("PUT /users/%s", id))
    })
}
```

## 独立路由器

可以创建独立的路由器并将其合并到主应用：

```go
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
}

// 将独立路由器合并到主应用
app.AddRouter(userRouter)
```

## 上下文功能

FastGo的Context提供了丰富的请求和响应处理方法：

- `c.Query(key)` - 获取查询参数
- `c.PostForm(key)` - 获取表单参数
- `c.SendString(code, body)` - 发送字符串响应
- `c.SendJson(code, jsonData)` - 发送JSON响应
- `c.SendHtml(code, html)` - 发送HTML响应
- `c.BindJSON(obj)` - 解析JSON请求体
- `c.ClientIP()` - 获取客户端IP
- `c.UserAgent()` - 获取User-Agent
- `c.Params.ByName(key)` - 获取路由参数

## 日志系统

FastGo内置了异步日志系统：

```go
// 使用内置日志
app.logger.InfoWithModule("APP", "Starting FastGo: "+app.server.Addr)
```

## 示例应用

请参考 [examples/basic/main.go](examples/basic/main.go) 查看完整示例，其中包含了各种特性的使用方法：

- 基本路由定义
- 路由分组
- 嵌套分组
- 中间件使用
- 参数路由
- 独立路由器合并

## 架构设计

FastGo采用以下核心组件：

- **App**: 应用程序入口，包含HTTP服务器和路由器
- **Router**: 路由系统，支持GET、POST、PUT、DELETE等方法
- **RouteGroup**: 路由分组，支持嵌套和中间件
- **Context**: 请求上下文，处理请求和响应
- **Middleware**: 中间件接口，用于请求处理链

## 性能优化

- 使用sync.Pool复用Context对象
- 异步日志系统减少I/O阻塞
- Trie树路由算法提供O(n)查找时间复杂度
- 高效的中间件链执行机制

## 许可证

MIT License