package FastGo

import (
	"LogX"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var defaultLogger = LogX.NewDefaultSyncLogger("FastGo")

type App struct {
	core        *core
	router      *Router
	middlewares []Middleware
}

func NewFastGo() *App {
	router := NewRouter()
	middlewares := make([]Middleware, 0)
	middlewares = append(middlewares, NewMiddlewareLog())
	app := &App{
		core:        newCore(),
		router:      router,
		middlewares: middlewares,
	}
	return app
}

// Router 返回路由器实例
func (h *App) Router() *Router {
	return h.router
}

// SetRoutes 允许通过函数设置路由
func (h *App) SetRoutes(setupFunc func(*Router)) {
	setupFunc(h.router)
}

// Group 创建路由组
func (h *App) Group(prefix string) *RouteGroup {
	return h.router.Group(prefix)
}

// AddRouter 添加一个完整的路由器
func (h *App) AddRouter(router *Router) {
	h.router.MergeRouter(router)
}

func (h *App) RunTLS(addr, certFile, keyFile string) {
	h.core.addHandler(midToHandler(h.middlewares)...)
	h.core.addHandler(h.router.HandleHTTP)
	h.core.SetCert(certFile, keyFile)
	defaultLogger.Info("Server started at %s (TLS)", addr)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		if err := h.core.listenHTTPS(addr, certFile, keyFile); err != nil && err != http.ErrServerClosed {
			defaultLogger.Error("Server failed to start (TLS): %v", err)
			return
		}
	}()
	h.gracefulShutdown()
	wg.Wait()
}

func (h *App) Run(addr string) {
	h.core.addHandler(midToHandler(h.middlewares)...)
	h.core.addHandler(h.router.HandleHTTP)
	defaultLogger.Info("Server started at %s", addr)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := h.core.listenHTTP(addr); err != nil && err != http.ErrServerClosed {
			defaultLogger.Error("Server failed to start: %v", err)
			return
		}
	}()
	h.gracefulShutdown()
	wg.Wait()
}

// Use 添加中间件到应用
func (h *App) Use(middlewares ...Middleware) {
	h.middlewares = append(h.middlewares, middlewares...)
}

// gracefulShutdown 优雅关闭服务器
func (h *App) gracefulShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	_ = <-sigCh
	defaultLogger.Info("Server shutting down...")
}

type core struct {
	server       *http.Server
	handlerChain []HandlerFunc
	contextPool  sync.Pool // 上下文池，复用ctx避免GC
	cert         string    // TLS证书路径
	key          string    // TLS密钥路径
}

func newCore() *core {
	return &core{
		server: &http.Server{
			Handler:        nil,
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    30 * time.Second,
			MaxHeaderBytes: 1 << 20, // 1MB
		},
		handlerChain: nil,
		contextPool: sync.Pool{
			New: func() interface{} {
				return NewContext(nil, nil)
			},
		},
	}
}

func (s *core) listenHTTP(addr string) error {
	s.server.Addr = addr
	s.server.Handler = s
	return s.server.ListenAndServe()
}

func (s *core) listenHTTPS(addr, certFile, keyFile string) error {
	s.server.Addr = addr
	s.server.Handler = s
	return s.server.ListenAndServeTLS(certFile, keyFile)
}

// ServeHTTP 单routine处理HTTP请求
func (s *core) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	// 1. 从对象池获取ctx，失败则新建（兜底）
	ctx, ok := s.contextPool.Get().(*Context)
	if !ok || ctx == nil {
		ctx = NewContext(writer, request)
	} else {
		ctx.Reset(writer, request)
	}

	// 2. 设置处理器链
	ctx.SetHandles(s.handlerChain)

	// 3. 直接在当前goroutine中执行处理器链（单routine模式）
	ctx.Next()

	// 4. 处理完成后归还context到池中
	s.contextPool.Put(ctx)
}

func (s *core) SetCert(cert, key string) {
	s.cert = cert
	s.key = key
}

func (s *core) addHandler(handler ...HandlerFunc) {
	s.handlerChain = append(s.handlerChain, handler...)
}

func (s *core) Close() {
	err := s.server.Close()
	if err != nil {
		return
	}
}

func midToHandler(middlewares []Middleware) []HandlerFunc {
	handlers := make([]HandlerFunc, 0)
	for _, middleware := range middlewares {
		handlers = append(handlers, middleware.HandleHTTP)
	}
	return handlers
}
