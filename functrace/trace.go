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
	var pc uintptr
	pc, file, line, _ = runtime.Caller(2)
	f := runtime.FuncForPC(pc)
	name = f.Name()
	return
}
