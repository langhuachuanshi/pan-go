// Package quark 是夸克网盘的 Go SDK 主包。
//
// 夸克网盘基于 cookie 鉴权（无 token、无签名）。
// cookie 来源二选一：扫码登录（quark/qrcode，Login.Wait 直接返回完整 cookie），
// 或从浏览器（pan.quark.cn 登录后 F12 → Network → 任意请求 → Cookie）复制。
// 最关键字段 __puus/__pus（扫码换发的 cookie 只有 __pus，实测 drive 接口认）。
//
// HTTP 执行走 pan-go/core（httpx/invoker），本包实现 core 标准菜单供业务子包使用。
//
// 典型用法：
//
//	c, err := quark.New(ctx, quark.WithCookie("浏览器复制的cookie"))
//	files, _ := c.Files().List(ctx, &file.ListRequest{PDirFID: "0"})
package quark

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	coreerrors "github.com/langhuachuanshi/pan-go/core/errors"
	"github.com/langhuachuanshi/pan-go/core/httpx"
	"github.com/langhuachuanshi/pan-go/quark/auth"
	"github.com/langhuachuanshi/pan-go/quark/download"
	"github.com/langhuachuanshi/pan-go/quark/file"
	"github.com/langhuachuanshi/pan-go/quark/invoker"
	"github.com/langhuachuanshi/pan-go/quark/share"
	"github.com/langhuachuanshi/pan-go/quark/upload"
)

// 夸克 API base 与固定参数。
const (
	apiBase = "https://drive-pc.quark.cn/1/clouddrive"
	// 夸克 PC 端请求的公共 query 参数。
	// 注意：pr=ucpro（不是 uqm，uqm 会被当作游客返回 31001）。
	commonPr = "ucpro"
	commonFr = "pc"
)

// 通用 header（伪装 PC 客户端）。
const (
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) quark-cloud-drive/2.5.20 Chrome/100.0.4896.160 Electron/18.3.2.4 Safari/537.36 Channel/pckk_other_ch"
	referer   = "https://pan.quark.cn/"
	origin    = "https://pan.quark.cn"
)

// Client 夸克网盘客户端。配置不可变，可并发复用。
type Client struct {
	exec   *httpx.Executor
	cookie string
}

// New 创建 Client。必须提供 cookie（通过 WithCookie 或 WithCookieFile）。
func New(ctx context.Context, opts ...Option) (*Client, error) {
	o := defaultOptions()
	for _, fn := range opts {
		fn(o)
	}

	cfg := &auth.Config{Cookie: o.cookie, CookieFile: o.cookieFile}
	result, err := auth.Load(cfg)
	if err != nil {
		return nil, err
	}
	if !auth.IsValid(result.Cookie) {
		return nil, fmt.Errorf("quark: cookie 无效（须含 __puus 或 __pus），请扫码登录或从浏览器复制完整 cookie")
	}

	return &Client{
		exec:   httpx.New(httpx.Config{UserAgent: userAgent, Timeout: 60 * time.Second}),
		cookie: result.Cookie,
	}, nil
}

// do 核心执行：拼 URL（base + path + 公共参数）→ 注鉴权头 → httpx 执行 → HTTP 状态判错。
func (c *Client) do(ctx context.Context, method, path string, body any, query map[string]string) (*httpx.Response, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	q := url.Values{"pr": {commonPr}, "fr": {commonFr}, "uc_param_str": {""}}
	for k, v := range query {
		q.Set(k, v)
	}
	var b httpx.Body
	if body != nil {
		b = httpx.JSONBody{V: body}
	}
	resp, err := c.exec.Do(ctx, &httpx.Request{
		Method: method,
		URL:    apiBase + path,
		Query:  q,
		Body:   b,
		Headers: map[string]string{
			"Cookie":  c.cookie,
			"Referer": referer,
			"Origin":  origin,
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, coreerrors.New("quark", 0, resp.StatusCode, truncateBody(resp.Body), coreerrors.KindOther)
	}
	return resp, nil
}

// decode JSON 解到 out（nil 忽略）。
func (c *Client) decode(body []byte, out any) error {
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return coreerrors.New("quark", 0, 200, "invalid json response: "+truncateBody(body), coreerrors.KindOther)
	}
	return nil
}

func truncateBody(b []byte) string {
	if len(b) > 200 {
		return string(b[:200]) + "..."
	}
	return string(b)
}

// ===== core/invoker 标准菜单实现 =====

// Get 发 GET，成功时把 JSON 响应解到 out（nil 忽略）。
func (c *Client) Get(ctx context.Context, path string, query map[string]string, out any) error {
	resp, err := c.do(ctx, http.MethodGet, path, nil, query)
	if err != nil {
		return err
	}
	return c.decode(resp.Body, out)
}

// Post 发 POST JSON，成功时把 JSON 响应解到 out（nil 忽略）。
func (c *Client) Post(ctx context.Context, path string, body any, query map[string]string, out any) error {
	resp, err := c.do(ctx, http.MethodPost, path, body, query)
	if err != nil {
		return err
	}
	return c.decode(resp.Body, out)
}

// PostForm 发 POST 表单，成功时把 JSON 响应解到 out（nil 忽略）。
func (c *Client) PostForm(ctx context.Context, path string, form map[string]string, out any) error {
	resp, err := c.do(ctx, http.MethodPost, path, httpx.FormBody(form), nil)
	if err != nil {
		return err
	}
	return c.decode(resp.Body, out)
}

// Multipart 发 multipart 文件上传，成功时把 JSON 响应解到 out（nil 忽略）。
func (c *Client) Multipart(ctx context.Context, path string, form map[string]string, field, filename string, file io.Reader, out any) error {
	body := &strings.Builder{}
	mw := multipart.NewWriter(body)
	for k, v := range form {
		if err := mw.WriteField(k, v); err != nil {
			return err
		}
	}
	part, err := mw.CreateFormFile(field, filename)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return err
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	resp, err := c.exec.Do(ctx, &httpx.Request{
		Method: http.MethodPost,
		URL:    apiBase + path,
		Body:   httpx.RawBody{MIME: mw.FormDataContentType(), R: strings.NewReader(body.String())},
		Headers: map[string]string{
			"Cookie":  c.cookie,
			"Referer": referer,
			"Origin":  origin,
		},
	})
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return coreerrors.New("quark", 0, resp.StatusCode, truncateBody(resp.Body), coreerrors.KindOther)
	}
	return c.decode(resp.Body, out)
}

// DownloadHeaders 返回直链下载所需的鉴权头（Cookie/Referer/UA）。
// 夸克 /file/download 返回的临时直链 GET 时需带登录态，否则 403 RequestDeniedByCallback。
func (c *Client) DownloadHeaders() map[string]string {
	return map[string]string{
		"Cookie":     c.cookie,
		"Referer":    referer,
		"User-Agent": userAgent,
	}
}

// ===== Service 访问器 =====

// Files 返回文件 service。
func (c *Client) Files() *file.Service { return file.New(c) }

// Share 返回分享 service。
func (c *Client) Share() *share.Service { return share.New(c) }

// Upload 返回上传 service。
func (c *Client) Upload() *upload.Service { return upload.New(c) }

// Download 返回下载 service。
func (c *Client) Download() *download.Service { return download.New(c) }

// Cookie 返回当前 cookie（只读）。
func (c *Client) Cookie() string { return c.cookie }

// 编译期保证 Client 实现 core 标准菜单。
var _ invoker.Invoker = (*Client)(nil)
