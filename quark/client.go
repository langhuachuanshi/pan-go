// Package quark 是夸克网盘的 Go SDK 主包。
//
// 夸克网盘基于 cookie 鉴权（无 token、无签名）。
// cookie 来源二选一：扫码登录（quark/qrcode，Login.Wait 直接返回完整 cookie），
// 或从浏览器（pan.quark.cn 登录后 F12 → Network → 任意请求 → Cookie）复制。
// 最关键字段 __puus/__pus（扫码换发的 cookie 只有 __pus，实测 drive 接口认）。
//
// 典型用法：
//
//	c, err := quark.New(ctx, quark.WithCookie("浏览器复制的cookie"))
//	files, _ := c.Files().List(ctx, &file.ListRequest{PDirFid: "0"})
package quark

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/langhuachuanshi/quark-go/quark/auth"
	"github.com/langhuachuanshi/quark-go/quark/download"
	"github.com/langhuachuanshi/quark-go/quark/file"
	"github.com/langhuachuanshi/quark-go/quark/invoker"
	"github.com/langhuachuanshi/quark-go/quark/share"
	"github.com/langhuachuanshi/quark-go/quark/types"
	"github.com/langhuachuanshi/quark-go/quark/upload"
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

// Client 夸克网盘客户端。线程安全。
type Client struct {
	mu      sync.RWMutex
	http    *http.Client
	cookie  string
	cookieMu sync.Mutex
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
		return nil, fmt.Errorf("quark: cookie 无效（缺少 __puus），请从浏览器复制完整 cookie")
	}

	c := &Client{
		http: &http.Client{
			Timeout: 60e9, // 60s
		},
		cookie: result.Cookie,
	}
	return c, nil
}

// —— invoker.Invoker 实现 ——

// Get 发 GET 请求。
func (c *Client) Get(ctx context.Context, path string, params map[string]string, headers map[string]string) ([]byte, int, error) {
	return c.request(ctx, http.MethodGet, path, nil, params, headers)
}

// Post 发 POST JSON 请求。
func (c *Client) Post(ctx context.Context, path string, body any, params map[string]string, headers map[string]string) ([]byte, int, error) {
	return c.request(ctx, http.MethodPost, path, body, params, headers)
}

// request 核心请求：拼 URL + 公共参数 + header 注入 + 发送。
func (c *Client) request(ctx context.Context, method, path string, body any, params map[string]string, headers map[string]string) ([]byte, int, error) {
	// 拼 URL：base + path + 公共参数 + 业务参数。
	u := apiBase
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u += path
	q := url.Values{}
	q.Set("pr", commonPr)
	q.Set("fr", commonFr)
	q.Set("uc_param_str", "")
	for k, v := range params {
		q.Set(k, v)
	}
	fullURL := u + "?" + q.Encode()

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reader = strings.NewReader(string(b))
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reader)
	if err != nil {
		return nil, 0, err
	}
	// 注入通用 header。
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", referer)
	req.Header.Set("Origin", origin)
	// cookie 直接注入 header（不用 cookiejar，避免跨域问题）。
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	}
	// 应用额外 header。
	for k, v := range headers {
		req.Header.Set(k, v)
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

// Share 返回分享 service。
func (c *Client) Share() *share.Service { return share.New(c) }

// Upload 返回上传 service。
func (c *Client) Upload() *upload.Service { return upload.New(c) }

// Download 返回下载 service。
func (c *Client) Download() *download.Service { return download.New(c) }

// Cookie 返回当前 cookie（只读）。
func (c *Client) Cookie() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cookie
}

// DownloadHeaders 返回下载直链所需的鉴权头（Cookie/Referer/User-Agent）。
// 夸克 /file/download 返回的临时直链 GET 时需带登录态，否则 403。
func (c *Client) DownloadHeaders() map[string]string {
	return map[string]string{
		"Cookie":     c.Cookie(),
		"Referer":    referer,
		"User-Agent": userAgent,
	}
}

// 编译期保证 Client 实现 Invoker。
var _ invoker.Invoker = (*Client)(nil)

// 占位：types 引用（部分 service 会用到）。
var _ = types.File{}
