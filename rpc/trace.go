package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/micro/go-micro/v2/client"
	"github.com/micro/go-micro/v2/registry"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"

	"github.com/shelton-hu/logger"
	"github.com/shelton-hu/util/protoutil"
)

func recoverCallWrapper() client.CallWrapper {
	return func(cf client.CallFunc) client.CallFunc {
		return func(ctx context.Context, node *registry.Node, req client.Request, resp interface{}, opts client.CallOptions) error {
			defer func() {
				if p := recover(); p != nil {
					s := make([]byte, 2048)
					n := runtime.Stack(s, false)
					logger.Error(ctx, "rpc client exception, %s, %s", p, s[:n])
				}
			}()
			return cf(ctx, node, req, resp, opts)
		}
	}
}

func tracerCallWrapper() client.CallWrapper {
	return func(cf client.CallFunc) client.CallFunc {
		return func(ctx context.Context, node *registry.Node, req client.Request, resp interface{}, opts client.CallOptions) error {
			span := opentracing.SpanFromContext(ctx)
			request, _ := json.Marshal(req.Body())
			if span != nil {
				ext.SpanKindRPCClient.Set(span)
				ext.PeerAddress.Set(span, node.Address)
				ext.PeerHostname.Set(span, node.Id)
				span.LogKV("request", string(request))
			}
			begin := time.Now()
			err := cf(ctx, node, req, resp, opts)
			end := time.Now()
			name := fmt.Sprintf("%s.%s", req.Service(), req.Endpoint())
			afterWrapper(span, ctx, name, request, resp, float64(end.Sub(begin))/1e6, err)
			return err
		}
	}
}

func afterWrapper(span opentracing.Span, ctx context.Context, name string, request []byte, resp interface{}, spend float64, err error) {
	response, _ := protoutil.ParseProtoToString(resp.(proto.Message))
	errStr := ""
	if span != nil {
		if err != nil {
			ext.SamplingPriority.Set(span, 1)
			errStr = err.Error()
			ext.Error.Set(span, true)
			span.LogKV("error_msg", errStr)
		}
		span.LogKV("response", response)
	}
	params := string(request)
	if strings.Contains(strings.ToLower(name), "collection") {
		params = "*"
	}

	logger.Info(ctx, "%s, %s, %v, %s, %f", name, params, response, errStr, spend)
}
