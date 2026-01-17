package FastGo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type App struct {
	server      *http.Server
	router      *Router
	middlewares []Middleware
	logger      *AsyncLogger
	contextPool sync.Pool // 上下文池
}

// Router 返回路由器实例
func (h *App) Router() *Router {
	return h.router
}

func NewFastGo(addr string) *App {
	router := NewRouter()

	// 创建中间件链，确保日志中间件在前，路由器在最后
	middlewares := make([]Middleware, 0)
	middlewares = append(middlewares, NewLogMid()) // 日志中间件
	middlewares = append(middlewares, router)      // 路由器作为最后一个中间件

	app := &App{
		server: &http.Server{
			Addr:           addr,
			Handler:        nil,
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    30 * time.Second,
			MaxHeaderBytes: 1 << 20, // 1MB
		},
		router:      router,
		middlewares: middlewares,
		logger:      NewAsyncLoggerSP(INFO),
	}

	// 初始化上下文池
	app.contextPool.New = func() interface{} {
		return NewContext(nil, nil)
	}

	return app
}

// Use 添加中间件到应用
func (h *App) Use(middlewares ...Middleware) {
	h.middlewares = append(h.middlewares, middlewares...)
}

// AddMiddleware 添加中间件到应用（与 Use 方法功能相同，提供另一种方式）
// 现在作为别名，指向 Use 方法
func (h *App) AddMiddleware(middlewares ...Middleware) {
	h.Use(middlewares...)
}

// Run 启动服务器并支持优雅关机
func (h *App) Run() error {
	// 应用中间件链
	h.server.Handler = h
	// 在一个新的 goroutine 中启动服务器
	h.logger.Info("Starting FastGo: " + h.server.Addr)
	go func() {
		if err := h.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			h.logger.Error(fmt.Sprintf("Server error: %v", err))
		}
	}()

	// 配置优雅关机
	return h.gracefulShutdown()
}

func (h *App) gracefulShutdown() error {
	// 创建一个缓冲通道来接收系统信号
	quit := make(chan os.Signal, 1)
	// 注册我们关心的信号：SIGINT (Ctrl+C) 和 SIGTERM (kill 命令的默认信号)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	// 阻塞，直到收到信号
	_ = <-quit
	h.logger.Info("FastGo is shutting down...")

	// 创建一个具有超时的上下文，用于Shutdown操作
	// 这里设置30秒超时，以便有足够时间处理正在进行的请求
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel() // 确保释放上下文资源

	// 调用Shutdown
	h.logger.Info("Shutting down server...")
	if err := h.server.Shutdown(ctx); err != nil {
		h.logger.Error(fmt.Sprintf("Server shutdown error: %v", err))
		return err
	}

	h.logger.Info("Server stopped gracefully")
	fmt.Println("服务器已优雅退出")
	return nil
}

func (h *App) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	// 从池中获取上下文
	ctx, ok := h.contextPool.Get().(*Context)
	if !ok {
		// 如果类型断言失败，创建新的上下文
		ctx = NewContext(writer, request)
	} else {
		// 初始化上下文
		ctx.Reset(writer, request)
	}

	// 设置中间件链
	ctx.SetHandles(h.middlewares)

	// 执行处理
	ctx.Next()

	// 将上下文放回池中
	h.contextPool.Put(ctx)
}
