package functrace

import (
	"context"
	"runtime"

	microTrace "github.com/micro/go-plugins/wrapper/trace/opentracing/v2"
	opentracing "github.com/opentracing/opentracing-go"

	"github.com/shelton-hu/logger"
)

func Trace(ctx context.Context) context.Context {
	name, file, line := runFunc()
	ctxWithTrace, span, err := microTrace.StartSpanFromContext(ctx, opentracing.GlobalTracer(), name)
	if err != nil {
		logger.Error(ctx, err.Error())
		return ctx
	}
	defer span.Finish()
	span.LogKV("file", file)
	span.LogKV("line", line)
	return ctxWithTrace
}

func runFunc() (name string, file string, line int) {
	pc := make([]uintptr, 1)
	runtime.Callers(3, pc)
	f := runtime.FuncForPC(pc[0])
	name = f.Name()
	file, line = f.FileLine(pc[0])
	return
}
