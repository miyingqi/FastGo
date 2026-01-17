package mcontext

import (
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Mcontext Context 请求上下文
type Mcontext struct {
	// 原始 HTTP 对象
	Request *http.Request
	Writer  http.ResponseWriter

	// 请求信息
	Method    string
	Path      string
	Params    map[string]string
	Query     url.Values
	clientIP  string
	userAgent string

	// 响应信息
	StatusCode int
	// 注意：headers map 主要是为了方便框架内部逻辑，
	// 真正的响应头必须写入 c.Writer.Header()
	Headers map[string]string

	// 数据存储
	store      map[interface{}]interface{}
	storeMutex sync.RWMutex

	Index int

	// 错误处理
	errors []error

	// 执行控制
	Aborted bool

	// 性能追踪
	startTime time.Time
	requestID string
	// 标记响应头是否已写入
	Written bool
}

func NewContext() *Mcontext {
	return &Mcontext{
		Params:  make(map[string]string),
		Headers: make(map[string]string),
		store:   make(map[interface{}]interface{}),
		errors:  make([]error, 0),
	}
}

func (c *Mcontext) Reset(writer http.ResponseWriter, request *http.Request) {
	c.Request = request
	c.Writer = writer
	c.Method = request.Method
	c.Path = request.URL.Path
	c.clientIP = request.RemoteAddr
	c.ClearParams()
	c.Query = request.URL.Query()
	c.Index = -1
	c.errors = make([]error, 0)
	c.store = make(map[interface{}]interface{})
	c.startTime = time.Now()
	c.requestID = request.Header.Get("X-Request-Id")
	c.Aborted = false
	c.Written = false
}

// ClearParams 清空所有路径参数（池化重置时调用）
func (c *Mcontext) ClearParams() {
	for k := range c.Params {
		delete(c.Params, k)
	}
}
