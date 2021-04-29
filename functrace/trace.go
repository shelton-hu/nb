package functrace

import (
	"context"
	"runtime"

	microTrace "github.com/micro/go-plugins/wrapper/trace/opentracing/v2"
	opentracing "github.com/opentracing/opentracing-go"
)

func Trace(ctx context.Context) context.Context {
	name, file, line := runFunc()
	ctx, span, _ := microTrace.StartSpanFromContext(ctx, opentracing.GlobalTracer(), name)
	defer span.Finish()
	span.LogKV("file", file)
	span.LogKV("line", line)
	return ctx
}

func runFunc() (name string, file string, line int) {
	pc := make([]uintptr, 1)
	runtime.Callers(3, pc)
	f := runtime.FuncForPC(pc[0])
	name = f.Name()
	file, line = f.FileLine(pc[0])
	return
}
