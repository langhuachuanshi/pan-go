// Package baidu 是百度网盘开放平台的 Go SDK 主包。
//
// 基于 OAuth 2.0 鉴权（access_token），通过开放平台官方 REST API 操作网盘。
// 域名分三套：
//   - openapi.baidu.com：OAuth 授权/换 token（在 auth 包内）
//   - pan.baidu.com：业务接口（列表/上传预创建/创建/文件管理）
//   - d.pcs.baidu.com：分片上传（PCS）
//
// 典型用法：
//
//	c, _ := baidu.New(ctx, &baidu.Config{AppKey: "...", SecretKey: "..."})
//	// 首次授权：浏览器打开 c.AuthorizeURL() → 拿 code → c.ExchangeToken(ctx, code)
//	files, _ := c.Files().List(ctx, "/")
package baidu

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/langhuachuanshi/panbaidu-go/baidu/auth"
	"github.com/langhuachuanshi/panbaidu-go/baidu/download"
	"github.com/langhuachuanshi/panbaidu-go/baidu/file"
	"github.com/langhuachuanshi/panbaidu-go/baidu/invoker"
	"github.com/langhuachuanshi/panbaidu-go/baidu/management"
	"github.com/langhuachuanshi/panbaidu-go/baidu/upload"
)

// 业务接口域名。
const (
	apiBase = "https://pan.baidu.com/rest/2.0" // 业务接口
	pcsBase = "https://d.pcs.baidu.com/rest/2.0" // 分片上传（PCS）
)

// 通用 header。
const userAgent = "panbaidu-go/1.0"

// Config SDK 配置。
type Config struct {
	AppKey      string
	SecretKey   string
	RedirectURI string // 授权回调地址，默认 oob
	TokenFile   string // token 持久化文件，默认 ~/.panbaidu/token.json
}

// Client 百度网盘客户端。线程安全。
type Client struct {
	mgr  *auth.Manager
	http *http.Client
}

// New 创建 Client。AppKey/SecretKey 必填。
// 不要求已有 token——可先创建 Client，再走 ExchangeToken 完成首次授权。
func New(_ context.Context, cfg *Config) (*Client, error) {
	if cfg == nil || cfg.AppKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("baidu: AppKey 和 SecretKey 必填")
	}
	mgr := auth.New(&auth.Config{
		AppKey:      cfg.AppKey,
		SecretKey:   cfg.SecretKey,
		RedirectURI: cfg.RedirectURI,
		TokenFile:   cfg.TokenFile,
	})
	return &Client{
		mgr:  mgr,
		http: &http.Client{Timeout: 60 * 1e9}, // 60s
	}, nil
}

// —— 鉴权相关（委托给 auth.Manager）——

// AuthorizeURL 返回用户授权链接（浏览器打开，授权后拿 code）。
func (c *Client) AuthorizeURL() string { return c.mgr.AuthorizeURL() }

// ExchangeToken 用授权码换 token 并持久化（首次授权用）。
func (c *Client) ExchangeToken(ctx context.Context, code string) error {
	_, err := c.mgr.ExchangeToken(ctx, code)
	return err
}

// accessToken 取当前有效 token（自动刷新）。
func (c *Client) accessToken(ctx context.Context) (string, error) {
	t, err := c.mgr.GetToken(ctx)
	if err != nil {
		return "", err
	}
	return t.AccessToken, nil
}

// RawToken 返回当前 access_token（自动刷新）。供需要手动拼接 token 的场景（如下载直链）。
func (c *Client) RawToken(ctx context.Context) (string, error) {
	return c.accessToken(ctx)
}

// —— invoker.Invoker 实现 ——

// Get 发 GET 请求（业务接口，走 pan.baidu.com）。
// access_token 自动注入到 query。
func (c *Client) Get(ctx context.Context, path string, params map[string]string) ([]byte, int, error) {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return nil, 0, err
	}
	q := url.Values{}
	q.Set("access_token", tok)
	for k, v := range params {
		q.Set(k, v)
	}
	fullURL := apiBase + path + "?" + q.Encode()
	return c.do(ctx, http.MethodGet, fullURL, nil, "")
}

// PostForm 发 POST form-urlencoded 请求（业务接口）。
// access_token 在 query，业务参数在 body。
func (c *Client) PostForm(ctx context.Context, path string, body map[string]string, params map[string]string) ([]byte, int, error) {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return nil, 0, err
	}
	q := url.Values{}
	q.Set("access_token", tok)
	for k, v := range params {
		q.Set(k, v)
	}
	form := url.Values{}
	for k, v := range body {
		form.Set(k, v)
	}
	fullURL := apiBase + path + "?" + q.Encode()
	return c.do(ctx, http.MethodPost, fullURL, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
}

// PostMultipart 发 POST multipart 请求（仅分片上传用，走 PCS 域名）。
// 必须先 buffer 出 body 带 Content-Length（百度 PCS 不支持 chunked）。
func (c *Client) PostMultipart(ctx context.Context, baseURL, path string, params map[string]string, fieldName, fileName string, file []byte) ([]byte, int, error) {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return nil, 0, err
	}
	q := url.Values{}
	q.Set("access_token", tok)
	for k, v := range params {
		q.Set(k, v)
	}
	base := baseURL
	if base == "" {
		base = pcsBase
	}
	fullURL := base + path + "?" + q.Encode()

	// 先 buffer 整个 multipart body（带 Content-Length）。
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(fieldName, fileName)
	if err != nil {
		return nil, 0, err
	}
	if _, err := fw.Write(file); err != nil {
		return nil, 0, err
	}
	mw.Close()

	return c.do(ctx, http.MethodPost, fullURL, bytes.NewReader(buf.Bytes()), mw.FormDataContentType())
}

// do 执行请求。
func (c *Client) do(ctx context.Context, method, fullURL string, body io.Reader, contentType string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", userAgent)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return data, resp.StatusCode, err
}

// —— Service 访问方法 ——

// Files 返回文件 service。
func (c *Client) Files() *file.Service { return file.New(c) }

// Management 返回文件管理 service。
func (c *Client) Management() *management.Service { return management.New(c) }

// Upload 返回上传 service。
func (c *Client) Upload() *upload.Service { return upload.New(c) }

// Download 返回下载 service。
func (c *Client) Download() *download.Service { return download.New(c, c) }

// 编译期保证 Client 实现 Invoker。
var _ invoker.Invoker = (*Client)(nil)
