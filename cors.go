package FastGo

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

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

// ... 所有的 Set 方法保持不变 ...

func (c *CorsConfig) HandleHTTP(ctx *Context) {
	origin := ctx.Request.Header.Get("Origin")

	// 如果没有 Origin 头，这不是 CORS 请求，直接继续
	if origin == "" {
		ctx.Next()
		return
	}

	// 设置基本的 CORS 头
	ctx.SetHeader("Vary", "Origin")

	// 检查源是否被允许
	if !c.isAllowedOrigin(origin) {
		ctx.SetStatus(http.StatusForbidden)
		_, _ = ctx.Writer.Write([]byte("Origin not allowed"))
		return
	}

	// 设置允许的源
	ctx.SetHeader("Access-Control-Allow-Origin", origin)

	// 如果允许凭据，则不能使用通配符
	if c.allowCredentials {
		ctx.SetHeader("Access-Control-Allow-Credentials", "true")
	}

	// 如果是预检请求
	if ctx.method == "OPTIONS" {
		c.handlePreflight(ctx)
		return
	}
	// 对于简单请求，设置暴露的头部
	c.setExposeHeaders(ctx)

	ctx.Next()
}

func (c *CorsConfig) handlePreflight(ctx *Context) {
	ctx.Abort()
	origin := ctx.Request.Header.Get("Origin")
	requestMethod := ctx.Request.Header.Get("Access-Control-Request-Method")
	requestHeaders := ctx.Request.Header.Get("Access-Control-Request-Headers")

	// 检查请求的方法是否被允许
	if requestMethod == "" || !c.isAllowedMethod(requestMethod) {
		ctx.Abort()
		ctx.SetStatus(http.StatusForbidden)
		_, _ = ctx.Writer.Write([]byte("Method not allowed"))
		return
	}

	// 检查请求的头部是否被允许
	if requestHeaders != "" {
		headers := strings.Split(requestHeaders, ",")
		for _, header := range headers {
			header = strings.TrimSpace(header)
			if !c.isAllowedHeaders(header) {
				ctx.SetStatus(http.StatusForbidden)
				_, _ = ctx.Writer.Write([]byte("Header not allowed: " + header))
				return
			}
		}
	}

	// 设置预检响应头部
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

	// 如果启用了私有网络访问
	if c.allowPrivateNetwork {
		ctx.SetHeader("Access-Control-Allow-Private-Network", "true")
	}

	// 设置暴露的头部
	if len(c.exposeHeaders) > 0 {
		ctx.SetHeader("Access-Control-Expose-Headers", strings.Join(c.exposeHeaders, ", "))
	}

	// 预检请求完成，不继续执行后续中间件
	ctx.SetStatus(http.StatusOK)
	return
}

func (c *CorsConfig) handleRequest(ctx *Context) {
	origin := ctx.Request.Header.Get("Origin")

	// 设置允许的源
	ctx.SetHeader("Access-Control-Allow-Origin", origin)

	// 如果允许凭据
	if c.allowCredentials {
		ctx.SetHeader("Access-Control-Allow-Credentials", "true")
	}

	// 设置暴露的头部
	c.setExposeHeaders(ctx)
}

func (c *CorsConfig) setExposeHeaders(ctx *Context) {
	if len(c.exposeHeaders) > 0 {
		ctx.SetHeader("Access-Control-Expose-Headers", strings.Join(c.exposeHeaders, ", "))
	}
}

// 修正 isAllowedOrigin 方法以支持正则表达式
func (c *CorsConfig) isAllowedOrigin(origin string) bool {
	if origin == "" {
		return false
	}

	// 检查静态允许的源
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

	// 检查正则表达式匹配的源
	if len(c.allowOriginRegex) != 0 {
		for _, regex := range c.allowOriginRegex {
			if regex.MatchString(origin) {
				return true
			}
		}
	}

	return false
}

// 修正 isAllowedMethod 方法
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

// 修正 isAllowedHeaders 方法
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

// SetAllowOriginRegex 添加设置正则表达式源的方法
func (c *CorsConfig) SetAllowOriginRegex(regexes []*regexp.Regexp) *CorsConfig {
	if regexes == nil || len(regexes) == 0 {
		return c
	}
	c.allowOriginRegex = regexes
	return c
}

// SetAllowPrivateNetwork 添加设置私有网络访问的方法
func (c *CorsConfig) SetAllowPrivateNetwork(allow bool) *CorsConfig {
	c.allowPrivateNetwork = allow
	return c
}
func (c *CorsConfig) SetAllowOrigins(origins ...string) *CorsConfig {
	if origins == nil || len(origins) == 0 {
		return c
	}
	c.allowOrigins = origins
	return c
}
func (c *CorsConfig) SetAllowMethods(methods ...string) *CorsConfig {
	if methods == nil || len(methods) == 0 {
		return c
	}
	c.allowMethods = methods
	return c
}
func (c *CorsConfig) SetAllowHeaders(headers ...string) *CorsConfig {
	if headers == nil || len(headers) == 0 {
		return c
	}
	c.allowHeaders = headers
	return c
}
func (c *CorsConfig) SetAllowCredentials(allow bool) *CorsConfig {
	c.allowCredentials = allow
	return c
}
func (c *CorsConfig) SetExposeHeaders(headers ...string) *CorsConfig {
	if headers == nil || len(headers) == 0 {
		return c
	}
	c.exposeHeaders = headers
	return c
}
func (c *CorsConfig) SetMaxAge(maxAge int) *CorsConfig {
	if maxAge < 0 {
		return c
	}
	c.maxAge = maxAge
	return c
}
