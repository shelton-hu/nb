package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	microTrace "github.com/micro/go-plugins/wrapper/trace/opentracing/v2"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"

	"github.com/shelton-hu/logger"
	"github.com/shelton-hu/util/strutil"
)

const (
	_ContentType = "Content-Type"

	_ContentTypeJson = "application/json"
	_ContentTypeForm = "application/x-www-form-urlencoded"
)

type Client struct {
	// 公共参数--公共域名
	//
	// url = host + pathPrefix + path + ? + query
	host string

	// 公共参数--公共路径前缀
	pathPrefix string

	// 公共参数--公共请求头
	//
	// 当私有请求头与公共请求头冲突时
	// 针对该请求，私有会覆盖公共
	header http.Header

	// 公共参数--公共超时时间
	//
	// 当私有超时时间与公共超时时间冲突时
	// 针对该请求，私有会覆盖公共
	timeout time.Duration
}

// 新建一个客户端
//
// host为整个Client的域名，后续无法更改
// 默认的超时时间为30s
//
// 可以通过opts来修改修改各个默认值
//
// 修改公共路径前缀
// SetPublicPathPrefix(string) ClientOptions
//
// 设置公共请求头
// SetPublicHeader(map[string]string) ClientOptions
//
// 设置公共超时时间
// SetPublicTimeout(time.Duration) ClientOptions
//
func NewClient(host string, opts ...ClientOptions) *Client {
	c := &Client{
		host:    host,
		header:  make(http.Header),
		timeout: 30 * time.Second,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

type ClientOptions func(*Client)

// 修改公共路径前缀
func SetPublicPathPrefix(pathPrefix string) ClientOptions {
	return func(c *Client) {
		c.pathPrefix = pathPrefix
	}
}

// 设置公共请求头
func SetPublicHeader(key, value string) ClientOptions {
	return func(c *Client) {
		c.header.Add(key, value)
	}
}

// 设置公共超时时间
func SetPublicTimeout(timeout time.Duration) ClientOptions {
	return func(c *Client) {
		c.timeout = timeout
	}
}

// 重置请求头
func (c *Client) SetHeader(key, value string) *Client {
	c.header.Set(key, value)
	return c
}

type Request struct {
	// 公共参数
	public *Client

	// 请求方法
	method string
	// 请求路径后缀
	path string
	// 请求url参数
	query url.Values
	// 请求头
	header http.Header
	// 请求体
	body []byte
	// 超时时间
	timeout time.Duration
}

// 新建一个请求实例
func (c *Client) NewRequest() *Request {
	return &Request{
		public: c,
		query:  make(url.Values),
		header: make(http.Header),
	}
}

type RequestOptions func(*Request)

// 设置请求方法
func SetMethod(method string) RequestOptions {
	return func(r *Request) {
		r.method = method
	}
}

// 设置请求路径
func SetPath(path string) RequestOptions {
	return func(r *Request) {
		r.path = path
	}
}

// 设置请求url参数
func SetQuery(key, value string) RequestOptions {
	return func(r *Request) {
		r.query.Add(key, value)
	}
}

// 设置请求头
func SetHeader(key, value string) RequestOptions {
	return func(r *Request) {
		r.header.Add(key, value)
	}
}

// 设置请求体
func SetBody(body []byte) RequestOptions {
	return func(r *Request) {
		r.body = body
	}
}

// 设置请求体表单
func SetForm(form url.Values) RequestOptions {
	return func(r *Request) {
		r.body = []byte(form.Encode())
	}
}

type Cook struct {
	// 请求方法
	method string
	// 请求完整url
	url string
	// 请求头
	header http.Header
	// 请求体
	body []byte
	// 超时时间
	timeout time.Duration
}

// 解析请求参数，并生成发送请求实例
func (r *Request) Parse(opts ...RequestOptions) *Cook {
	for _, opt := range opts {
		opt(r)
	}
	return r.build()
}

// 解析Get请求参数，并生成发送请求实例
func (r *Request) ParseGet(path string, query url.Values, opts ...RequestOptions) *Cook {
	opts = append(opts, SetMethod(http.MethodGet), SetPath(path))
	for key, values := range query {
		for _, value := range values {
			opts = append(opts, SetQuery(key, value))
		}
	}

	for _, opt := range opts {
		opt(r)
	}

	return r.build()
}

// 解析PostJson请求参数，并生成发送请求实例
func (r *Request) ParsePostJson(path string, body []byte, opts ...RequestOptions) *Cook {
	opts = append(opts, SetMethod(http.MethodPost), SetPath(path), SetHeader(_ContentType, _ContentTypeJson), SetBody(body))

	for _, opt := range opts {
		opt(r)
	}

	return r.build()
}

// 解析PostForm请求参数，并生成发送请求实例
func (r *Request) ParsePostForm(path string, form url.Values, opts ...RequestOptions) *Cook {
	opts = append(opts, SetMethod(http.MethodPost), SetPath(path), SetHeader(_ContentType, _ContentTypeForm), SetForm(form))

	for _, opt := range opts {
		opt(r)
	}

	return r.build()
}

// 生成发送请求实例
func (r *Request) build() *Cook {
	cook := &Cook{
		method:  r.method,
		header:  r.public.header,
		body:    r.body,
		timeout: r.public.timeout,
	}

	url := strutil.Join(r.public.host, r.public.pathPrefix, r.path)
	if len(r.query) > 0 {
		url = strutil.Join(url, r.query.Encode())
	}
	cook.url = url

	for key, values := range r.header {
		for _, value := range values {
			cook.header.Add(key, value)
		}
	}

	if r.timeout > 0 {
		cook.timeout = r.timeout
	}

	return cook
}

// 发送请求
func (c *Cook) Do(ctx context.Context, returnObj interface{}) (code int, body []byte, err error) {
	// 发送请求
	code, body, err = c.do(ctx)
	if err != nil {
		logger.Error(ctx, "http: request error: %s", err.Error())
		return code, body, err
	}

	// 解码响应体
	if returnObj != nil {
		if err := json.Unmarshal(body, &returnObj); err != nil {
			logger.Error(ctx, "http: unmarshal response body failed, error: %s", err.Error())
			return code, body, err
		}
	}

	return code, body, nil
}

// 发送请求
func (c *Cook) do(ctx context.Context) (code int, body []byte, err error) {
	// 新建一个请求实例
	req, err := http.NewRequest(c.method, c.url, bytes.NewReader(c.body))
	if err != nil {
		logger.Error(ctx, "http: do request failed, error: %s", err.Error())
		return code, body, err
	}
	req.Header = c.header

	// 记录trace
	name := fmt.Sprintf("%s://%s%s", req.URL.Scheme, req.URL.Host, req.URL.EscapedPath())
	_, span, err := microTrace.StartSpanFromContext(ctx, opentracing.GlobalTracer(), name)
	if err != nil {
		logger.Error(ctx, "http: start span failed, error: %s", err.Error())
		return code, body, err
	}
	defer span.Finish()
	span.LogKV("request", string(c.body))

	// 发送请求
	resp, err := (&http.Client{Timeout: c.timeout}).Do(req)
	if err != nil {
		logger.Error(ctx, "http: do request failed, error: %s", err.Error())
		return code, body, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Error(ctx, "http: close response body failed, error: %s", err.Error())
		}
	}()

	// 读取响应体
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Error(ctx, "http: read body from response failed, error: %s", err.Error())
		return code, body, err
	}

	// 记录trace
	ext.HTTPStatusCode.Set(span, uint16(resp.StatusCode))
	ext.HTTPMethod.Set(span, c.method)
	if resp.StatusCode >= http.StatusBadRequest {
		ext.SamplingPriority.Set(span, 1)
		ext.Error.Set(span, true)
	}
	span.LogKV("response", string(body))

	return resp.StatusCode, body, nil
}
