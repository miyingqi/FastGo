package FastGo

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
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

// ByNameInt 根据参数名获取参数值并转为整数
func (ps Params) ByNameInt(name string) int {
	value, _ := strconv.Atoi(ps.ByName(name))
	return value
}

// ByNameInt64 根据参数名获取参数值并转为64位整数
func (ps Params) ByNameInt64(name string) int64 {
	value, _ := strconv.ParseInt(ps.ByName(name), 10, 64)
	return value
}

// ByNameUint 根据参数名获取参数值并转为无符号整数
func (ps Params) ByNameUint(name string) uint {
	value, _ := strconv.ParseUint(ps.ByName(name), 10, 64)
	return uint(value)
}

// ByNameUint64 根据参数名获取参数值并转为64位无符号整数
func (ps Params) ByNameUint64(name string) uint64 {
	value, _ := strconv.ParseUint(ps.ByName(name), 10, 64)
	return value
}

// ByNameFloat64 根据参数名获取参数值并转为浮点数
func (ps Params) ByNameFloat64(name string) float64 {
	value, _ := strconv.ParseFloat(ps.ByName(name), 64)
	return value
}

// ByNameBool 根据参数名获取参数值并转为布尔值
func (ps Params) ByNameBool(name string) bool {
	value, _ := strconv.ParseBool(ps.ByName(name))
	return value
}

// ByNameDefault 根据参数名获取参数值，如果不存在则返回默认值
func (ps Params) ByNameDefault(name, defaultValue string) string {
	value := ps.ByName(name)
	if value == "" {
		return defaultValue
	}
	return value
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
	c.Request = request
	c.Writer = writer
	c.method = request.Method
	c.path = request.URL.Path
	c.requestID = request.Header.Get("X-Request-Id")
	c.startTime = time.Now()

	c.clientIP = ""
	c.userAgent = ""

	c.query = request.URL.Query()

	c.statusCode = 0
	c.headers = make(map[string]string) // 创建新map以避免引用共享

	c.handlers = c.handlers[:0] // 清空切片但保留容量

	c.index = -1
	c.aborted = false
	c.written = false

	c.errors = c.errors[:0]

	c.Params = c.Params[:0]

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
	c.headers[key] = value
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
	if !c.written {
		c.Writer.WriteHeader(c.statusCode)
		c.written = true
	}
	return c.Writer.Write(bytes)
}

// SendString 发送纯文本响应
func (c *Context) SendString(code int, body string) {
	c.SetStatus(code)
	c.SetHeader("Content-Type", "text/plain; charset=utf-8")
	_, err := c.Write([]byte(body))
	if err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

// SendJson 发送 JSON 响应
func (c *Context) SendJson(code int, jsonData FJ) {
	c.SetStatus(code)
	c.SetHeader("Content-Type", "application/json; charset=utf-8")

	bytes, err := json.Marshal(jsonData)
	if err != nil {
		c.SendString(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	_, err = c.Write(bytes)
	if err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

// SendHtml 发送 HTML 响应
func (c *Context) SendHtml(code int, html string) {
	c.SetStatus(code)
	c.SetHeader("Content-Type", "text/html; charset=utf-8")
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

// QueryArray 获取查询参数数组
func (c *Context) QueryArray(key string) []string {
	return c.query[key]
}

// QueryBool 获取布尔类型的查询参数
func (c *Context) QueryBool(key string) bool {
	value, _ := strconv.ParseBool(c.Query(key))
	return value
}

// QueryInt 获取整数类型的查询参数
func (c *Context) QueryInt(key string) int {
	value, _ := strconv.Atoi(c.Query(key))
	return value
}

// QueryInt64 获取64位整数类型的查询参数
func (c *Context) QueryInt64(key string) int64 {
	value, _ := strconv.ParseInt(c.Query(key), 10, 64)
	return value
}

// QueryFloat64 获取浮点数类型的查询参数
func (c *Context) QueryFloat64(key string) float64 {
	value, _ := strconv.ParseFloat(c.Query(key), 64)
	return value
}

// QueryUint 获取无符号整数类型的查询参数
func (c *Context) QueryUint(key string) uint {
	value, _ := strconv.ParseUint(c.Query(key), 10, 64)
	return uint(value)
}

// QueryUint64 获取64位无符号整数类型的查询参数
func (c *Context) QueryUint64(key string) uint64 {
	value, _ := strconv.ParseUint(c.Query(key), 10, 64)
	return value
}

// QuerySlice 获取字符串切片类型的查询参数
func (c *Context) QuerySlice(key string) []string {
	return c.Request.URL.Query()[key]
}

// DefaultQueryWithSlice 当查询参数不存在时返回默认值切片
func (c *Context) DefaultQueryWithSlice(key string, defaultValue []string) []string {
	values := c.QuerySlice(key)
	if len(values) == 0 {
		return defaultValue
	}
	return values
}

// GetHeader 获取请求头
func (c *Context) GetHeader(key string) string {
	return c.Request.Header.Get(key)
}

// Host 获取主机名
func (c *Context) Host() string {
	return c.Request.Host
}

// Path 获取请求路径
func (c *Context) Path() string {
	return c.Request.URL.Path
}

// Protocol 获取请求协议
func (c *Context) Protocol() string {
	if c.Request.TLS != nil {
		return "https"
	}
	return "http"
}

// FullPath 获取完整路径（包含查询参数）
func (c *Context) FullPath() string {
	return c.Request.URL.RequestURI()
}

// RemoteIP 获取远程IP地址
func (c *Context) RemoteIP() string {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return host
}

// IsAjax 判断是否为AJAX请求
func (c *Context) IsAjax() bool {
	return c.GetHeader("X-Requested-With") == "XMLHttpRequest"
}

// IsJSON 判断请求内容类型是否为JSON
func (c *Context) IsJSON() bool {
	contentType := c.GetHeader("Content-Type")
	return strings.Contains(contentType, "application/json")
}

// IsMethod 判断请求方法
func (c *Context) IsMethod(method string) bool {
	return strings.ToUpper(c.Request.Method) == strings.ToUpper(method)
}

// Referer 获取请求来源
func (c *Context) Referer() string {
	return c.Request.Referer()
}

// Referrer 同Referer（别名）
func (c *Context) Referrer() string {
	return c.Referer()
}

// Accept 获取Accept头信息
func (c *Context) Accept() string {
	return c.GetHeader("Accept")
}

// AcceptEncoding 获取Accept-Encoding头信息
func (c *Context) AcceptEncoding() string {
	return c.GetHeader("Accept-Encoding")
}

// AcceptLanguage 获取Accept-Language头信息
func (c *Context) AcceptLanguage() string {
	return c.GetHeader("Accept-Language")
}

// ContentType 获取Content-Type头信息
func (c *Context) ContentType() string {
	return c.GetHeader("Content-Type")
}

// Authorization 获取Authorization头信息
func (c *Context) Authorization() string {
	return c.GetHeader("Authorization")
}

// IsGet 判断是否为GET请求
func (c *Context) IsGet() bool {
	return c.IsMethod("GET")
}

// IsPost 判断是否为POST请求
func (c *Context) IsPost() bool {
	return c.IsMethod("POST")
}

// IsPut 判断是否为PUT请求
func (c *Context) IsPut() bool {
	return c.IsMethod("PUT")
}

// IsDelete 判断是否为DELETE请求
func (c *Context) IsDelete() bool {
	return c.IsMethod("DELETE")
}

// IsPatch 判断是否为PATCH请求
func (c *Context) IsPatch() bool {
	return c.IsMethod("PATCH")
}

// IsHead 判断是否为HEAD请求
func (c *Context) IsHead() bool {
	return c.IsMethod("HEAD")
}

// IsOptions 判断是否为OPTIONS请求
func (c *Context) IsOptions() bool {
	return c.IsMethod("OPTIONS")
}

// Range 获取Range头信息
func (c *Context) Range() string {
	return c.GetHeader("Range")
}

// AcceptRanges 检查是否接受范围请求
func (c *Context) AcceptRanges() bool {
	return c.GetHeader("Accept-Ranges") != ""
}

// ContentLength 获取内容长度
func (c *Context) ContentLength() int64 {
	length, _ := strconv.ParseInt(c.GetHeader("Content-Length"), 10, 64)
	return length
}

// XForwardedFor 获取X-Forwarded-For头信息
func (c *Context) XForwardedFor() string {
	return c.GetHeader("X-Forwarded-For")
}

// XRealIP 获取X-Real-IP头信息
func (c *Context) XRealIP() string {
	return c.GetHeader("X-Real-IP")
}

// XRequestedWith 获取X-Requested-With头信息
func (c *Context) XRequestedWith() string {
	return c.GetHeader("X-Requested-With")
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

// BindJSONStrict 解析JSON请求体（严格模式，不允许未知字段）
func (c *Context) BindJSONStrict(obj interface{}) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(obj)
}

// ShouldBindJSON 尝试绑定JSON，失败时返回错误
func (c *Context) ShouldBindJSON(obj interface{}) error {
	contentType := c.GetHeader("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return fmt.Errorf("content type is not application/json")
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}

	if len(strings.TrimSpace(string(body))) == 0 {
		return fmt.Errorf("request body is empty")
	}

	return json.Unmarshal(body, obj)
}

// ShouldBindQuery 将查询参数绑定到结构体
func (c *Context) ShouldBindQuery(obj interface{}) error {
	values := c.Request.URL.Query()
	return bindValuesToObject(values, obj)
}

// ShouldBindForm 将表单参数绑定到结构体
func (c *Context) ShouldBindForm(obj interface{}) error {
	contentType := c.GetHeader("Content-Type")
	if strings.Contains(contentType, "multipart/") {
		err := c.Request.ParseMultipartForm(32 << 20) // 32MB
		if err != nil {
			return err
		}
	} else {
		err := c.Request.ParseForm()
		if err != nil {
			return err
		}
	}

	values := c.Request.Form
	return bindValuesToObject(values, obj)
}

// bindValuesToObject 将url.Values绑定到结构体
func bindValuesToObject(values url.Values, obj interface{}) error {
	objValue := reflect.ValueOf(obj)
	if objValue.Kind() != reflect.Ptr || objValue.IsNil() {
		return fmt.Errorf("obj must be a pointer to struct")
	}

	objElem := objValue.Elem()
	if objElem.Kind() != reflect.Struct {
		return fmt.Errorf("obj must be a pointer to struct")
	}

	objType := objElem.Type()
	for i := 0; i < objElem.NumField(); i++ {
		field := objElem.Field(i)
		fieldType := objType.Field(i)

		// 获取tag中的键名，如果没有tag则使用字段名
		key := fieldType.Tag.Get("json")
		if key == "" {
			key = fieldType.Name
		}
		if key == "" {
			continue
		}

		if field.CanSet() {
			value := values.Get(key)
			if value != "" {
				setField(field, value)
			}
		}
	}
	return nil
}

// setField 设置字段值
func setField(field reflect.Value, value string) {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			field.SetInt(intValue)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if uintValue, err := strconv.ParseUint(value, 10, 64); err == nil {
			field.SetUint(uintValue)
		}
	case reflect.Bool:
		if boolValue, err := strconv.ParseBool(value); err == nil {
			field.SetBool(boolValue)
		}
	case reflect.Float32, reflect.Float64:
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			field.SetFloat(floatValue)
		}
	}
}

// ClientIP 获取客户端IP地址
func (c *Context) ClientIP() string {
	if c.clientIP != "" {
		return c.clientIP
	}

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

	xRealIP := c.Request.Header.Get("X-Real-IP")
	if xRealIP != "" {
		c.clientIP = xRealIP
		return xRealIP
	}

	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		if strings.Contains(c.Request.RemoteAddr, ":") {
			host = strings.Split(c.Request.RemoteAddr, ":")[0]
		} else {
			host = c.Request.RemoteAddr
		}
	}

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
	c.SetStatus(http.StatusNotFound)
	c.SetHeader("Content-Type", "text/plain; charset=utf-8")
	_, err := c.Write([]byte("404 Not Found"))
	if err != nil {
		log.Printf("Error writing 404 response: %v", err)
	}
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

// Error 添加错误到错误列表
func (c *Context) Error(err error) {
	if err != nil {
		c.errors = append(c.errors, err)
	}
}

// Errors 获取所有错误
func (c *Context) Errors() []error {
	return c.errors
}

// HasErrors 检查是否有错误
func (c *Context) HasErrors() bool {
	return len(c.errors) > 0
}

// GetError 获取第一个错误
func (c *Context) GetError() error {
	if len(c.errors) > 0 {
		return c.errors[0]
	}
	return nil
}

// Fail 执行失败操作，设置状态码并终止请求
func (c *Context) Fail(code int, errorMsg string) {
	c.SetStatus(code)
	c.SetHeader("Content-Type", "application/json; charset=utf-8")
	c.SendJson(code, FJ{
		"error":   true,
		"message": errorMsg,
		"status":  code,
	})
	c.Abort()
}

// FailWithError 执行失败操作，使用错误对象
func (c *Context) FailWithError(code int, err error) {
	c.SetStatus(code)
	c.SetHeader("Content-Type", "application/json; charset=utf-8")
	c.SendJson(code, FJ{
		"error":   true,
		"message": err.Error(),
		"status":  code,
	})
	c.Abort()
}

// NotFound 返回404错误
func (c *Context) NotFound(message string) {
	if message == "" {
		message = "Not Found"
	}
	c.Fail(http.StatusNotFound, message)
}

// BadRequest 返回400错误
func (c *Context) BadRequest(message string) {
	if message == "" {
		message = "Bad Request"
	}
	c.Fail(http.StatusBadRequest, message)
}

// Unauthorized 返回401错误
func (c *Context) Unauthorized(message string) {
	if message == "" {
		message = "Unauthorized"
	}
	c.Fail(http.StatusUnauthorized, message)
}

// Forbidden 返回403错误
func (c *Context) Forbidden(message string) {
	if message == "" {
		message = "Forbidden"
	}
	c.Fail(http.StatusForbidden, message)
}

// InternalServerError 返回500错误
func (c *Context) InternalServerError(message string) {
	if message == "" {
		message = "Internal Server Error"
	}
	c.Fail(http.StatusInternalServerError, message)
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
