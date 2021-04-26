package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	microTrace "github.com/micro/go-plugins/wrapper/trace/opentracing/v2"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"

	"github.com/shelton-hu/logger"
	"github.com/shelton-hu/util/netutil"
)

const (
	_ContentType = "Content-Type"

	_ContentTypeJson = "application/json"
	_ContentTypeForm = "application/x-www-form-urlencoded"
)

type Client struct {
	host       string
	pathPrefix string
	header     map[string]string
	timeout    time.Duration

	privateArg PrivateArg

	request  Request
	response Response
}

type PrivateArg struct {
	method  string
	path    string
	query   map[string]string
	header  map[string]string
	body    []byte
	timeout time.Duration
}

type Request struct {
	method  string
	url     string
	header  map[string]string
	body    []byte
	timeout time.Duration
}

type Response struct {
	code int
	body []byte
}

type Options func(*Client)

func NewClient(host string, opts ...Options) *Client {
	return &Client{
		host:    host,
		header:  make(map[string]string),
		timeout: 30 * time.Second,
		request: Request{
			header: make(map[string]string),
		},
	}
}

func SetPublicPathPrefix(pathPrefix string) Options {
	return func(c *Client) {
		c.pathPrefix = pathPrefix
	}
}

func SetPublicHeader(header map[string]string) Options {
	return func(c *Client) {
		for key, val := range header {
			c.header[key] = val
		}
	}
}

func SetPublicTimeout(timeout time.Duration) Options {
	return func(c *Client) {
		c.timeout = timeout
	}
}

func (c *Client) resetPrivateArg() {
	c.privateArg = PrivateArg{}
}

func (c *Client) SetMethod(method string) *Client {
	c.privateArg.method = method
	return c
}

func (c *Client) SetPath(path string) *Client {
	c.privateArg.path = path
	return c
}

func (c *Client) SetQuery(query map[string]string) *Client {
	if c.privateArg.query == nil {
		c.privateArg.query = make(map[string]string)
	}
	for key, val := range query {
		c.privateArg.query[key] = val
	}
	return c
}

func (c *Client) SetHeader(header map[string]string) *Client {
	if c.privateArg.header == nil {
		c.privateArg.header = make(map[string]string)
	}
	for key, val := range header {
		c.privateArg.header[key] = val
	}
	return c
}

func (c *Client) SetBody(body []byte) *Client {
	c.privateArg.body = body
	return c
}

func (c *Client) SetTimeout(timeout time.Duration) *Client {
	c.privateArg.timeout = timeout
	return c
}

func (c *Client) resetRequest() {
	c.request = Request{
		timeout: c.timeout,
	}
	for key, val := range c.privateArg.header {
		c.request.header[key] = val
	}
}

func (c *Client) buildRequest(ctx context.Context) (err error) {
	c.resetRequest()
	c.request.method = c.privateArg.method
	if c.request.method == "" {
		c.request.method = http.MethodHead
	}
	c.request.url, err = netutil.BuildUrl(c.host+c.pathPrefix+c.privateArg.path, c.privateArg.query)
	if err != nil {
		logger.Error(ctx, err.Error())
		return err
	}
	for key, val := range c.header {
		c.request.header[key] = val
	}
	if c.privateArg.timeout > 0 {
		c.request.timeout = c.privateArg.timeout
	}
	c.resetPrivateArg()
	return nil
}

func (c *Client) resetResponse() {
	c.response = Response{}
}

func (c *Client) Do(ctx context.Context, method string, path string, query map[string]string, header map[string]string, body []byte, returnObj interface{}) error {
	c.SetMethod(http.MethodGet).SetPath(path).SetQuery(query).SetHeader(header).SetBody(body)

	if err := c.buildRequest(ctx); err != nil {
		logger.Error(ctx, err.Error())
		return err
	}

	if err := c.doRequest(ctx); err != nil {
		logger.Error(ctx, err.Error())
		return err
	}

	if !c.isOk() {
		return fmt.Errorf("request exception: [%d]%s", c.response.code, string(c.response.body))
	}

	if returnObj != nil {
		if err := json.Unmarshal(c.response.body, &returnObj); err != nil {
			logger.Error(ctx, err.Error())
			return err
		}
	}

	return nil
}

func (c *Client) Get(ctx context.Context, path string, query map[string]string, returnObj interface{}) error {
	return c.Do(ctx, http.MethodGet, path, query, nil, nil, returnObj)
}

func (c *Client) PostJson(ctx context.Context, path string, body []byte, returnObj interface{}) error {
	return c.Do(ctx, http.MethodPost, path, nil, map[string]string{_ContentType: _ContentTypeJson}, body, returnObj)
}

func (c *Client) PostForm(ctx context.Context, path string, form map[string]string, returnObj interface{}) error {
	return c.Do(ctx, http.MethodPost, path, nil, map[string]string{_ContentType: _ContentTypeForm}, []byte(netutil.BuildQuery(form)), returnObj)
}

func (c *Client) doRequest(ctx context.Context) error {
	c.resetResponse()

	req, err := http.NewRequest(c.request.method, c.request.url, bytes.NewReader(c.request.body))
	if err != nil {
		logger.Error(ctx, err.Error())
		return err
	}

	name := fmt.Sprintf("%s://%s%s", req.URL.Scheme, req.URL.Host, req.URL.EscapedPath())
	_, span, err := microTrace.StartSpanFromContext(ctx, opentracing.GlobalTracer(), name)
	if err != nil {
		logger.Error(ctx, err.Error())
		return err
	}
	defer span.Finish()

	span.LogKV("request", string(c.request.body))

	client := &http.Client{
		Timeout: c.request.timeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.Error(ctx, err.Error())
		return err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Error(ctx, err.Error())
		return err
	}

	ext.HTTPStatusCode.Set(span, uint16(resp.StatusCode))
	ext.HTTPMethod.Set(span, c.request.method)
	if resp.StatusCode >= http.StatusBadRequest {
		ext.SamplingPriority.Set(span, 1)
		ext.Error.Set(span, true)
	}
	span.LogKV("response", string(respBody))

	c.response = Response{
		code: resp.StatusCode,
		body: respBody,
	}

	return nil
}

func (c *Client) isOk() bool {
	return c.response.code == http.StatusOK || c.response.code == http.StatusCreated
}
