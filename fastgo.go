package FastGo

import (
	"LogX"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var defaultLogger = LogX.NewDefaultSyncLogger("FastGo")

type App struct {
	core        *core
	router      *Router
	middlewares []Engine
}

func NewFastGo() *App {
	router := NewRouter()
	middlewares := make([]Engine, 0)
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
	addr, port := parseAddress(addr, true)
	if addr == "" || port == 0 {
		defaultLogger.Error("Invalid address: %s", addr)
		return
	}
	h.core.addHandler(midToHandler(h.middlewares)...)
	h.core.addHandler(h.router.Handle)
	h.core.SetCert(certFile, keyFile)
	if addr == "0.0.0.0" {
		se := getAllIPs()
		defaultLogger.Info("Server started at all address (TLS)")
		for _, addr := range se {
			defaultLogger.Info("Running https://%s:%d", addr, port)
		}
	} else if addr == "localhost" || addr == "127.0.0.1" {
		defaultLogger.Info("Server started at %s", addr)
		defaultLogger.Info("Running https://localhost:%d", port)
	} else {
		defaultLogger.Info("Server started at %s (TLS)", addr)
		defaultLogger.Info("Running https://localhost:%d", port)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := h.core.listenHTTPS(addr+":"+strconv.Itoa(port), certFile, keyFile); err != nil && err != http.ErrServerClosed {
			defaultLogger.Error("Server failed to start (TLS): %v", err)
			h.core.Close()
			return
		}
	}()
	h.gracefulShutdown()
	wg.Wait()
}

func (h *App) Run(addr string) {

	addr, port := parseAddress(addr, false)
	if addr == "" || port == 0 {
		defaultLogger.Error("Invalid address: %s", addr)
		return
	}
	h.core.addHandler(midToHandler(h.middlewares)...)
	h.core.addHandler(h.router.Handle)

	if addr == "0.0.0.0" {
		se := getAllIPs()
		defaultLogger.Info("Server started at all address")
		for _, addr := range se {
			defaultLogger.Info("Running http://%s:%d", addr, port)
		}
	} else if addr == "localhost" || addr == "127.0.0.1" {
		defaultLogger.Info("Server started at %s", addr)
		defaultLogger.Info("Running http://localhost:%d", port)
	} else {
		defaultLogger.Info("Server started at %s", addr)
		defaultLogger.Info("Running http://%s:%d", addr, port)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := h.core.listenHTTP(addr + ":" + strconv.Itoa(port)); err != nil && err != http.ErrServerClosed {
			defaultLogger.Fatal("Server failed to start: %v", err)
			h.core.Close()
			return
		}
	}()
	h.gracefulShutdown()
	wg.Wait()
}

// Use 添加中间件到应用
func (h *App) Use(middlewares ...Engine) {
	h.middlewares = append(h.middlewares, middlewares...)
}

// gracefulShutdown 优雅关闭服务器
func (h *App) gracefulShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sign := <-sigCh
	defaultLogger.Info("Receive %s Server shutting down...", sign)
	h.core.Close()
	defaultLogger.Info("Server shutdown complete")
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

func midToHandler(middlewares []Engine) []HandlerFunc {
	handlers := make([]HandlerFunc, 0)
	for _, middleware := range middlewares {
		handlers = append(handlers, middleware.Handle)
	}
	return handlers
}

func parseAddress(addr string, https bool) (host string, port int) {
	// 1. 处理空地址，设置默认值
	if addr == "" {
		if https {
			return "0.0.0.0", 443
		}
		return "0.0.0.0", 80
	}

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		defaultLogger.Error("Invalid address: %v", err)
		return "", 0
	}

	// 3. 校验并转换端口
	port, err = strconv.Atoi(portStr)
	if err != nil {
		defaultLogger.Error("Invalid port: %v", err)
		return "", 0
	}
	if port < 0 || port > 65535 {
		defaultLogger.Error("port out of range (0-65535): %d", port)
		return "", 0
	}

	// 4. 处理 Host 为空的情况（例如 ":8080"）
	if host == "" {
		host = "0.0.0.0"
	}

	return host, port
}
func getAllIPs() []string {
	// 初始化结果切片，第一个元素固定为127.0.0.1
	ipList := []string{"localhost"}
	// 用于去重：key为IP地址，避免同一IP多次添加
	ipSet := make(map[string]struct{})
	ipSet["localhost"] = struct{}{}

	interfaces, err := net.Interfaces()
	if err != nil {
		return ipList
	}

	for _, iface := range interfaces {

		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		if isVirtualInterface(iface.Name) {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			default:
				continue
			}
			if ip == nil || ip.To4() == nil {
				continue
			}

			ipStr := ip.String()
			if _, exists := ipSet[ipStr]; !exists {
				ipSet[ipStr] = struct{}{}
				ipList = append(ipList, ipStr)
			}
		}
	}

	return ipList
}

// isVirtualInterface 判断是否为虚拟网卡（保留原逻辑，兼容Windows/Linux/Mac）
func isVirtualInterface(name string) bool {
	lowerName := strings.ToLower(name)
	// 常见虚拟网卡关键字，覆盖Docker/VMware/桥接/隧道等场景
	virtualKeywords := []string{
		"virtual", "vmware", "vbox", "docker", "bridge",
		"tunnel", "hyper-v", "veth", "utun", "tap",
		"virbr", "docker0", "kube-", "cni-", "wsl",
	}

	for _, keyword := range virtualKeywords {
		if strings.Contains(lowerName, keyword) {
			return true
		}
	}
	return false
}
