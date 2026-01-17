package FastGo

import "strconv"

type LogMid struct {
	defaultLoggerMid *AsyncLogger
}

func NewLogMid() *LogMid {
	return &LogMid{
		NewAsyncLoggerSP(INFO),
	}
}
func (l *LogMid) HandleHTTP(ctx *Context) {
	l.defaultLoggerMid.Info(ctx.clientIP + "-" + ctx.method + " ")
	ctx.Next()
	l.defaultLoggerMid.Info(ctx.clientIP + "-" + ctx.method + " " + strconv.Itoa(ctx.statusCode) + " " + ctx.StatusString(ctx.statusCode))
}
