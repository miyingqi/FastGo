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

func (c *CorsConfig) HandleHTTP(ctx *Context) {
	origin := ctx.request.Header.Get("Origin")

	if origin == "" {
		ctx.Next()
		return
	}

	ctx.SetHeader("Vary", "Origin")

	if !c.isAllowedOrigin(origin) {
		ctx.SetStatus(http.StatusForbidden)
		_, _ = ctx.writer.Write([]byte("Origin not allowed"))
		return
	}

	ctx.SetHeader("Access-Control-Allow-Origin", origin)

	if c.allowCredentials {
		ctx.SetHeader("Access-Control-Allow-Credentials", "true")
	}

	if ctx.method == "OPTIONS" || ctx.request.Header.Get("Access-Control-Request-Method") != "" {
		c.handlePreflight(ctx)
		return
	}
	c.setExposeHeaders(ctx)

	ctx.Next()
}

func (c *CorsConfig) handlePreflight(ctx *Context) {
	ctx.Abort()
	origin := ctx.request.Header.Get("Origin")
	requestMethod := ctx.request.Header.Get("Access-Control-Request-Method")
	requestHeaders := ctx.request.Header.Get("Access-Control-Request-Headers")

	if requestMethod == "" || !c.isAllowedMethod(requestMethod) {
		ctx.Abort()
		ctx.SetStatus(http.StatusForbidden)
		_, _ = ctx.writer.Write([]byte("Method not allowed"))
		return
	}

	if requestHeaders != "" {
		headers := strings.Split(requestHeaders, ",")
		for _, header := range headers {
			header = strings.TrimSpace(header)
			if !c.isAllowedHeaders(header) {
				ctx.SetStatus(http.StatusForbidden)
				_, _ = ctx.writer.Write([]byte("Header not allowed: " + header))
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
	ctx.Write([]byte{})
	return
}

func (c *CorsConfig) handleRequest(ctx *Context) {
	origin := ctx.request.Header.Get("Origin")

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
