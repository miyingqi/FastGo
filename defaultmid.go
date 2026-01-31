package FastGo

import (
	"LogX"
	"strconv"
	"time"
)

type MiddlewareLog struct {
	defaultLoggerMid *LogX.AsyncLogger
}

// HandleHTTP 核心中间件逻辑：采集并打印HTTP请求日志
func (m *MiddlewareLog) HandleHTTP(context ContextInterface) {
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
