package FastGo

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/miyingqi/FastGo/internal/mcontext"
)

type FJ map[string]interface{}
type HandlerFunc func(ctx *Context)
type HandleFuncChain []HandlerFunc

type Middleware interface {
	HandleHTTP(Mcontext *Context)
}

type Context struct {
	ctx *mcontext.Mcontext
	// 处理器链
	Middlewares []Middleware
}

func NewContext() *Context {
	return &Context{
		ctx: mcontext.NewContext(),
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
	c.ctx.Headers[key] = value
	// 直接写入底层的 ResponseWriter
	c.ctx.Writer.Header().Set(key, value)
}

// StatusString 获取状态码文本
func (c *Context) StatusString(code int) string {
	return http.StatusText(code)
}

// SetStatus 仅设置状态码变量，不立即写入
func (c *Context) SetStatus(code int) {
	c.ctx.StatusCode = code
}

// SetHandles 设置中间件链
func (c *Context) SetHandles(mids []Middleware) {
	c.Middlewares = mids
}

// SetParam 设置单个路径参数（如 id -> 123）
func (c *Context) SetParam(key, value string) {
	c.ctx.Params[key] = value
}

// GetStatus
func (c *Context) GetStatus() int {
	return c.ctx.StatusCode
}

// GetMethod
func (c *Context) GetMethod() string {
	return c.ctx.Method
}

func (c *Context) GetPath() string {
	return c.ctx.Path
}

// GetHeader 获取单个响应头，不存在则返回空字符串
func (c *Context) GetHeader(key string) string {
	return c.ctx.Request.Header.Get(key)
}

// GetParam 获取单个路径参数，不存在则返回空字符串
func (c *Context) GetParam(key string) string {
	return c.ctx.Params[key]
}

// GetParamOrDefault 获取路径参数，不存在则返回默认值
func (c *Context) GetParamOrDefault(key, defaultValue string) string {
	if val, ok := c.ctx.Params[key]; ok {
		return val
	}
	return defaultValue
}

// GetParams 获取所有路径参数的副本（避免外部修改内部map）
func (c *Context) GetParams() map[string]string {
	paramsCopy := make(map[string]string, len(c.ctx.Params))
	for k, v := range c.ctx.Params {
		paramsCopy[k] = v
	}
	return paramsCopy
}

// GetQuery
func (c *Context) GetQuery(key string) string {
	return c.ctx.Query.Get(key)
}

// GetClientIP ClientIP
func (c *Context) GetClientIP() string {
	// 优先检查 X-Forwarded-For 头部
	xForwardedFor := c.ctx.Request.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		// 取第一个 IP（如果有多个代理服务器）
		ips := strings.Split(xForwardedFor, ",")
		ip := strings.TrimSpace(ips[0])
		if ip != "" {
			return ip
		}
	}

	// 检查 X-Real-IP 头部
	xRealIP := c.ctx.Request.Header.Get("X-Real-IP")
	if xRealIP != "" {
		return xRealIP
	}

	// 最后使用 RemoteAddr
	remoteAddr := c.ctx.Request.RemoteAddr
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}

	return remoteAddr
}

// Write 写入响应体（辅助方法，通常不直接暴露）
func (c *Context) Write(bytes []byte) (int, error) {
	// 如果还没写入状态码，先写入
	if !c.ctx.Written {
		c.ctx.Writer.WriteHeader(c.ctx.StatusCode)
		c.ctx.Written = true
	}
	return c.ctx.Writer.Write(bytes)
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
	c.ctx.Index++
	s := len(c.Middlewares)
	for ; c.ctx.Index < s; c.ctx.Index++ {
		// 如果已经中止，停止执行
		if c.ctx.Aborted {
			return
		}
		c.Middlewares[c.ctx.Index].HandleHTTP(c)
	}
}

// Abort 中止后续处理器的执行
func (c *Context) Abort() {
	c.ctx.Aborted = true
}

func (c *Context) Reset(writer http.ResponseWriter, request *http.Request) {
	c.ctx.Reset(writer, request)
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
