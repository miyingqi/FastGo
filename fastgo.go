package FastGo

import (
	"net/http"
	"sync"
	"time"
)

type App struct {
	server      *app
	router      *Router
	middlewares []HandlerStruct
	logger      *SyncLogger
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

func NewFastGo() *App {
	router := NewRouter()
	middlewares := make([]HandlerStruct, 0)
	middlewares = append(middlewares, NewMiddlewareLog())
	app := &App{
		server:      newServer(),
		router:      router,
		middlewares: middlewares,
		logger:      NewSyncLogger(INFO),
	}
	return app
}
func (h *App) Run(addr string) error {

	h.server.handlersChain = append(h.server.handlersChain, midToHandler(h.middlewares)...)
	h.server.handlersChain = append(h.server.handlersChain, h.router.HandleHTTP)
	return h.server.ListenAndServe(addr)
}

// Use 添加中间件到应用
func (h *App) Use(middlewares ...HandlerStruct) {
	h.middlewares = append(h.middlewares, middlewares...)
}

// Run 启动服务器并支持优雅关机

func (h *App) gracefulShutdown() {

}

type app struct {
	server        *http.Server
	handlersChain []HandlerFunc
	contextPool   sync.Pool // 上下文池
}

func newServer() *app {
	return &app{
		server: &http.Server{
			Handler:        nil,
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    30 * time.Second,
			MaxHeaderBytes: 1 << 20, // 1MB
		},
		handlersChain: nil,
		contextPool: sync.Pool{
			New: func() interface{} {
				return NewContext(nil, nil)
			},
		},
	}

}

func (s *app) ListenAndServe(addr string) error {
	s.server.Addr = addr
	s.server.Handler = s
	return s.server.ListenAndServe()
}
func (s *app) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	ctx, ok := s.contextPool.Get().(*Context)
	if !ok {
		ctx = NewContext(writer, request)
	} else {
		ctx.Reset(writer, request)
	}

	ctx.SetHandles(s.handlersChain)

	ctx.Next()

	s.contextPool.Put(ctx)
}

func midToHandler(middlewares []HandlerStruct) []HandlerFunc {
	handlers := make([]HandlerFunc, 0)
	for _, middleware := range middlewares {
		handlers = append(handlers, middleware.HandleHTTP)
	}
	return handlers
}
