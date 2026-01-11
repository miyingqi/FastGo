package FastGo

import (
	"net/http"
	"net/url"
	"sync"
	"time"
)

type HandlerFunc func(*Context)

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
	headers    map[string]string

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
	}
}
func (c *Context) SetHeader(key, value string) {
	c.headers[key] = value
}
func (c *Context) SetStatus(code int) {
	c.statusCode = code
}
func (c *Context) SendString(code int, body string) {
	c.SetStatus(code)
	c.SetHeader("Content-Type", "text/plain; charset=utf-8")
	_, err := c.Writer.Write([]byte(body))
	if err != nil {
		panic(err)
	}
}
func (c *Context) Next() {
	c.index++
	s := len(c.handlers)
	for ; c.index < s; c.index++ {
		c.handlers[c.index](c)
	}
}

func HTTPNotFound(c *Context) {
	c.SetStatus(http.StatusNotFound)
	c.SetHeader("Content-Type", "text/plain; charset=utf-8")
	_, err := c.Writer.Write([]byte("404 Not Found"))
	if err != nil {
		panic(err)
	}

}
