package FastGo

import (
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type FJ map[string]interface{}
type HandlerFunc func(*Context)
type Middleware interface {
	HandleHTTP(*Context)
}

// Param 是单个URL参数的表示
type Param struct {
	Key   string
	Value string
}

// Params 是URL参数列表
type Params []Param

// ByName 根据参数名获取参数值
func (ps Params) ByName(name string) string {
	for _, p := range ps {
		if p.Key == name {
			return p.Value
		}
	}
	return ""
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

	// 路由参数
	Params Params
}

func NewContext(writer http.ResponseWriter, request *http.Request) *Context {
	return &Context{
		Request:   request,
		Writer:    writer,
		method:    "",
		path:      "",
		params:    make(map[string]string),
		query:     make(url.Values),
		headers:   make(map[string]string),
		handlers:  make([]HandlerFunc, 0),
		index:     -1,
		errors:    make([]error, 0),
		store:     make(map[interface{}]interface{}),
		startTime: time.Time{},
		requestID: "",
		aborted:   false,
		written:   false,
		Params:    make(Params, 0),
	}
}

// Reset 重置上下文以供复用
func (c *Context) Reset(writer http.ResponseWriter, request *http.Request) {
	// 重置请求相关字段
	c.Request = request
	c.Writer = writer
	c.method = request.Method
	c.path = request.URL.Path
	c.requestID = request.Header.Get("X-Request-Id")
	c.startTime = time.Now()

	// 重置客户端IP和User-Agent
	c.clientIP = ""
	c.userAgent = ""

	// 重置查询参数
	c.query = request.URL.Query()

	// 重置响应相关字段
	c.statusCode = 0
	c.headers = make(map[string]string) // 创建新map以避免引用共享

	// 重置处理器链
	c.handlers = c.handlers[:0] // 清空切片但保留容量

	// 重置执行控制
	c.index = -1
	c.aborted = false
	c.written = false

	// 重置错误
	c.errors = c.errors[:0]

	// 重置路由参数
	c.Params = c.Params[:0]

	// 重置存储
	c.storeMutex.Lock()
	for k := range c.store {
		delete(c.store, k)
	}
	c.storeMutex.Unlock()
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

// SendHtml 发送 HTML 响应
func (c *Context) SendHtml(code int, html string) {
	// 1. 设置状态码
	c.SetStatus(code)
	// 2. 设置 Content-Type
	c.SetHeader("Content-Type", "text/html; charset=utf-8")
	// 3. 写入数据
	_, err := c.Write([]byte(html))
	if err != nil {
		log.Printf("Error writing HTML response: %v", err)
	}
}

// Query 获取查询参数
func (c *Context) Query(key string) string {
	return c.query.Get(key)
}

// QueryDefault 获取查询参数，如果不存在则返回默认值
func (c *Context) QueryDefault(key, defaultValue string) string {
	value := c.Query(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// PostForm 获取表单参数
func (c *Context) PostForm(key string) string {
	return c.Request.FormValue(key)
}

// PostFormDefault 获取表单参数，如果不存在则返回默认值
func (c *Context) PostFormDefault(key, defaultValue string) string {
	value := c.PostForm(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// FormFile 获取上传的文件
func (c *Context) FormFile(name string) (multipart.File, *multipart.FileHeader, error) {
	return c.Request.FormFile(name)
}

// MultipartForm 获取多部分表单
func (c *Context) MultipartForm() (*multipart.Form, error) {
	err := c.Request.ParseMultipartForm(32 << 20) // 32MB
	if err != nil {
		return nil, err
	}
	return c.Request.MultipartForm, nil
}

// BindJSON 解析JSON请求体
func (c *Context) BindJSON(obj interface{}) error {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, obj)
}

// ClientIP 获取客户端IP地址
func (c *Context) ClientIP() string {
	if c.clientIP != "" {
		return c.clientIP
	}

	// 尝试从 X-Forwarded-For 头部获取
	xForwardedFor := c.Request.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		ips := strings.Split(xForwardedFor, ",")
		for _, ip := range ips {
			ip = strings.TrimSpace(ip)
			if ip != "" {
				c.clientIP = ip
				return ip
			}
		}
	}

	// 尝试从 X-Real-IP 头部获取
	xRealIP := c.Request.Header.Get("X-Real-IP")
	if xRealIP != "" {
		c.clientIP = xRealIP
		return xRealIP
	}

	// 使用 RemoteAddr
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		// 如果无法解析，则使用RemoteAddr作为备选
		// 移除端口号，如果有的话
		if strings.Contains(c.Request.RemoteAddr, ":") {
			host = strings.Split(c.Request.RemoteAddr, ":")[0]
		} else {
			host = c.Request.RemoteAddr
		}
	}

	// 处理IPv6回环地址
	if host == "::1" {
		host = "127.0.0.1"
	}

	c.clientIP = host
	return host
}

// UserAgent 获取用户代理字符串
func (c *Context) UserAgent() string {
	if c.userAgent != "" {
		return c.userAgent
	}

	c.userAgent = c.Request.UserAgent()
	return c.userAgent
}

// Cookie 获取Cookie值
func (c *Context) Cookie(name string) (string, error) {
	cookie, err := c.Request.Cookie(name)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

// SetCookie 设置Cookie
func (c *Context) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		MaxAge:   maxAge,
		Path:     path,
		Domain:   domain,
		Secure:   secure,
		HttpOnly: httpOnly,
	}
	http.SetCookie(c.Writer, cookie)
}

// Redirect 重定向
func (c *Context) Redirect(code int, location string) {
	c.SetStatus(code)
	c.SetHeader("Location", location)
	c.Writer.WriteHeader(code)
	_, _ = c.Writer.Write([]byte("Redirecting to: " + location))
}

// Data 发送原始数据
func (c *Context) Data(code int, contentType string, data []byte) {
	c.SetStatus(code)
	c.SetHeader("Content-Type", contentType)
	_, err := c.Write(data)
	if err != nil {
		log.Printf("Error writing raw data: %v", err)
	}
}

// File 发送文件响应
func (c *Context) File(filepath string) {
	http.ServeFile(c.Writer, c.Request, filepath)
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

// Get 获取存储的数据
func (c *Context) Get(key interface{}) (value interface{}, exists bool) {
	c.storeMutex.RLock()
	defer c.storeMutex.RUnlock()
	value, exists = c.store[key]
	return
}

// Set 设置存储的数据
func (c *Context) Set(key, value interface{}) {
	c.storeMutex.Lock()
	defer c.storeMutex.Unlock()
	c.store[key] = value
}

// MustGet 获取存储的数据，如果不存在会panic
func (c *Context) MustGet(key interface{}) interface{} {
	if value, exists := c.Get(key); exists {
		return value
	}
	panic("Key \"" + key.(string) + "\" does not exist")
}

// GetString 获取字符串类型的存储数据
func (c *Context) GetString(key interface{}) (s string) {
	if val, ok := c.Get(key); ok && val != nil {
		s, _ = val.(string)
	}
	return
}

// GetInt 获取整数类型的存储数据
func (c *Context) GetInt(key interface{}) (i int) {
	if val, ok := c.Get(key); ok && val != nil {
		i, _ = val.(int)
	}
	return
}

// GetInt64 获取64位整数类型的存储数据
func (c *Context) GetInt64(key interface{}) (i64 int64) {
	if val, ok := c.Get(key); ok && val != nil {
		i64, _ = val.(int64)
	}
	return
}

// GetFloat64 获取浮点数类型的存储数据
func (c *Context) GetFloat64(key interface{}) (f64 float64) {
	if val, ok := c.Get(key); ok && val != nil {
		f64, _ = val.(float64)
	}
	return
}

// GetBool 获取布尔类型的存储数据
func (c *Context) GetBool(key interface{}) (b bool) {
	if val, ok := c.Get(key); ok && val != nil {
		b, _ = val.(bool)
	}
	return
}

// IsAborted 检查是否已被中止
func (c *Context) IsAborted() bool {
	return c.aborted
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
