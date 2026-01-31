package FastGo

import (
	"context"
	"mime/multipart"
	"net/http"
	"time"
)

// ContextInterface 定义了Context必须实现的方法集合
// 这个接口包含了所有请求处理、响应生成、参数获取等核心方法
type ContextInterface interface {
	// 生命周期控制
	Next()
	Abort()
	IsAborted() bool
	SetHandles(handlers []HandlerFunc)

	// 请求信息
	Method() string
	Path() string
	ClientIP() string
	StatusCode() int
	UserAgent() string
	Request() *http.Request

	// 响应处理
	SetHeader(key string, values ...string)
	SetStatus(code int)
	Write(bytes []byte) (int, error)
	SendString(code int, body string)
	SendJson(code int, jsonData FJ)
	SendHtml(code int, html string)
	SendXml(code int, xmlData interface{})
	JSONP(code int, callback string, data interface{})
	Data(code int, contentType string, data []byte)
	File(filepath string)
	SendSuccess(data interface{})
	SendError(code int, message string, errData ...interface{})
	Status(code int) ContextInterface
	NotFound(message string)
	BadRequest(message string)
	Unauthorized(message string)
	Forbidden(message string)
	InternalServerError(message string)
	Fail(code int, errorMsg string)
	FailWithError(code int, err error)

	// 参数处理
	Query(key string) string
	QueryDefault(key, defaultValue string) string
	QueryArray(key string) []string
	QueryBool(key string) bool
	QueryInt(key string) int
	QueryInt64(key string) int64
	QueryFloat64(key string) float64
	QueryUint(key string) uint
	QueryUint64(key string) uint64
	QuerySlice(key string) []string
	DefaultQueryWithSlice(key string, defaultValue []string) []string
	GetQueryInt64Default(key string, defaultValue int64) int64
	GetQueryFloat64Default(key string, defaultValue float64) float64
	GetQueryBoolDefault(key string, defaultValue bool) bool

	// 路径参数
	SetParam(key string, value string)
	SetParams(params Params)
	GetPathParam(key string) string
	GetPathParamInt(key string) int
	GetPathParamInt64(key string) int64
	GetPathParamUint(key string) uint
	GetPathParamUint64(key string) uint64
	GetPathParamFloat64(key string) float64
	GetPathParamBool(key string) bool
	GetPathParamDefault(key, defaultValue string) string

	// 请求头处理
	GetHeader(key string) string
	Host() string
	Protocol() string
	Referer() string
	Referrer() string
	Accept() string
	AcceptEncoding() string
	AcceptLanguage() string
	ContentType() string
	Authorization() string
	Range() string
	XForwardedFor() string
	XRealIP() string
	XRequestedWith() string
	GetRangeHeader() string
	GetAcceptRanges() bool
	GetAcceptEncoding() string
	GetIfMatch() string
	GetIfNoneMatch() string
	GetIfModifiedSince() string
	GetIfUnmodifiedSince() string
	GetIfRange() string
	GetConnection() string
	GetCacheControl() string
	GetPragma() string
	GetUpgrade() string
	GetTransferEncoding() string

	// HTTP方法判断
	IsMethod(method string) bool
	IsGet() bool
	IsPost() bool
	IsPut() bool
	IsDelete() bool
	IsPatch() bool
	IsHead() bool
	IsOptions() bool
	IsAjax() bool
	IsJSON() bool
	IsWebSocket() bool
	IsGzip() bool
	IsDeflate() bool

	// 表单和请求体处理
	PostForm(key string) string
	PostFormDefault(key, defaultValue string) string
	GetPostFormInt64(key string) int64
	GetPostFormFloat64(key string) float64
	GetPostFormBool(key string) bool
	GetPostFormInt64Default(key string, defaultValue int64) int64
	GetPostFormFloat64Default(key string, defaultValue float64) float64
	GetPostFormBoolDefault(key string, defaultValue bool) bool
	FormFile(name string) (multipart.File, *multipart.FileHeader, error)
	MultipartForm() (*multipart.Form, error)
	Body() ([]byte, error)
	BindJSON(obj interface{}) error
	BindJSONStrict(obj interface{}) error
	ShouldBindJSON(obj interface{}) error
	BindXML(obj interface{}) error
	ShouldBindXML(obj interface{}) error
	ShouldBindQuery(obj interface{}) error
	ShouldBindForm(obj interface{}) error

	// Cookie处理
	Cookie(name string) (string, error)
	SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool)
	HasCookie(name string) bool
	GetCookieInt(name string) int
	GetCookieInt64(name string) int64
	GetCookieFloat64(name string) float64
	GetCookieBool(name string) bool
	GetCookieDefault(name, defaultValue string) string

	// 重定向和错误处理
	Redirect(code int, location string)

	// 数据存储
	Get(key interface{}) (value interface{}, exists bool)
	Set(key, value interface{})
	MustGet(key interface{}) interface{}
	GetString(key interface{}) string
	GetInt(key interface{}) int
	GetInt64(key interface{}) int64
	GetFloat64(key interface{}) float64
	GetBool(key interface{}) bool

	// 通用头部和内容长度处理
	ContentLength() int64
	GetContentLength() int64
	ContainsFileHeader(filename string) bool

	// 上下文接口实现
	Deadline() (deadline time.Time, ok bool)
	Done() <-chan struct{}
	Err() error
	Value(key interface{}) interface{}

	// 辅助和工具方法
	Copy() ContextInterface
	SetFlash(key, value string)
	GetFlash(key string) []string
	ServeRange(ctx context.Context, reader RangeReader)
}

// ============================================================================
// 类型定义
// ============================================================================

type FJ map[string]interface{}

// HandlerFunc 定义处理函数类型，使用ContextInterface而不是具体Context
type HandlerFunc func(ContextInterface)

// HandlerStruct 定义处理结构体接口，使用ContextInterface
type HandlerStruct interface {
	HandleHTTP(ContextInterface)
}

func (h HandlerFunc) HandleHTTP(c ContextInterface) {
	h(c)
}
