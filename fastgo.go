package FastGo

import (
	"context"
	"errors"
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
	contextPool *sync.Pool
	certFile    string
	keyFile     string
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
		contextPool: nil,
		certFile:    "",
		keyFile:     "",
	}

}

// Run 启动服务器并支持优雅关机
func (h *App) Run() error {
	// 应用中间件链
	h.server.Handler = h
	h.AddMiddleware(h.router)
	contextpool := &sync.Pool{
		New: func() interface{} {
			// 返回零值的 Context 结构体，而不是调用 NewContext
			return NewContext()
		},
	}

	h.contextPool = contextpool
	go func() {
		var err error
		// 判断是否配置了证书文件
		if h.certFile != "" && h.keyFile != "" {
			h.logger.Info("Starting FastGo HTTPS on " + h.server.Addr)
			// 启动 HTTPS 服务
			err = h.server.ListenAndServeTLS(h.certFile, h.keyFile)
		} else {
			h.logger.Info("Starting FastGo HTTP on " + h.server.Addr)
			// 启动 HTTP 服务
			err = h.server.ListenAndServe()
		}

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
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

	// 创建一个具有超时的上下文，用于Shutdown操作
	// 这里设置5秒超时，你可以根据实际需要调整
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel() // 确保释放上下文资源

	// 调用Shutdown
	if err := h.server.Shutdown(ctx); err != nil {
		h.logger.Info("FastGo is shutdown!")
		return err
	}

	h.logger.Info("FastGo exited")
	return nil
}

// AddMiddleware 添加中间件到应用（与 Use 方法功能相同，提供另一种方式）
func (h *App) AddMiddleware(middlewares ...Middleware) {
	h.middlewares = append(h.middlewares, middlewares...)
}

func (h *App) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	ctx := h.contextPool.Get().(*Context)
	ctx.Reset(writer, request)
	ctx.SetHandles(h.middlewares)
	ctx.Next()
	h.contextPool.Put(ctx)
}

// SetTLS 设置证书路径，开启 HTTPS
func (h *App) SetTLS(certFile, keyFile string) {
	h.certFile = certFile
	h.keyFile = keyFile
}

// POST 添加路由
func (h *App) GET(path string, handlers ...HandlerFunc) {
	h.router.addRoute(path, "GET", handlers...)
}

// POST 添加路由
func (h *App) POST(path string, handlers ...HandlerFunc) {
	h.router.addRoute(path, "POST", handlers...)
}

// PUT 添加路由
func (h *App) PUT(path string, handlers ...HandlerFunc) {
	h.router.addRoute(path, "PUT", handlers...)
}

// DELETE 添加路由
func (h *App) DELETE(path string, handlers ...HandlerFunc) {
	h.router.addRoute(path, "DELETE", handlers...)
}
