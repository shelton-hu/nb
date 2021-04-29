package goroutine

import (
	"context"
	"reflect"
	"runtime"

	"github.com/shelton-hu/logger"
)

// 开启协程调用指定函数
func Go(function interface{}, args ...interface{}) {
	go func() {
		goroutine(function, args...)
	}()
}

func goroutine(function interface{}, args ...interface{}) bool {
	var ctx context.Context
	if len(args) > 0 {
		var ok bool
		if ctx, ok = args[0].(context.Context); !ok {
			ctx = context.TODO()
		}
	} else {
		ctx = context.TODO()
	}

	defer func() {
		if e := recover(); e != nil {
			logger.Error(ctx, "%v", e)
		}
	}()

	value := reflect.ValueOf(function)
	if value.Kind() != reflect.Func {
		logger.Error(ctx, "err: param function is not function, args: %v", args)
		return true
	}

	if value.Type().NumIn() != len(args) {
		logger.Error(ctx, "err: param length %d not equal to provide length %d, func: %s, args: %v", value.Type().NumIn(), len(args), runtime.FuncForPC(reflect.ValueOf(function).Pointer()).Name(), args)
		return true
	}

	argValues := make([]reflect.Value, 0, value.Type().NumIn())
	for i := 0; i < len(args); i++ {
		if args[i] == nil {
			argValues = append(argValues, reflect.Zero(value.Type().In(i)))
		} else {
			argValues = append(argValues, reflect.ValueOf(args[i]))
		}
	}

	reply := value.Call(argValues)

	if len(reply) > 0 {
		if _, ok := reply[len(reply)-1].Interface().(error); ok {
			return false
		}
	}

	return true
}
