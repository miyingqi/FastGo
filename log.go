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
	l.defaultLoggerMid.Info(ctx.GetClientIP() + "-" + ctx.GetMethod() + " ")
	ctx.Next()
	l.defaultLoggerMid.Info(ctx.GetClientIP() + "-" + ctx.GetMethod() + " " + strconv.Itoa(ctx.GetStatus()) + " " + ctx.StatusString(ctx.GetStatus()))
}
