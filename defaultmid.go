package FastGo

import (
	"LogX"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type MiddlewareLog struct {
	defaultLoggerMid *LogX.AsyncLogger
}

// HandleHTTP 核心中间件逻辑：采集并打印HTTP请求日志
func (m *MiddlewareLog) Handle(context *Context) {
	startTime := time.Now()
	context.Next()
	elapsed := time.Since(startTime)
	responseTime := float64(elapsed.Nanoseconds()) / 1e6 // 转毫秒

	// 采集核心日志字段
	logFields := map[string]interface{}{

		"method":        context.Method(),
		"path":          context.Path(),
		"client_ip":     context.ClientIP(), // 真实客户端IP（处理反向代理）
		"status_code":   context.StatusCode(),
		"response_time": strconv.FormatFloat(responseTime, 'f', 1, 64),
		"user_agent":    context.UserAgent(),
		"service":       "HTTP", // 服务标识
		"timestamp":     time.Now().Format("2006-01-02 15:04:05.000"),
	}

	// 正常请求：打印INFO级日志
	m.defaultLoggerMid.Info(
		"method:%s path:%s ip:%s code:%d rt:%sms user_agent:%s",
		logFields["method"],
		logFields["path"],
		logFields["client_ip"],
		logFields["status_code"],
		logFields["response_time"],
		logFields["user_agent"],
	)
}

// NewMiddlewareLog 创建日志中间件实例（初始化默认日志器）
func NewMiddlewareLog() *MiddlewareLog {
	logger := LogX.NewDefaultAsyncLogger("HTTP")
	return &MiddlewareLog{
		defaultLoggerMid: logger,
	}
}

// SetLogger 自定义日志器（支持外部替换）
func (m *MiddlewareLog) SetLogger(logger *LogX.AsyncLogger) {
	m.defaultLoggerMid = logger

}

type CorsConfig struct {
	allowOrigins        []string
	allowMethods        []string
	allowHeaders        []string
	allowCredentials    bool
	allowOriginRegex    []*regexp.Regexp
	exposeHeaders       []string
	maxAge              int
	allowPrivateNetwork bool
}

func NewCors() *CorsConfig {
	return &CorsConfig{
		allowOrigins:        make([]string, 0),
		allowMethods:        []string{"GET"},
		allowHeaders:        []string{"Accept", "Accept-Language", "Content-Language", "Content-Type", "Authorization", "X-Requested-With"},
		allowCredentials:    false,
		exposeHeaders:       []string{},
		maxAge:              600,
		allowPrivateNetwork: false,
	}
}

func (c *CorsConfig) Handle(ctx *Context) {
	origin := ctx.GetHeader("Origin")

	if origin == "" {
		ctx.Next()
		return
	}

	ctx.SetHeader("Vary", "Origin")

	if !c.isAllowedOrigin(origin) {
		ctx.SetStatus(http.StatusForbidden)
		_, _ = ctx.Write([]byte("Origin not allowed"))
		return
	}

	ctx.SetHeader("Access-Control-Allow-Origin", origin)

	if c.allowCredentials {
		ctx.SetHeader("Access-Control-Allow-Credentials", "true")
	}

	if ctx.Method() == "OPTIONS" || ctx.GetHeader("Access-Control-Request-Method") != "" {
		c.handlePreflight(ctx)
		return
	}
	c.setExposeHeaders(ctx)

	ctx.Next()
}

func (c *CorsConfig) handlePreflight(ctx *Context) {
	ctx.Abort()
	origin := ctx.GetHeader("Origin")
	requestMethod := ctx.GetHeader("Access-Control-Request-Method")
	requestHeaders := ctx.GetHeader("Access-Control-Request-Headers")

	if requestMethod == "" || !c.isAllowedMethod(requestMethod) {
		ctx.Abort()
		ctx.SetStatus(http.StatusForbidden)
		_, _ = ctx.Write([]byte("Method not allowed"))
		return
	}

	if requestHeaders != "" {
		headers := strings.Split(requestHeaders, ",")
		for _, header := range headers {
			header = strings.TrimSpace(header)
			if !c.isAllowedHeaders(header) {
				ctx.SetStatus(http.StatusForbidden)
				_, _ = ctx.Write([]byte("Header not allowed: " + header))
				return
			}
		}
	}

	ctx.SetHeader("Access-Control-Allow-Origin", origin)
	ctx.SetHeader("Access-Control-Allow-Methods", strings.Join(c.allowMethods, ", "))

	if requestHeaders != "" {
		ctx.SetHeader("Access-Control-Allow-Headers", requestHeaders)
	} else {
		ctx.SetHeader("Access-Control-Allow-Headers", strings.Join(c.allowHeaders, ", "))
	}

	if c.allowCredentials {
		ctx.SetHeader("Access-Control-Allow-Credentials", "true")
	}

	if c.maxAge > 0 {
		ctx.SetHeader("Access-Control-Max-Age", strconv.Itoa(c.maxAge))
	}

	if c.allowPrivateNetwork {
		ctx.SetHeader("Access-Control-Allow-Private-Network", "true")
	}

	if len(c.exposeHeaders) > 0 {
		ctx.SetHeader("Access-Control-Expose-Headers", strings.Join(c.exposeHeaders, ", "))
	}

	ctx.SetStatus(204)
	_, err := ctx.Write([]byte{})
	if err != nil {
		return
	}
	return
}

func (c *CorsConfig) handleRequest(ctx *Context) {
	origin := ctx.GetHeader("Origin")

	ctx.SetHeader("Access-Control-Allow-Origin", origin)

	if c.allowCredentials {
		ctx.SetHeader("Access-Control-Allow-Credentials", "true")
	}

	c.setExposeHeaders(ctx)
}

func (c *CorsConfig) setExposeHeaders(ctx *Context) {
	if len(c.exposeHeaders) > 0 {
		ctx.SetHeader("Access-Control-Expose-Headers", strings.Join(c.exposeHeaders, ", "))
	}
}

// isAllowedOrigin 检查源是否被允许
func (c *CorsConfig) isAllowedOrigin(origin string) bool {
	if origin == "" {
		return false
	}

	if len(c.allowOrigins) != 0 {
		if c.allowOrigins[0] == "*" {
			return true
		}
		for _, o := range c.allowOrigins {
			if o == origin {
				return true
			}
		}
	}

	if len(c.allowOriginRegex) != 0 {
		for _, regex := range c.allowOriginRegex {
			if regex.MatchString(origin) {
				return true
			}
		}
	}

	return false
}

// isAllowedMethod 检查方法是否被允许
func (c *CorsConfig) isAllowedMethod(method string) bool {
	if len(c.allowMethods) != 0 {
		if c.allowMethods[0] == "*" {
			return true
		}
		for _, m := range c.allowMethods {
			if strings.ToUpper(m) == strings.ToUpper(method) {
				return true
			}
		}
	}
	return false
}

// isAllowedHeaders 检查头部是否被允许
func (c *CorsConfig) isAllowedHeaders(header string) bool {
	if len(c.allowHeaders) != 0 {
		if c.allowHeaders[0] == "*" {
			return true
		}
		header = strings.ToLower(strings.TrimSpace(header))
		for _, h := range c.allowHeaders {
			if strings.ToLower(h) == header {
				return true
			}
		}
	}
	return false
}

// SetAllowOriginRegex 设置允许的源的正则表达式
func (c *CorsConfig) SetAllowOriginRegex(regexes []*regexp.Regexp) *CorsConfig {
	if regexes == nil || len(regexes) == 0 {
		return c
	}
	c.allowOriginRegex = regexes
	return c
}

// SetAllowPrivateNetwork 设置是否允许私有网络访问
func (c *CorsConfig) SetAllowPrivateNetwork(allow bool) *CorsConfig {
	c.allowPrivateNetwork = allow
	return c
}

// SetAllowOrigins 设置允许的源
func (c *CorsConfig) SetAllowOrigins(origins ...string) *CorsConfig {
	if origins == nil || len(origins) == 0 {
		return c
	}
	c.allowOrigins = origins
	return c
}

// SetAllowMethods 设置允许的方法
func (c *CorsConfig) SetAllowMethods(methods ...string) *CorsConfig {
	if methods == nil || len(methods) == 0 {
		return c
	}
	c.allowMethods = methods
	return c
}

// SetAllowHeaders 设置允许的头部
func (c *CorsConfig) SetAllowHeaders(headers ...string) *CorsConfig {
	if headers == nil || len(headers) == 0 {
		return c
	}
	c.allowHeaders = headers
	return c
}

// SetAllowCredentials 设置是否允许凭据
func (c *CorsConfig) SetAllowCredentials(allow bool) *CorsConfig {
	c.allowCredentials = allow
	return c
}

// SetExposeHeaders 设置暴露的头部
func (c *CorsConfig) SetExposeHeaders(headers ...string) *CorsConfig {
	if headers == nil || len(headers) == 0 {
		return c
	}
	c.exposeHeaders = headers
	return c
}

// SetMaxAge 设置预检请求的最大生存时间
func (c *CorsConfig) SetMaxAge(maxAge int) *CorsConfig {
	if maxAge < 0 {
		return c
	}
	c.maxAge = maxAge
	return c
}
