package FastGo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type App struct {
	server      *http.Server
	router      *Router
	middlewares []Middleware
	logger      *AsyncLogger
}

func NewFastGo(addr string) *App {
	middlewares := make([]Middleware, 0)
	middlewares = append(middlewares, NewLogMid())
	return &App{
		server: &http.Server{
			Addr:           addr,
			Handler:        nil,
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    30 * time.Second,
			MaxHeaderBytes: 1 << 20, // 1MB
		},
		router:      NewRouter(),
		middlewares: middlewares,
		logger:      NewAsyncLoggerSP(INFO),
	}

}

// Run 启动服务器并支持优雅关机
func (h *App) Run() error {
	// 应用中间件链
	h.server.Handler = h
	h.Use(h.router)
	// 在一个新的 goroutine 中启动服务器
	h.logger.Info("Starting FastGo:" + h.server.Addr)
	go func() {
		if err := h.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			h.logger.Error(err.Error())
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
	h.logger.Info("FastGo is out!")

	// 创建一个具有超时的上下文，用于Shutdown操作
	// 这里设置5秒超时，你可以根据实际需要调整
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel() // 确保释放上下文资源

	// 调用Shutdown
	if err := h.server.Shutdown(ctx); err != nil {
		h.logger.Info("FastGo is shutdown!")
		return err
	}

	fmt.Println("服务器已退出")
	return nil
}

// Use 添加中间件到应用
func (h *App) Use(middlewares ...Middleware) {
	h.middlewares = append(h.middlewares, middlewares...)
}

// AddMiddleware 添加中间件到应用（与 Use 方法功能相同，提供另一种方式）
func (h *App) AddMiddleware(middlewares ...Middleware) {
	h.Use(middlewares...)
}

func (h *App) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	ctx := NewContext(writer, request)
	ctx.SetHandles(h.middlewares)
	ctx.Next()

	// 应用中间件链

}

// GET 添加路由
func (h *App) GET(path string, handlers ...HandlerFunc) {
	h.router.addRoute(path, "GET", handlers)
}

// POST 添加路由
func (h *App) POST(path string, handlers ...HandlerFunc) {
	h.router.addRoute(path, "POST", handlers)
}

// PUT 添加路由
func (h *App) PUT(path string, handlers ...HandlerFunc) {
	h.router.addRoute(path, "PUT", handlers)
}

// DELETE 添加路由
func (h *App) DELETE(path string, handlers ...HandlerFunc) {
	h.router.addRoute(path, "DELETE", handlers)
}
