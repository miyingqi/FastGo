package FastGo

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type FJ map[string]interface{}
type HandlerFunc func(*Context)
type Middleware interface {
	HandleHTTP(*Context)
}

// Context 请求上下文
type Context struct {
	// 原始 HTTP 对象
	Request *http.Request
	Writer  http.ResponseWriter

	// 请求信息
	method    string
	path      string
	params    map[string]string
	query     url.Values
	clientIP  string
	userAgent string

	// 响应信息
	statusCode int
	// 注意：headers map 主要是为了方便框架内部逻辑，
	// 真正的响应头必须写入 c.Writer.Header()
	headers map[string]string

	// 数据存储
	store      map[interface{}]interface{}
	storeMutex sync.RWMutex

	// 处理器链
	handlers []HandlerFunc
	index    int

	// 错误处理
	errors []error

	// 执行控制
	aborted bool

	// 性能追踪
	startTime time.Time
	requestID string
	// 标记响应头是否已写入
	written bool
}

func NewContext(writer http.ResponseWriter, request *http.Request) *Context {
	return &Context{
		Request:   request,
		Writer:    writer,
		method:    request.Method,
		path:      request.URL.Path,
		params:    make(map[string]string),
		query:     request.URL.Query(),
		headers:   make(map[string]string),
		handlers:  make([]HandlerFunc, 0),
		index:     -1,
		errors:    make([]error, 0),
		store:     make(map[interface{}]interface{}),
		startTime: time.Now(),
		requestID: request.Header.Get("X-Request-Id"),
		aborted:   false,
		written:   false,
	}
}

// SetHeader 设置响应头
func (c *Context) SetHeader(key string, values ...string) {
	if len(values) == 0 {
		return
	}
	var value string
	for i, v := range values {
		if i > 0 {
			value += ", "
		}
		value += v
	}
	// 更新内部 map
	c.headers[key] = value
	// 直接写入底层的 ResponseWriter
	c.Writer.Header().Set(key, value)
}

// StatusString 获取状态码文本
func (c *Context) StatusString(code int) string {
	return http.StatusText(code)
}

// SetStatus 仅设置状态码变量，不立即写入
func (c *Context) SetStatus(code int) {
	c.statusCode = code
}

// Write 写入响应体（辅助方法，通常不直接暴露）
func (c *Context) Write(bytes []byte) (int, error) {
	// 如果还没写入状态码，先写入
	if !c.written {
		c.Writer.WriteHeader(c.statusCode)
		c.written = true
	}
	return c.Writer.Write(bytes)
}

// SendString 发送纯文本响应
func (c *Context) SendString(code int, body string) {
	// 1. 设置状态码
	c.SetStatus(code)
	// 2. 设置 Content-Type
	c.SetHeader("Content-Type", "text/plain; charset=utf-8")
	// 3. 写入数据
	_, err := c.Write([]byte(body))
	if err != nil {
		// 不要 panic，记录日志即可
		log.Printf("Error writing response: %v", err)
	}
}

// SendJson 发送 JSON 响应
func (c *Context) SendJson(code int, jsonData FJ) {
	// 1. 设置状态码
	c.SetStatus(code)
	// 2. 设置 Content-Type
	c.SetHeader("Content-Type", "application/json; charset=utf-8")

	// 3. 序列化 JSON
	bytes, err := json.Marshal(jsonData)
	if err != nil {
		// 序列化失败是服务器内部错误
		c.SendString(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	// 4. 写入数据
	_, err = c.Write(bytes)
	if err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

// Next 执行下一个中间件/处理器
func (c *Context) Next() {
	c.index++
	s := len(c.handlers)
	for ; c.index < s; c.index++ {
		// 如果已经中止，停止执行
		if c.aborted {
			return
		}
		c.handlers[c.index](c)
	}
}

// Abort 中止后续处理器的执行
func (c *Context) Abort() {
	c.aborted = true
}

// SetHandles 设置中间件链
func (c *Context) SetHandles(middleware []Middleware) {
	c.handlers = middlewaresToHandlerFuncs(middleware)
}

// HTTPNotFound 处理 404 错误
func HTTPNotFound(c *Context) {
	// 1. 设置状态码
	c.SetStatus(http.StatusNotFound)
	// 2. 设置 Content-Type
	c.SetHeader("Content-Type", "text/plain; charset=utf-8")
	// 3. 写入数据
	_, err := c.Write([]byte("404 Not Found"))
	if err != nil {
		log.Printf("Error writing 404 response: %v", err)
	}
	// 4. 中止后续执行
	c.Abort()
}

// 将 Middleware 接口转换为 HandlerFunc
func middlewareToHandlerFunc(mw Middleware) HandlerFunc {
	return func(c *Context) {
		mw.HandleHTTP(c)
	}
}

// 批量转换接口切片为 HandlerFunc 切片
func middlewaresToHandlerFuncs(middlewares []Middleware) []HandlerFunc {
	if len(middlewares) == 0 {
		return nil
	}

	handlers := make([]HandlerFunc, len(middlewares))
	for i, mw := range middlewares {
		handlers[i] = middlewareToHandlerFunc(mw)
	}
	return handlers
}
