package FastGo

import (
	"context"
	"encoding/json"
	"encoding/xml"
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

func (h HandlerFunc) Handle(c *Context) {
	h(c)
}

type HandleChain []HandlerFunc
type Engine interface {
	Handle(*Context)
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
	request *http.Request
	writer  http.ResponseWriter

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
	// 真正的响应头必须写入 c.writer.Header()
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

func (c *Context) SetParam(key string, value string) {
	c.params[key] = value
}

func (c *Context) SetParams(params Params) {
	for _, pa := range params {
		c.params[pa.Key] = pa.Value
	}
}

func NewContext(writer http.ResponseWriter, request *http.Request) *Context {
	return &Context{
		request:   request,
		writer:    writer,
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
	c.request = request
	c.writer = writer
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
	c.writer.Header().Set(key, value)
}

// StatusString 获取状态码文本
func (c *Context) StatusString(code int) string {
	return http.StatusText(code)
}

// SetStatus 仅设置状态码变量，不立即写入
func (c *Context) SetStatus(code int) {
	c.statusCode = code
}

// StatusCode 获取状态码
func (c *Context) StatusCode() int {
	return c.statusCode
}

// Write 写入响应体（辅助方法，通常不直接暴露）
func (c *Context) Write(bytes []byte) (int, error) {
	if !c.written {
		c.writer.WriteHeader(c.statusCode)
		c.written = true
	}
	return c.writer.Write(bytes)
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

// SendXml 发送 XML 响应
func (c *Context) SendXml(code int, xmlData interface{}) {
	c.SetStatus(code)
	c.SetHeader("Content-Type", "application/xml; charset=utf-8")

	bytes, err := xml.Marshal(xmlData)
	if err != nil {
		c.SendString(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	// 添加 XML声明
	xmlResponse := []byte(xml.Header + string(bytes))
	_, err = c.Write(xmlResponse)
	if err != nil {
		log.Printf("Error writing XML response: %v", err)
	}
}

// JSONP 发送 JSONP 响应
func (c *Context) JSONP(code int, callback string, data interface{}) {
	c.SetStatus(code)
	c.SetHeader("Content-Type", "application/javascript; charset=utf-8")

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		c.SendString(http.StatusInternalServerError, "Internal Server Error")
		return
	}

	response := fmt.Sprintf("%s(%s)", callback, string(jsonBytes))
	_, err = c.Write([]byte(response))
	if err != nil {
		log.Printf("Error writing JSONP response: %v", err)
	}
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
	http.ServeFile(c.writer, c.request, filepath)
}

// SendSuccess 发送成功响应
func (c *Context) SendSuccess(data interface{}) {
	c.SendJson(http.StatusOK, FJ{
		"success": true,
		"data":    data,
		"code":    http.StatusOK,
	})
}

// SendError 发送错误响应
func (c *Context) SendError(code int, message string, errData ...interface{}) {
	response := FJ{
		"success": false,
		"message": message,
		"code":    code,
	}
	if len(errData) > 0 {
		response["data"] = errData[0]
	}
	c.SendJson(code, response)
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
	return c.request.URL.Query()[key]
}

// DefaultQueryWithSlice 当查询参数不存在时返回默认值切片
func (c *Context) DefaultQueryWithSlice(key string, defaultValue []string) []string {
	values := c.QuerySlice(key)
	if len(values) == 0 {
		return defaultValue
	}
	return values
}

// GetQueryInt64Default 获取64位整数类型的查询参数，如果不存在则返回默认值
func (c *Context) GetQueryInt64Default(key string, defaultValue int64) int64 {
	value := c.QueryInt64(key)
	if value == 0 && c.Query(key) == "" {
		return defaultValue
	}
	return value
}

// GetQueryFloat64Default 获取浮点数类型的查询参数，如果不存在则返回默认值
func (c *Context) GetQueryFloat64Default(key string, defaultValue float64) float64 {
	value := c.QueryFloat64(key)
	if value == 0 && c.Query(key) == "" {
		return defaultValue
	}
	return value
}

// GetQueryBoolDefault 获取布尔类型的查询参数，如果不存在则返回默认值
func (c *Context) GetQueryBoolDefault(key string, defaultValue bool) bool {
	queryValue := c.Query(key)
	if queryValue == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(queryValue)
	if err != nil {
		return defaultValue
	}
	return value
}

// Request 获取原始请求
func (c *Context) Request() *http.Request {
	return c.request
}
func (c *Context) Method() string {
	return c.request.Method
}

// GetHeader 获取请求头
func (c *Context) GetHeader(key string) string {
	return c.request.Header.Get(key)
}

// Host 获取主机名
func (c *Context) Host() string {
	return c.request.Host
}

// Protocol 获取请求协议
func (c *Context) Protocol() string {
	if c.request.TLS != nil {
		return "https"
	}
	return "http"
}

// Referer 获取请求来源
func (c *Context) Referer() string {
	return c.request.Referer()
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

// Range 获取Range头信息
func (c *Context) Range() string {
	return c.GetHeader("Range")
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

// GetRangeHeader 获取Range请求头
func (c *Context) GetRangeHeader() string {
	return c.GetHeader("Range")
}

// GetAcceptRanges 检查是否接受范围请求
func (c *Context) GetAcceptRanges() bool {
	return c.GetHeader("Accept-Ranges") != ""
}

// GetAcceptEncoding 获取Accept-Encoding头信息
func (c *Context) GetAcceptEncoding() string {
	return c.GetHeader("Accept-Encoding")
}

// GetIfMatch 获取If-Match头信息
func (c *Context) GetIfMatch() string {
	return c.GetHeader("If-Match")
}

// GetIfNoneMatch 获取If-None-Match头信息
func (c *Context) GetIfNoneMatch() string {
	return c.GetHeader("If-None-Match")
}

// GetIfModifiedSince 获取If-Modified-Since头信息
func (c *Context) GetIfModifiedSince() string {
	return c.GetHeader("If-Modified-Since")
}

// GetIfUnmodifiedSince 获取If-Unmodified-Since头信息
func (c *Context) GetIfUnmodifiedSince() string {
	return c.GetHeader("If-Unmodified-Since")
}

// GetIfRange 获取If-Range头信息
func (c *Context) GetIfRange() string {
	return c.GetHeader("If-Range")
}

// GetConnection 获取Connection头信息
func (c *Context) GetConnection() string {
	return c.GetHeader("Connection")
}

// GetCacheControl 获取Cache-Control头信息
func (c *Context) GetCacheControl() string {
	return c.GetHeader("Cache-Control")
}

// GetPragma 获取Pragma头信息
func (c *Context) GetPragma() string {
	return c.GetHeader("Pragma")
}

// GetUpgrade 获取Upgrade头信息
func (c *Context) GetUpgrade() string {
	return c.GetHeader("Upgrade")
}

// GetTransferEncoding 获取Transfer-Encoding头信息
func (c *Context) GetTransferEncoding() string {
	return c.GetHeader("Transfer-Encoding")
}

// IsMethod 判断请求方法
func (c *Context) IsMethod(method string) bool {
	return strings.ToUpper(c.request.Method) == strings.ToUpper(method)
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

// IsAjax 判断是否为AJAX请求
func (c *Context) IsAjax() bool {
	return c.GetHeader("X-Requested-With") == "XMLHttpRequest"
}

// IsJSON 判断请求内容类型是否为JSON
func (c *Context) IsJSON() bool {
	contentType := c.GetHeader("Content-Type")
	return strings.Contains(contentType, "application/json")
}

// IsWebSocket 检查是否为WebSocket请求
func (c *Context) IsWebSocket() bool {
	connection := strings.ToLower(c.GetConnection())
	upgrade := strings.ToLower(c.GetUpgrade())
	return strings.Contains(connection, "upgrade") && upgrade == "websocket"
}

// IsGzip 检查是否支持gzip压缩
func (c *Context) IsGzip() bool {
	acceptEncoding := c.GetAcceptEncoding()
	return strings.Contains(strings.ToLower(acceptEncoding), "gzip")
}

// IsDeflate 检查是否支持deflate压缩
func (c *Context) IsDeflate() bool {
	acceptEncoding := c.GetAcceptEncoding()
	return strings.Contains(strings.ToLower(acceptEncoding), "deflate")
}

// Path 获取请求路径
func (c *Context) Path() string {
	return c.request.URL.Path
}

// FullPath 获取完整路径（包含查询参数）
func (c *Context) FullPath() string {
	return c.request.URL.RequestURI()
}

// RemoteIP 获取远程IP地址
func (c *Context) RemoteIP() string {
	host, _, err := net.SplitHostPort(c.request.RemoteAddr)
	if err != nil {
		return c.request.RemoteAddr
	}
	return host
}

// ClientIP 获取客户端IP地址
func (c *Context) ClientIP() string {
	if c.clientIP != "" {
		return c.clientIP
	}

	xForwardedFor := c.request.Header.Get("X-Forwarded-For")
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

	xRealIP := c.request.Header.Get("X-Real-IP")
	if xRealIP != "" {
		c.clientIP = xRealIP
		return xRealIP
	}

	host, _, err := net.SplitHostPort(c.request.RemoteAddr)
	if err != nil {
		if strings.Contains(c.request.RemoteAddr, ":") {
			host = strings.Split(c.request.RemoteAddr, ":")[0]
		} else {
			host = c.request.RemoteAddr
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

	c.userAgent = c.request.UserAgent()
	return c.userAgent
}

// PostForm 获取表单参数
func (c *Context) PostForm(key string) string {
	return c.request.FormValue(key)
}

// PostFormDefault 获取表单参数，如果不存在则返回默认值
func (c *Context) PostFormDefault(key, defaultValue string) string {
	value := c.PostForm(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// GetPostFormInt64 获取64位整数类型的表单参数
func (c *Context) GetPostFormInt64(key string) int64 {
	value, _ := strconv.ParseInt(c.PostForm(key), 10, 64)
	return value
}

// GetPostFormFloat64 获取浮点数类型的表单参数
func (c *Context) GetPostFormFloat64(key string) float64 {
	value, _ := strconv.ParseFloat(c.PostForm(key), 64)
	return value
}

// GetPostFormBool 获取布尔类型的表单参数
func (c *Context) GetPostFormBool(key string) bool {
	value, _ := strconv.ParseBool(c.PostForm(key))
	return value
}

// GetPostFormInt64Default 获取64位整数类型的表单参数，如果不存在则返回默认值
func (c *Context) GetPostFormInt64Default(key string, defaultValue int64) int64 {
	value := c.GetPostFormInt64(key)
	if value == 0 && c.PostForm(key) == "" {
		return defaultValue
	}
	return value
}

// GetPostFormFloat64Default 获取浮点数类型的表单参数，如果不存在则返回默认值
func (c *Context) GetPostFormFloat64Default(key string, defaultValue float64) float64 {
	value := c.GetPostFormFloat64(key)
	if value == 0 && c.PostForm(key) == "" {
		return defaultValue
	}
	return value
}

// GetPostFormBoolDefault 获取布尔类型的表单参数，如果不存在则返回默认值
func (c *Context) GetPostFormBoolDefault(key string, defaultValue bool) bool {
	formValue := c.PostForm(key)
	if formValue == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(formValue)
	if err != nil {
		return defaultValue
	}
	return value
}

// FormFile 获取上传的文件
func (c *Context) FormFile(name string) (multipart.File, *multipart.FileHeader, error) {
	return c.request.FormFile(name)
}

// MultipartForm 获取多部分表单
func (c *Context) MultipartForm() (*multipart.Form, error) {
	err := c.request.ParseMultipartForm(32 << 20) // 32MB
	if err != nil {
		return nil, err
	}
	return c.request.MultipartForm, nil
}

// Body 获取请求体内容
func (c *Context) Body() ([]byte, error) {
	return io.ReadAll(c.request.Body)
}

// BindJSON 解析JSON请求体
func (c *Context) BindJSON(obj interface{}) error {
	body, err := io.ReadAll(c.request.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, obj)
}

// BindJSONStrict 解析JSON请求体（严格模式，不允许未知字段）
func (c *Context) BindJSONStrict(obj interface{}) error {
	decoder := json.NewDecoder(c.request.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(obj)
}

// ShouldBindJSON 尝试绑定JSON，失败时返回错误
func (c *Context) ShouldBindJSON(obj interface{}) error {
	contentType := c.GetHeader("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return fmt.Errorf("content type is not application/json")
	}

	body, err := io.ReadAll(c.request.Body)
	if err != nil {
		return err
	}

	if len(strings.TrimSpace(string(body))) == 0 {
		return fmt.Errorf("request body is empty")
	}

	return json.Unmarshal(body, obj)
}

// BindXML 绑定XML请求体
func (c *Context) BindXML(obj interface{}) error {
	body, err := io.ReadAll(c.request.Body)
	if err != nil {
		return err
	}
	return xml.Unmarshal(body, obj)
}

// ShouldBindXML 尝试绑定XML，失败时返回错误
func (c *Context) ShouldBindXML(obj interface{}) error {
	contentType := c.GetHeader("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "application/xml") &&
		!strings.Contains(strings.ToLower(contentType), "text/xml") {
		return fmt.Errorf("content type is not application/xml or text/xml")
	}

	body, err := io.ReadAll(c.request.Body)
	if err != nil {
		return err
	}

	if len(strings.TrimSpace(string(body))) == 0 {
		return fmt.Errorf("request body is empty")
	}

	return xml.Unmarshal(body, obj)
}

// ShouldBindQuery 将查询参数绑定到结构体
func (c *Context) ShouldBindQuery(obj interface{}) error {
	values := c.request.URL.Query()
	return bindValuesToObject(values, obj)
}

// ShouldBindForm 将表单参数绑定到结构体
func (c *Context) ShouldBindForm(obj interface{}) error {
	contentType := c.GetHeader("Content-Type")
	if strings.Contains(contentType, "multipart/") {
		err := c.request.ParseMultipartForm(32 << 20) // 32MB
		if err != nil {
			return err
		}
	} else {
		err := c.request.ParseForm()
		if err != nil {
			return err
		}
	}

	values := c.request.Form
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
	default:

	}
}

// Cookie 获取Cookie值
func (c *Context) Cookie(name string) (string, error) {
	cookie, err := c.request.Cookie(name)
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
	http.SetCookie(c.writer, cookie)
}

// HasCookie 检查是否存在指定Cookie
func (c *Context) HasCookie(name string) bool {
	_, err := c.Cookie(name)
	return err == nil
}

// GetCookieInt 获取Cookie值并转换为整数
func (c *Context) GetCookieInt(name string) int {
	cookieVal, err := c.Cookie(name)
	if err != nil {
		return 0
	}
	value, _ := strconv.Atoi(cookieVal)
	return value
}

// GetCookieInt64 获取Cookie值并转换为64位整数
func (c *Context) GetCookieInt64(name string) int64 {
	cookieVal, err := c.Cookie(name)
	if err != nil {
		return 0
	}
	value, _ := strconv.ParseInt(cookieVal, 10, 64)
	return value
}

// GetCookieFloat64 获取Cookie值并转换为浮点数
func (c *Context) GetCookieFloat64(name string) float64 {
	cookieVal, err := c.Cookie(name)
	if err != nil {
		return 0.0
	}
	value, _ := strconv.ParseFloat(cookieVal, 64)
	return value
}

// GetCookieBool 获取Cookie值并转换为布尔值
func (c *Context) GetCookieBool(name string) bool {
	cookieVal, err := c.Cookie(name)
	if err != nil {
		return false
	}
	value, _ := strconv.ParseBool(cookieVal)
	return value
}

// GetCookieDefault 获取Cookie值，如果不存在则返回默认值
func (c *Context) GetCookieDefault(name, defaultValue string) string {
	cookieVal, err := c.Cookie(name)
	if err != nil {
		return defaultValue
	}
	return cookieVal
}

// Redirect 重定向
func (c *Context) Redirect(code int, location string) {
	c.SetStatus(code)
	c.SetHeader("Location", location)
	c.writer.WriteHeader(code)
	_, _ = c.writer.Write([]byte("Redirecting to: " + location))
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

// Status 设置响应状态码并返回Context以支持链式调用
func (c *Context) Status(code int) *Context {
	c.SetStatus(code)
	return c
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
		message = "Bad request"
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

// IsAborted 检查是否已被中止
func (c *Context) IsAborted() bool {
	return c.aborted
}

// SetHandles 设置中间件链
func (c *Context) SetHandles(handlers []HandlerFunc) {
	c.handlers = handlers
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

// GetPathParam 获取路径参数
func (c *Context) GetPathParam(key string) string {
	return c.Params.ByName(key)
}

// GetPathParamInt 获取路径参数并转换为整数
func (c *Context) GetPathParamInt(key string) int {
	return c.Params.ByNameInt(key)
}

// GetPathParamInt64 获取路径参数并转换为64位整数
func (c *Context) GetPathParamInt64(key string) int64 {
	return c.Params.ByNameInt64(key)
}

// GetPathParamUint 获取路径参数并转换为无符号整数
func (c *Context) GetPathParamUint(key string) uint {
	return c.Params.ByNameUint(key)
}

// GetPathParamUint64 获取路径参数并转换为64位无符号整数
func (c *Context) GetPathParamUint64(key string) uint64 {
	return c.Params.ByNameUint64(key)
}

// GetPathParamFloat64 获取路径参数并转换为浮点数
func (c *Context) GetPathParamFloat64(key string) float64 {
	return c.Params.ByNameFloat64(key)
}

// GetPathParamBool 获取路径参数并转换为布尔值
func (c *Context) GetPathParamBool(key string) bool {
	return c.Params.ByNameBool(key)
}

// GetPathParamDefault 获取路径参数，如果不存在则返回默认值
func (c *Context) GetPathParamDefault(key, defaultValue string) string {
	return c.Params.ByNameDefault(key, defaultValue)
}

// ContentLength 获取内容长度
func (c *Context) ContentLength() int64 {
	length, _ := strconv.ParseInt(c.GetHeader("Content-Length"), 10, 64)
	return length
}

// GetContentLength 获取内容长度
func (c *Context) GetContentLength() int64 {
	cl := c.GetHeader("Content-Length")
	if cl == "" {
		return 0
	}
	n, err := strconv.ParseInt(cl, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// ContainsFileHeader 检查请求是否包含文件上传
func (c *Context) ContainsFileHeader(filename string) bool {
	_, fh, err := c.request.FormFile(filename)
	if err != nil {
		return false
	}
	return fh != nil
}

// Deadline 返回请求的截止时间
func (c *Context) Deadline() (deadline time.Time, ok bool) {
	if c.request.Context() != nil {
		return c.request.Context().Deadline()
	}
	return
}

// Done 返回当请求结束或取消时关闭的通道
func (c *Context) Done() <-chan struct{} {
	if c.request.Context() != nil {
		return c.request.Context().Done()
	}
	return nil
}

// Err 返回请求上下文的错误原因
func (c *Context) Err() error {
	if c.request.Context() != nil {
		return c.request.Context().Err()
	}
	return nil
}

// Value 返回与键关联的值
func (c *Context) Value(key interface{}) interface{} {
	if c.request.Context() != nil {
		return c.request.Context().Value(key)
	}
	return nil
}

// Copy 创建Context副本
func (c *Context) Copy() *Context {
	cp := &Context{
		request:   c.request,
		writer:    c.writer,
		method:    c.method,
		path:      c.path,
		params:    make(map[string]string),
		query:     make(url.Values),
		clientIP:  c.clientIP,
		userAgent: c.userAgent,

		statusCode: c.statusCode,
		headers:    make(map[string]string),

		store:     make(map[interface{}]interface{}),
		handlers:  c.handlers,
		index:     c.index,
		errors:    make([]error, len(c.errors)),
		aborted:   c.aborted,
		startTime: c.startTime,
		requestID: c.requestID,
		written:   c.written,
		Params:    make(Params, len(c.Params)),
	}

	// 复制参数映射
	for k, v := range c.params {
		cp.params[k] = v
	}

	// 复制查询参数
	for k, v := range c.query {
		cp.query[k] = v
	}

	// 复制头信息
	for k, v := range c.headers {
		cp.headers[k] = v
	}

	// 复制错误列表
	copy(cp.errors, c.errors)

	// 复制存储数据
	c.storeMutex.RLock()
	for k, v := range c.store {
		cp.store[k] = v
	}
	c.storeMutex.RUnlock()

	// 复制路径参数
	copy(cp.Params, c.Params)

	return cp
}

// Flash 用于存储临时消息
type Flash struct {
	Data map[string][]string
}

// NewFlash 创建一个新的Flash实例
func NewFlash() *Flash {
	return &Flash{
		Data: make(map[string][]string),
	}
}

// SetFlash 设置flash消息
func (c *Context) SetFlash(key, value string) {
	flashInterface, exists := c.Get("flash")
	if !exists {
		flash := NewFlash()
		flash.Data[key] = []string{value}
		c.Set("flash", flash)
	} else {
		flash, ok := flashInterface.(*Flash)
		if ok {
			flash.Data[key] = append(flash.Data[key], value)
		} else {
			flash := NewFlash()
			flash.Data[key] = []string{value}
			c.Set("flash", flash)
		}
	}
}

// GetFlash 获取flash消息
func (c *Context) GetFlash(key string) []string {
	flashInterface, exists := c.Get("flash")
	if !exists {
		return []string{}
	}
	flash, ok := flashInterface.(*Flash)
	if !ok {
		return []string{}
	}
	if values, ok := flash.Data[key]; ok {
		// 移除flash消息，使其只在下次请求前有效
		delete(flash.Data, key)
		return values
	}
	return []string{}
}

func (c *Context) ServeRange(ctx context.Context, reader RangeReader) {
	// 1. 基础校验
	if reader == nil {
		c.InternalServerError("range reader is nil")
		return
	}
	fileSize := reader.Size()
	if fileSize <= 0 {
		c.BadRequest("invalid data size")
		return
	}

	// 2. 设置通用响应头
	c.SetHeader("Accept-Ranges", "bytes")                                                         // 声明支持断点续传
	c.SetHeader("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", reader.Name())) // 下载文件名
	c.SetHeader("Content-Type", reader.ContentType())                                             // 数据MIME类型

	// 3. 解析Range请求头
	rangeHeader := c.GetRangeHeader()
	specs, isRangeRequest, err := ParseRange(rangeHeader, fileSize)
	if err != nil {
		// Range格式错误，返回416
		c.SetHeader("Content-Range", fmt.Sprintf("bytes */%d", fileSize))
		c.Fail(http.StatusRequestedRangeNotSatisfiable, fmt.Sprintf("invalid range: %v", err))
		return
	}

	// 4. 处理非Range请求（返回完整数据）
	if !isRangeRequest {
		c.SetStatus(http.StatusOK)
		c.SetHeader("Content-Length", strconv.FormatInt(fileSize, 10))

		// 读取完整数据
		fullReader, _, err := reader.ReadRange(ctx, 0, fileSize-1)
		if err != nil {
			c.InternalServerError(fmt.Sprintf("read full data failed: %v", err))
			return
		}
		_, err = io.Copy(c.writer, fullReader)
		if err != nil {
			return
		}
		return
	}

	// 5. 处理Range请求（返回部分数据，206状态码）
	spec := specs[0] // 主流场景仅处理单个范围
	c.SetStatus(http.StatusPartialContent)

	// 设置断点续传核心响应头
	c.SetHeader("Content-Range", fmt.Sprintf("bytes %d-%d/%d", spec.Start, spec.End, fileSize))
	c.SetHeader("Content-Length", strconv.FormatInt(spec.Length, 10))

	// 读取指定范围数据并写入响应
	rangeDataReader, _, err := reader.ReadRange(ctx, spec.Start, spec.End)
	if err != nil {
		c.InternalServerError(fmt.Sprintf("read range data failed: %v", err))
		return
	}
	_, err = io.Copy(c.writer, rangeDataReader)
	if err != nil {
		return
	}
}

// RangeSpec 表示解析后的Range范围
type RangeSpec struct {
	Start  int64 // 起始字节
	End    int64 // 结束字节（包含）
	Length int64 // 片段长度
}

// ParseRange 通用Range头解析方法（适配所有场景）
// rangeHeader: Range请求头值（如bytes=0-1024）
// totalSize: 数据总大小
// 返回：解析后的范围列表、是否是Range请求、错误
func ParseRange(rangeHeader string, totalSize int64) ([]RangeSpec, bool, error) {
	if rangeHeader == "" {
		return nil, false, nil
	}

	// 仅支持bytes类型的Range
	if !strings.HasPrefix(strings.ToLower(rangeHeader), "bytes=") {
		return nil, false, fmt.Errorf("unsupported range type (only bytes is supported): %s", rangeHeader)
	}

	// 拆分多个范围（如bytes=0-1024,2048-3072）
	parts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), ",")
	specs := make([]RangeSpec, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// 拆分起始和结束位置
		sepIdx := strings.Index(part, "-")
		if sepIdx == -1 {
			return nil, false, fmt.Errorf("invalid range format: %s", part)
		}

		startStr, endStr := part[:sepIdx], part[sepIdx+1:]
		var start, end int64 = -1, -1
		var err error

		// 解析起始位置
		if startStr != "" {
			start, err = strconv.ParseInt(startStr, 10, 64)
			if err != nil || start < 0 {
				return nil, false, fmt.Errorf("invalid start position: %s", startStr)
			}
		}

		// 解析结束位置
		if endStr != "" {
			end, err = strconv.ParseInt(endStr, 10, 64)
			if err != nil || end < 0 {
				return nil, false, fmt.Errorf("invalid end position: %s", endStr)
			}
		}

		// 处理特殊Range格式
		switch {
		// 1. 从末尾开始的范围（如bytes=-512 → 最后512字节）
		case start == -1 && end != -1:
			start = totalSize - end
			end = totalSize - 1
		// 2. 从指定位置到末尾（如bytes=1024- → 1024到末尾）
		case start != -1 && end == -1:
			end = totalSize - 1
		// 3. 无效格式
		case start == -1 && end == -1:
			return nil, false, fmt.Errorf("empty range: %s", part)
		}

		// 校验范围有效性
		if start > end || start >= totalSize || end >= totalSize {
			return nil, false, fmt.Errorf("range out of bounds: %s (total size: %d)", part, totalSize)
		}

		specs = append(specs, RangeSpec{
			Start:  start,
			End:    end,
			Length: end - start + 1,
		})
	}

	return specs, len(specs) > 0, nil
}
