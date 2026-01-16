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
	middlewares []Middleware
	index       int

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
		Request:     request,
		Writer:      writer,
		method:      request.Method,
		path:        request.URL.Path,
		params:      make(map[string]string),
		query:       request.URL.Query(),
		headers:     make(map[string]string),
		middlewares: nil,
		index:       -1,
		errors:      make([]error, 0),
		store:       make(map[interface{}]interface{}),
		startTime:   time.Now(),
		requestID:   request.Header.Get("X-Request-Id"),
		aborted:     false,
		written:     false,
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

// SetHandles 设置中间件链
func (c *Context) SetHandles(mids []Middleware) {
	c.middlewares = mids
}

// SetParam 设置单个路径参数（如 id -> 123）
func (c *Context) SetParam(key, value string) {
	c.params[key] = value
}

// GetParam 获取单个路径参数，不存在则返回空字符串
func (c *Context) GetParam(key string) string {
	return c.params[key]
}

// GetParamOrDefault 获取路径参数，不存在则返回默认值
func (c *Context) GetParamOrDefault(key, defaultValue string) string {
	if val, ok := c.params[key]; ok {
		return val
	}
	return defaultValue
}

// GetParams 获取所有路径参数的副本（避免外部修改内部map）
func (c *Context) GetParams() map[string]string {
	paramsCopy := make(map[string]string, len(c.params))
	for k, v := range c.params {
		paramsCopy[k] = v
	}
	return paramsCopy
}

// ClearParams 清空所有路径参数（池化重置时调用）
func (c *Context) ClearParams() {
	for k := range c.params {
		delete(c.params, k)
	}
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
	s := len(c.middlewares)
	for ; c.index < s; c.index++ {
		// 如果已经中止，停止执行
		if c.aborted {
			return
		}
		c.middlewares[c.index].HandleHTTP(c)
	}
}

// Abort 中止后续处理器的执行
func (c *Context) Abort() {
	c.aborted = true
}

func (c *Context) Reset(writer http.ResponseWriter, request *http.Request) {
	c.Request = request
	c.Writer = writer
	c.method = request.Method
	c.path = request.URL.Path
	c.ClearParams()
	c.query = request.URL.Query()
	c.headers = make(map[string]string)
	c.index = -1
	c.errors = make([]error, 0)
	c.store = make(map[interface{}]interface{})
	c.startTime = time.Now()
	c.requestID = request.Header.Get("X-Request-Id")
	c.aborted = false
	c.written = false

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
