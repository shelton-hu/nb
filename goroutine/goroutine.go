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
		if ok, err := goroutine(function, args...); !ok {
			panic("goroutine: fatal error: params not invaild or function panic")
		} else if err != nil {
			logger.Error(context.Background(), "goroutine: error: %s", err.Error())
		}
	}()
}

// 安全的调用制定的函数
//
// function需要为函数类型
// args可以为任意类型，但是如果有context.Context，需要放在第一个参数
// 如果有error，需要放在最后一个参数
//
// 该函数有两个返回值
// 第一个返回值为bool类型，表示参数校验是否有问题
// 第二个返回值为error类型，表示函数本身执行过程中panic了，或者返回了error
//
func goroutine(function interface{}, args ...interface{}) (bool, error) {
	// 从入参中获取ctx，或者新建一个ctx
	var ctx context.Context
	if len(args) > 0 {
		var ok bool
		if ctx, ok = args[0].(context.Context); !ok {
			ctx = context.Background()
		}
	} else {
		ctx = context.Background()
	}

	// 捕获异常，防止因为协程的异常，导致整个进程退出
	defer func() {
		if e := recover(); e != nil {
			logger.Error(ctx, "goroutine: fatal error: %v", e)
		}
	}()

	// 将function反射成func类型
	fn := reflect.ValueOf(function)
	if fn.Kind() != reflect.Func {
		logger.Error(ctx, "goroutine: fatal error: param function is not kind of func")
		return false, nil
	}

	if fn.Type().NumIn() != len(args) {
		logger.Error(ctx, "goroutine: fatal error: function args's length %d not equal to provide length %d, func: %s, args: %v", fn.Type().NumIn(), len(args), runtime.FuncForPC(reflect.ValueOf(function).Pointer()).Name(), args)
		return false, nil
	}

	// 将args反射成reflect.Value
	argValues := make([]reflect.Value, 0, fn.Type().NumIn())
	for i := 0; i < len(args); i++ {
		if args[i] == nil {
			argValues = append(argValues, reflect.Zero(fn.Type().In(i)))
		} else {
			argValues = append(argValues, reflect.ValueOf(args[i]))
		}
	}

	// 执行fn
	reply := fn.Call(argValues)

	// 将执行结果的error返回
	if len(reply) > 0 {
		if err, ok := reply[len(reply)-1].Interface().(error); ok {
			return true, err
		}
	}

	return true, nil
}
