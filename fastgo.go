package FastGo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type App struct {
	server *http.Server
	router *Router
}

func (h *App) initServer(addr string, handler http.Handler) {
	h.server = &http.Server{
		Addr:           addr,
		Handler:        handler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    30 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}
	if h.router == nil {
		h.router = NewRouter()
	}

}

// Run 启动服务器并支持优雅关机
func (h *App) Run(addr string) error {

	// 应用中间件链

	fmt.Printf("准备启动服务器在 %s\n", addr)

	h.initServer(addr, h)
	// 在一个新的 goroutine 中启动服务器
	go func() {
		if err := h.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("服务器启动失败: %v\n", err)
		}
	}()

	// 配置优雅关机
	return h.gracefulShutdown()
}
func (h *App) gracefulShutdown() error {
	// 创建一个缓冲通道来接收系统信号
	quit := make(chan os.Signal, 1)

	// 注册我们关心的信号：SIGINT (Ctrl+C) 和 SIGTERM (kill 命令的默认信号)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 阻塞，直到收到信号
	sig := <-quit
	fmt.Printf("\n接收到信号: %s，正在优关闭服务器...\n", sig)

	// 创建一个具有超时的上下文，用于Shutdown操作
	// 这里设置5秒超时，你可以根据实际需要调整
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel() // 确保释放上下文资源

	// 调用Shutdown开始优雅关机过程
	if err := h.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("服务器强制关闭: %v", err)
	}

	fmt.Println("服务器已退出")
	return nil
}

func (h *App) Use() {

}
func (h *App) AddMiddleware() {

}
func (h *App) UseRouter(router *Router) {
	h.router = router
}
func (h *App) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	ctx := NewContext(writer, request)
	h.router.getRoute(request.Method).FindChild(request.URL.Path)
	h.router.HandleHTTP(ctx)
}
