package FastGo

import "regexp"

type CorsConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	AllowCredentials bool
	AllowOriginRegex []*regexp.Regexp
	ExposeHeaders    []string
	MaxAge           int
}

func NewCors() *CorsConfig {
	return &CorsConfig{
		AllowOrigins:     make([]string, 0),
		AllowMethods:     []string{"GET"},
		AllowHeaders:     []string{"Accept", "Accept-Language", "Content-Language", "Content-Type"},
		AllowCredentials: false,
		ExposeHeaders:    nil,
		MaxAge:           600,
	}
}
func (c *CorsConfig) SetCors(allowOrigins []string,
	allowMethods []string, allowHeaders []string,
	allowCredentials bool, allowOriginRegex []*regexp.Regexp,
	exposeHeaders []string, maxAge int) {

}
