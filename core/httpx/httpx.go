// Package httpx 提供 pan-go 各网盘模块共享的 HTTP 执行器。
//
// 只负责执行：拼 URL、编码请求体、注入公共头、传输层重试、请求日志。
// 鉴权与业务判错是各网盘的方言：模块侧把鉴权头放进 Request.Headers，
// 响应的业务码判定由模块的调用方完成（core/errors 提供统一错误类型）。
package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultUA       = "pan-go/core"
	defaultTimeout  = 30 * time.Second
)

// Config 执行器配置。零值可用（默认 UA + 30s 超时 + 不重试）。
type Config struct {
	UserAgent string                           // 空=默认 UA
	Timeout   time.Duration                    // 单请求超时，0=30s
	Retries   int                              // 传输层错误重试次数（0=不重试；总尝试=Retries+1）
	Logf      func(format string, args ...any) // 可选请求日志（方法/URL/状态码/耗时）
	HTTPClient *http.Client                    // 自定义客户端（测试/代理用），nil=内置默认
}

// Body 请求体编码接口。现成实现：JSONBody / FormBody / RawBody。
// Retries>0 时 Encode 可能被多次调用，实现需保证每次返回可用的 Reader。
type Body interface {
	ContentType() string
	Encode() (io.Reader, error)
}

// JSONBody JSON 请求体。
type JSONBody struct{ V any }

// ContentType 实现 Body。
func (b JSONBody) ContentType() string { return "application/json;charset=UTF-8" }

// Encode 实现 Body。
func (b JSONBody) Encode() (io.Reader, error) {
	data, err := json.Marshal(b.V)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// FormBody 表单请求体。
type FormBody map[string]string

// ContentType 实现 Body。
func (b FormBody) ContentType() string { return "application/x-www-form-urlencoded" }

// Encode 实现 Body。
func (b FormBody) Encode() (io.Reader, error) {
	v := url.Values{}
	for k, val := range b {
		v.Set(k, val)
	}
	return strings.NewReader(v.Encode()), nil
}

// RawBody 自定义内容请求体（multipart 等由模块自行构建后包进来）。
type RawBody struct {
	MIME string
	R    io.Reader
}

// ContentType 实现 Body。
func (b RawBody) ContentType() string { return b.MIME }

// Encode 实现 Body。
func (b RawBody) Encode() (io.Reader, error) { return b.R, nil }

// Request 一次 HTTP 请求的描述。
type Request struct {
	Method  string            // 空=GET
	URL     string
	Query   url.Values        // 可空，拼到 URL 后
	Body    Body              // nil=无请求体
	Headers map[string]string // 额外头（鉴权 / Referer 等模块差异头）
}

// Response HTTP 响应。
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// Executor 执行器。配置不可变，可并发复用。
type Executor struct {
	hc      *http.Client
	ua      string
	retries int
	logf    func(format string, args ...any)
}

// New 创建执行器。
func New(cfg Config) *Executor {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	ua := cfg.UserAgent
	if ua == "" {
		ua = defaultUA
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: timeout}
	}
	return &Executor{
		hc:      hc,
		ua:      ua,
		retries: cfg.Retries,
		logf:    cfg.Logf,
	}
}

// Do 执行请求。仅传输层错误按 Retries 重试；HTTP 非 2xx 不算传输错误，
// 原样返回 Response 由调用方判定业务语义。
func (e *Executor) Do(ctx context.Context, r *Request) (*Response, error) {
	attempts := e.retries + 1
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if ctx.Err() != nil {
			return nil, lastErr
		}
		resp, err := e.doOnce(ctx, r)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// doOnce 单次执行。
func (e *Executor) doOnce(ctx context.Context, r *Request) (*Response, error) {
	method := r.Method
	if method == "" {
		method = http.MethodGet
	}
	u := r.URL
	if len(r.Query) > 0 {
		sep := "?"
		if strings.Contains(u, "?") {
			sep = "&"
		}
		u += sep + r.Query.Encode()
	}

	var body io.Reader
	var ct string
	if r.Body != nil {
		reader, err := r.Body.Encode()
		if err != nil {
			return nil, fmt.Errorf("encode body: %w", err)
		}
		body = reader
		ct = r.Body.ContentType()
	}

	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", e.ua)
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := e.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if e.logf != nil {
		e.logf("httpx %s %s -> %d %s", method, u, resp.StatusCode, time.Since(start).Round(time.Millisecond))
	}
	return &Response{StatusCode: resp.StatusCode, Header: resp.Header, Body: data}, nil
}
