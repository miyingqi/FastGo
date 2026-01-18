package FastGo

import "strconv"

type MiddlewareLog struct {
	defaultLoggerMid *AsyncLogger
}

func (m *MiddlewareLog) HandleHTTP(context *Context) {
	protocol := "HTTP"
	if context.Request.TLS != nil {
		protocol = "HTTPS"
	}
	m.defaultLoggerMid.InfoWithModule(protocol, context.clientIP+"-"+context.method+" ")
	context.Next()
	m.defaultLoggerMid.InfoWithModule(protocol, context.clientIP+"-"+context.method+" "+strconv.Itoa(context.statusCode)+" "+context.StatusString(context.statusCode))
}

func NewMiddlewareLog() *MiddlewareLog {
	return &MiddlewareLog{
		NewAsyncLoggerSP(INFO),
	}
}
