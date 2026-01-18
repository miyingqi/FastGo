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

	middlewares := make([]Middleware, 0)
	middlewares = append(middlewares, NewLogMid())
	middlewares = append(middlewares, router)

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
	h.server.Handler = h
	h.logger.Info("Starting FastGo: " + h.server.Addr)
	go func() {
		if err := h.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			h.logger.Error(fmt.Sprintf("Server error: %v", err))
		}
	}()

	return h.gracefulShutdown()
}

func (h *App) gracefulShutdown() error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	_ = <-quit
	h.logger.Info("FastGo is shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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
	ctx, ok := h.contextPool.Get().(*Context)
	if !ok {
		ctx = NewContext(writer, request)
	} else {
		ctx.Reset(writer, request)
	}

	ctx.SetHandles(h.middlewares)

	ctx.Next()

	h.contextPool.Put(ctx)
}
