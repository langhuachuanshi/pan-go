// Package baidu 是百度网盘网页端（抓包）的 Go SDK 主包。
//
// 基于 BDUSS + STOKEN cookie 鉴权（网页登录态），走网页端 REST API 操作网盘。
// 与 openapi 分支（OAuth + /xpan/*）是两套完全不同的鉴权模型，不要混用。
//
// 所有接口走 pan.baidu.com（实测确认，旧的 pcs.baidu.com 路线已失效，报 31030 pcs token not exist）：
//   - /api/list：文件列表（GET，仅需 BDUSS）
//   - /api/create：建目录 / 创建文件（POST，需 STOKEN）
//   - /api/precreate：预上传（POST，需 STOKEN）
//   - /api/filemanager?opera={move|rename|delete}：文件管理（POST，需 STOKEN）
//   - /pcs/superfile2：分片上传（POST multipart，走 pcs 域名）
//
// 通用约定（实测确认）：query 带 channel=chunlei&web=1&app_id=250528&clienttype=0，
// 写操作额外带 bdstoken，header 带 Referer: https://pan.baidu.com/disk/main，cookie=BDUSS+STOKEN。
//
// 典型用法：
//
//	c, _ := baidu.New(&baidu.Config{BDUSS: "...", STOKEN: "..."})
//	files, _ := c.Files().List(ctx, &file.ListRequest{Dir: "/"})
package baidu

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/langhuachuanshi/baidupan-go/baidu/auth"
	"github.com/langhuachuanshi/baidupan-go/baidu/download"
	"github.com/langhuachuanshi/baidupan-go/baidu/file"
	"github.com/langhuachuanshi/baidupan-go/baidu/invoker"
	"github.com/langhuachuanshi/baidupan-go/baidu/management"
	"github.com/langhuachuanshi/baidupan-go/baidu/upload"
)

// 接口域名（实测：网页端接口全走 pan.baidu.com，pcs 仅用于分片上传域名）。
const (
	panBase = "https://pan.baidu.com"          // 网页端 API（/api/*）
	pcsBase = "https://d.pcs.baidu.com"        // 分片上传域名（superfile2）
)

// 通用 header。
const (
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"
	referer   = "https://pan.baidu.com/disk/main"
)

// Config SDK 配置。
type Config struct {
	BDUSS  string // 网页端登录 cookie，必填
	STOKEN string // 写操作必需（list 可空），必填
}

// Client 百度网盘客户端。线程安全。
type Client struct {
	mgr  *auth.Manager
	http *http.Client
}

// New 创建 Client。BDUSS 必填，STOKEN 建议填（写操作需要）。
func New(ctx context.Context, cfg *Config) (*Client, error) {
	if cfg == nil || cfg.BDUSS == "" {
		return nil, fmt.Errorf("baidu: BDUSS 必填")
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	// BDUSS + STOKEN 塞进 cookiejar，domain=.baidu.com，后续所有请求自动携带。
	panURL := &url.URL{Scheme: "https", Host: "pan.baidu.com"}
	cookies := []*http.Cookie{
		{Name: "BDUSS", Value: cfg.BDUSS, Domain: ".baidu.com"},
	}
	if cfg.STOKEN != "" {
		cookies = append(cookies, &http.Cookie{Name: "STOKEN", Value: cfg.STOKEN, Domain: ".baidu.com"})
	}
	jar.SetCookies(panURL, cookies)
	return &Client{
		mgr:  auth.New(&auth.Config{BDUSS: cfg.BDUSS, STOKEN: cfg.STOKEN}),
		http: &http.Client{Timeout: 60 * 1e9, Jar: jar}, // 60s
	}, nil
}

// —— invoker.Invoker 实现 ——

// Get 发 GET 请求（list 用）。注入通用 query，BDUSS cookie 自动带，无需 bdstoken。
// path 是相对 panBase 的路径（如 /api/list）。
func (c *Client) Get(ctx context.Context, path string, params map[string]string) ([]byte, int, error) {
	q := buildQuery(params, "")
	fullURL := panBase + path + "?" + q.Encode()
	return c.do(ctx, http.MethodGet, fullURL, nil, "")
}

// PostForm 发 POST form 请求（写操作用）。注入通用 query + bdstoken，cookie 自动带。
// path 相对 panBase。bdstoken 自动获取并注入（需配置 STOKEN）。
func (c *Client) PostForm(ctx context.Context, path string, body map[string]string, params map[string]string) ([]byte, int, error) {
	bdstoken, err := c.mgr.BDstoken(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("获取 bdstoken 失败: %w", err)
	}
	q := buildQuery(params, bdstoken)
	fullURL := panBase + path + "?" + q.Encode()
	form := url.Values{}
	for k, v := range body {
		form.Set(k, v)
	}
	return c.do(ctx, http.MethodPost, fullURL, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
}

// PostMultipart 发 POST multipart 请求（仅分片上传用）。
// baseURL 指定域名：分片上传传 pcsBase，其他传 ""（默认 panBase）。
// path 是相对路径。先 buffer 出 body 带 Content-Length（百度不支持 chunked）。
func (c *Client) PostMultipart(ctx context.Context, baseURL, path string, params map[string]string, fieldName, fileName string, file []byte) ([]byte, int, error) {
	bdstoken, err := c.mgr.BDstoken(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("获取 bdstoken 失败: %w", err)
	}
	q := buildQuery(params, bdstoken)
	base := baseURL
	if base == "" {
		base = panBase
	}
	fullURL := base + path + "?" + q.Encode()

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

// buildQuery 合并通用 query 参数（channel/web/app_id/clienttype）+ 业务参数 + 可选 bdstoken。
func buildQuery(params map[string]string, bdstoken string) url.Values {
	q := url.Values{}
	for k, v := range auth.CommonQuery() {
		q.Set(k, v)
	}
	for k, v := range params {
		q.Set(k, v)
	}
	if bdstoken != "" {
		q.Set("bdstoken", bdstoken)
	}
	return q
}

// do 执行请求。注入 Referer 和 UA。
func (c *Client) do(ctx context.Context, method, fullURL string, body io.Reader, contentType string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", referer)
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
func (c *Client) Download() *download.Service { return download.New(c) }

// 编译期保证 Client 实现 Invoker。
var _ invoker.Invoker = (*Client)(nil)
