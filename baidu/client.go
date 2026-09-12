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
//	c, _ := baidu.New(ctx, &baidu.Config{BDUSS: "...", STOKEN: "..."})
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
	"time"

	"github.com/langhuachuanshi/pan-go/baidu/auth"
	"github.com/langhuachuanshi/pan-go/baidu/clouddl"
	"github.com/langhuachuanshi/pan-go/baidu/download"
	"github.com/langhuachuanshi/pan-go/baidu/file"
	"github.com/langhuachuanshi/pan-go/baidu/invoker"
	"github.com/langhuachuanshi/pan-go/baidu/management"
	"github.com/langhuachuanshi/pan-go/baidu/share"
	"github.com/langhuachuanshi/pan-go/baidu/upload"
	"github.com/langhuachuanshi/pan-go/baidu/user"
	coreerrors "github.com/langhuachuanshi/pan-go/core/errors"
	"github.com/langhuachuanshi/pan-go/core/httpx"
)

// 接口域名（实测：网页端接口全走 pan.baidu.com，pcs 仅用于分片上传域名）。
const (
	panBase = "https://pan.baidu.com"   // 网页端 API（/api/*）
	pcsBase = "https://d.pcs.baidu.com" // 分片上传域名（superfile2）
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
	http *http.Client    // 携带 BDUSS/STOKEN cookiejar
	exec *httpx.Executor // core 执行器（与 c.http 共享客户端）
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
	c := &Client{
		mgr:  auth.New(&auth.Config{BDUSS: cfg.BDUSS, STOKEN: cfg.STOKEN}),
		http: &http.Client{Timeout: 60 * time.Second, Jar: jar}, // 60s
	}
	c.exec = httpx.New(httpx.Config{UserAgent: userAgent, HTTPClient: c.http})
	return c, nil
}

// —— core/invoker 标准菜单实现 ——

// Get 发 GET，成功时把 JSON 响应解到 out（nil 忽略）。
// path 相对 panBase；通用 query + cookie 自动注入，无需 bdstoken。
func (c *Client) Get(ctx context.Context, path string, query map[string]string, out any) error {
	q := buildQuery(query, "")
	resp, err := c.exec.Do(ctx, &httpx.Request{
		URL:     panBase + path + "?" + q.Encode(),
		Headers: c.baseHeaders(),
	})
	if err != nil {
		return err
	}
	return c.decode(resp, out)
}

// Post 发 POST：body 为 map[string]string 时走表单（等价 PostForm），否则 JSON。
func (c *Client) Post(ctx context.Context, path string, body any, query map[string]string, out any) error {
	if form, ok := body.(map[string]string); ok {
		return c.PostFormQuery(ctx, path, form, query, out)
	}
	q := buildQuery(query, "")
	resp, err := c.exec.Do(ctx, &httpx.Request{
		Method:  http.MethodPost,
		URL:     panBase + path + "?" + q.Encode(),
		Body:    httpx.JSONBody{V: body},
		Headers: c.baseHeaders(),
	})
	if err != nil {
		return err
	}
	return c.decode(resp, out)
}

// PostForm 发 POST form（写操作：bdstoken 自动注入），成功时解 JSON 到 out。
func (c *Client) PostForm(ctx context.Context, path string, form map[string]string, out any) error {
	return c.PostFormQuery(ctx, path, form, nil, out)
}

// PostFormQuery POST form 且 body 与 query 分离（baidu 方言：/api/filemanager 的 opera 在 query）。
func (c *Client) PostFormQuery(ctx context.Context, path string, body, params map[string]string, out any) error {
	bdstoken, err := c.mgr.BDstoken(ctx)
	if err != nil {
		return fmt.Errorf("获取 bdstoken 失败: %w", err)
	}
	q := buildQuery(params, bdstoken)
	form := url.Values{}
	for k, v := range body {
		form.Set(k, v)
	}
	resp, err := c.exec.Do(ctx, &httpx.Request{
		Method:  http.MethodPost,
		URL:     panBase + path + "?" + q.Encode(),
		Body:    httpx.FormBody(body),
		Headers: c.baseHeaders(),
	})
	_ = form
	if err != nil {
		return err
	}
	return c.decode(resp, out)
}

// Multipart 发 multipart 文件上传（contract 完整性实现；分片上传走方言 PostMultipart）。
func (c *Client) Multipart(ctx context.Context, path string, form map[string]string, field, filename string, file io.Reader, out any) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range form {
		if err := mw.WriteField(k, v); err != nil {
			return err
		}
	}
	fw, err := mw.CreateFormFile(field, filename)
	if err != nil {
		return err
	}
	if _, err := io.Copy(fw, file); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return err
	}
	resp, err := c.exec.Do(ctx, &httpx.Request{
		Method:  http.MethodPost,
		URL:     panBase + path,
		Body:    httpx.RawBody{MIME: mw.FormDataContentType(), R: bytes.NewReader(buf.Bytes())},
		Headers: c.baseHeaders(),
	})
	if err != nil {
		return err
	}
	return c.decode(resp, out)
}

// DownloadHeaders 下载直链所需头（cookie 由 jar 携带，这里给 UA/Referer）。
func (c *Client) DownloadHeaders() map[string]string {
	return map[string]string{
		"User-Agent": userAgent,
		"Referer":    referer,
	}
}

// baseHeaders 请求头（UA 由 exec 注入；cookie 由 jar 携带）。
func (c *Client) baseHeaders() map[string]string {
	return map[string]string{"Referer": referer}
}

// decode HTTP 状态判错 + JSON 解到 out（nil 忽略）。
func (c *Client) decode(resp *httpx.Response, out any) error {
	if resp.StatusCode >= 400 {
		return coreerrors.New("baidu", 0, resp.StatusCode, truncateBody(resp.Body), coreerrors.KindOther)
	}
	return invoker.Decode(resp.Body, out)
}

func truncateBody(b []byte) string {
	if len(b) > 200 {
		return string(b[:200]) + "..."
	}
	return string(b)
}

// PostMultipart 分片上传（baidu 方言：superfile2 走 pcs 域名，params 进 query）。
// baseURL 指定域名：分片上传传 pcsUploadBase，空=panBase。先 buffer 出 body 带 Content-Length（百度不支持 chunked）。
func (c *Client) PostMultipart(ctx context.Context, baseURL, path string, params map[string]string, fieldName, fileName string, data []byte) ([]byte, int, error) {
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
	if _, err := fw.Write(data); err != nil {
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

// do 执行请求（方言 Raw 系列共用；执行走 core/httpx，UA/Referer 注入）。
func (c *Client) do(ctx context.Context, method, fullURL string, body io.Reader, contentType string) ([]byte, int, error) {
	var b httpx.Body
	if body != nil {
		b = httpx.RawBody{MIME: contentType, R: body}
	}
	resp, err := c.exec.Do(ctx, &httpx.Request{
		Method:  method,
		URL:     fullURL,
		Body:    b,
		Headers: map[string]string{"Referer": referer},
	})
	if err != nil {
		return nil, 0, err
	}
	return resp.Body, resp.StatusCode, nil
}

// —— 原始请求方法（不注入通用 query / bdstoken） ——

// HTTPClient 返回内部 HTTP 客户端（携带 BDUSS cookie）。
// 用于下载、PanHome 签名等需要直接 HTTP 操作的场景。
func (c *Client) HTTPClient() *http.Client { return c.http }

// GetRaw 发 GET 到完整 URL，不注入通用 query。用于 share/record 等特殊接口。
func (c *Client) GetRaw(ctx context.Context, fullURL string) ([]byte, int, error) {
	return c.do(ctx, http.MethodGet, fullURL, nil, "")
}

// PostFormRaw 发 POST form 到完整 URL，不注入通用 query。用于 share、PCS 等特殊接口。
func (c *Client) PostFormRaw(ctx context.Context, fullURL string, body map[string]string) ([]byte, int, error) {
	form := url.Values{}
	for k, v := range body {
		form.Set(k, v)
	}
	return c.do(ctx, http.MethodPost, fullURL, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
}

// PostMultipartForm 发 POST multipart/form-data（字段模式）。
func (c *Client) PostMultipartForm(ctx context.Context, fullURL string, fields map[string]string) ([]byte, int, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			return nil, 0, err
		}
	}
	mw.Close()
	return c.do(ctx, http.MethodPost, fullURL, bytes.NewReader(buf.Bytes()), mw.FormDataContentType())
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

// Share 返回分享 service。
func (c *Client) Share() *share.Service { return share.New(c) }

// User 返回用户信息 service。
func (c *Client) User() *user.Service { return user.New(c) }

// CloudDL 返回离线下载 service。
func (c *Client) CloudDL() *clouddl.Service { return clouddl.New(c) }

// 编译期保证 Client 实现 Invoker。
var _ invoker.Invoker = (*Client)(nil)
