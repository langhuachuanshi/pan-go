// Package alipan 是阿里云盘（alipan / aliyundrive）的 Go SDK 主包。
//
// 对应 Python 库 aligo (https://github.com/foyoux/aligo) 的核心子集 + 分享功能。
// 主包提供 Client 入口，各业务功能在子包中：
//
//   - alipan/types   数据模型
//   - alipan/auth    登录认证
//   - alipan/file    文件操作（列表/搜索/复制/移动/上传/下载等）
//   - alipan/share   分享（创建/管理/保存他人分享）
//   - alipan/drive   网盘
//   - alipan/user    用户
//
// Client 实现 invoker.Invoker 接口，各子包通过该接口访问 HTTP 能力，避免循环依赖。
// 线程安全，可被多 goroutine 共享。
package alipan

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/langhuachuanshi/alipan-go/alipan/auth"
	"github.com/langhuachuanshi/alipan-go/alipan/drive"
	"github.com/langhuachuanshi/alipan-go/alipan/file"
	"github.com/langhuachuanshi/alipan-go/alipan/invoker"
	"github.com/langhuachuanshi/alipan-go/alipan/share"
	"github.com/langhuachuanshi/alipan-go/alipan/types"
	"github.com/langhuachuanshi/alipan-go/alipan/user"
)

// host 与固定 header 常量（与 auth 子包保持一致，此处供 client 注入 header 用）。
const (
	hostAPI    = "https://api.aliyundrive.com"
	userAgent  = auth.UserAgent
	xCanary    = auth.XCanary
	xSignature = auth.XSignature
)

// Client 是阿里云盘客户端，所有 API 的入口。对应 aligo 的 Aligo 类。
type Client struct {
	mu       sync.RWMutex
	token    *types.Token
	http     *http.Client
	deviceID string
	config   clientConfig

	// 缓存的资源盘 ID（快传分享需要）。
	resourceDriveID string
	resDriveMu      sync.Mutex // 保护 resourceDriveID 缓存（独立于 mu，避免死锁）

	// 用于 401 刷新时避免并发重复刷新。
	refreshMu sync.Mutex
}

// clientConfig 保存 New 时的配置。
type clientConfig struct {
	name              string
	configDir         string
	requestIntv       time.Duration
	retryMax          int
	defaultDriveID    string // 显式指定的默认网盘 ID（option 设置）
}

// New 创建并初始化 Client。登录行为由 option 决定（见 option.go）。
//
// 默认行为：尝试读取 ~/.aligo/aligo.json 复用登录；不存在则终端二维码扫码登录。
func New(ctx context.Context, opts ...Option) (*Client, error) {
	o := defaultOptions()
	for _, fn := range opts {
		fn(o)
	}

	c := &Client{
		http: &http.Client{Timeout: 60 * time.Second},
		config: clientConfig{
			name:           o.name,
			configDir:      o.configDir,
			requestIntv:    o.requestInterval,
			retryMax:       defaultIf(o.retryMax, 5),
			defaultDriveID: o.defaultDriveID,
		},
	}

	authCfg := &auth.Config{
		Name:         o.name,
		ConfigDir:    o.configDir,
		RefreshToken: o.refreshToken,
		Token:        o.token,
		DefaultDrive: o.defaultDriveID,
		LoginMethod:  o.loginMethod,
		WebPort:      o.webPort,
		LoginTimeout: o.loginTimeout,
		ShowQR:       o.showQR,
	}
	result, err := auth.Login(ctx, authCfg)
	if err != nil {
		return nil, err
	}
	c.token = result.Token
	c.deviceID = result.DeviceID
	return c, nil
}

// —— invoker.Invoker 实现 ——

// Post 发送带鉴权的 POST JSON。
func (c *Client) Post(ctx context.Context, path string, body any, extraHeaders map[string]string, okStatus []int) ([]byte, int, error) {
	return c.requestWithRetry(ctx, http.MethodPost, hostAPI+path, body, true, extraHeaders, okStatus)
}

// PostAnon 发送匿名 POST（不带 access_token），用于访问他人分享。
func (c *Client) PostAnon(ctx context.Context, path string, body any, extraHeaders map[string]string, okStatus []int) ([]byte, int, error) {
	return c.requestWithRetry(ctx, http.MethodPost, hostAPI+path, body, false, extraHeaders, okStatus)
}

// PostRaw 发送 POST 到完整 URL。
func (c *Client) PostRaw(ctx context.Context, fullURL string, body any, withAuth bool, extraHeaders map[string]string, okStatus []int) ([]byte, int, error) {
	return c.requestWithRetry(ctx, http.MethodPost, fullURL, body, withAuth, extraHeaders, okStatus)
}

// DefaultDriveID 返回默认网盘 ID（优先 option 显式指定，其次 token 中的值）。
func (c *Client) DefaultDriveID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.config.defaultDriveID != "" {
		return c.config.defaultDriveID
	}
	return c.token.GetDefaultDriveID()
}

// AccessToken 返回当前 access_token。
func (c *Client) AccessToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token.GetAccessToken()
}

// UserID 返回当前用户 ID（分享列表接口的 creator 字段必需）。
func (c *Client) UserID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token.GetUserIDStr()
}

// ResourceDriveID 返回资源盘 ID（快传分享需要资源盘，备份盘不允许分享）。
// 通过 list_my_drives 查找 category=resource 的 drive，结果缓存。
// 若账号无资源盘，返回空串。
func (c *Client) ResourceDriveID() string {
	c.resDriveMu.Lock()
	defer c.resDriveMu.Unlock()
	if c.resourceDriveID != "" {
		return c.resourceDriveID
	}
	// 查 list_my_drives 找资源盘（注意：不能持 token 的 mu 锁，否则与 Post→AccessToken 死锁）。
	var resp struct {
		Items []struct {
			DriveID  string `json:"drive_id"`
			Category string `json:"category"`
			Name     string `json:"drive_name"`
		} `json:"items"`
	}
	if err := invoker.PostAndDecode(context.Background(), c, "/v2/drive/list_my_drives", map[string]any{}, &resp, []int{200}); err == nil {
		for _, it := range resp.Items {
			if it.Category == "resource" || it.Name == "resource" {
				c.resourceDriveID = it.DriveID
				return it.DriveID
			}
		}
	}
	return ""
}

// requestWithRetry 核心请求逻辑：注入 header、发送、401 自动刷新重试、退避。
func (c *Client) requestWithRetry(ctx context.Context, method, fullURL string, body any, withAuth bool, extraHeaders map[string]string, okStatus []int) ([]byte, int, error) {
	if len(okStatus) == 0 {
		okStatus = []int{200}
	}
	retryMax := c.config.retryMax
	var lastErr error
	for attempt := 1; attempt <= retryMax; attempt++ {
		if c.config.requestIntv > 0 {
			if err := sleepCtx(ctx, c.config.requestIntv); err != nil {
				return nil, 0, err
			}
		}
		data, status, err := c.doOnce(ctx, method, fullURL, body, withAuth, extraHeaders)
		if err == nil {
			if containsInt(okStatus, status) {
				return data, status, nil
			}
			apiErr := invoker.ParseAPIError(status, data)
			handled := c.handleNonOK(ctx, apiErr, attempt)
			if handled == nil {
				lastErr = apiErr
				continue
			}
			return nil, status, handled
		}
		lastErr = err
		_ = sleepCtx(ctx, 3*time.Second)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("alipan: request failed after %d retries", retryMax)
	}
	return nil, 0, lastErr
}

func (c *Client) doOnce(ctx context.Context, method, fullURL string, body any, withAuth bool, extraHeaders map[string]string) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		b, err := marshalBody(body)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Referer", "https://aliyundrive.com")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("x-canary", xCanary)
	if withAuth {
		if at := c.AccessToken(); at != "" {
			req.Header.Set("Authorization", at)
		}
		if dev := c.deviceID; dev != "" {
			req.Header.Set("x-device-id", dev)
			req.Header.Set("x-signature", xSignature)
		}
	}
	if method == http.MethodPost && reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range extraHeaders {
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

// handleNonOK 处理非成功状态码。返回 nil 表示已修复可重试。
func (c *Client) handleNonOK(ctx context.Context, e *invoker.APIError, attempt int) error {
	switch e.StatusCode {
	case 401:
		if e.Code == "ShareLinkTokenInvalid" || e.Code == "UserDeviceOffline" {
			return e
		}
		if c.tryRefreshToken(ctx) {
			return nil
		}
		return e
	case 429, 502, 504:
		_ = sleepCtx(ctx, time.Duration(backoffSec(attempt))*time.Second)
		return nil
	case 500:
		return e
	default:
		return e
	}
}

func (c *Client) tryRefreshToken(ctx context.Context) bool {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	rt := c.token.GetRefreshToken()
	if rt == "" {
		return false
	}
	return auth.RefreshToken(ctx, c.token, rt, c.config.name, c.config.configDir, false) == nil
}

// —— Service 访问方法 ——

// Files 返回文件 service。
func (c *Client) Files() *file.Service { return file.New(c) }

// Share 返回分享 service。
func (c *Client) Share() *share.Service { return share.New(c) }

// Drives 返回网盘 service。
func (c *Client) Drives() *drive.Service { return drive.New(c) }

// Users 返回用户 service。
func (c *Client) Users() *user.Service { return user.New(c) }

// Token 返回当前账号凭证的拷贝（只读）。
func (c *Client) Token() *types.Token {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.token == nil {
		return nil
	}
	cp := *c.token
	return &cp
}

// Logout 删除本地配置文件（对应 aligo logout）。
func (c *Client) Logout() error {
	return auth.DeleteTokenFile(c.config.name, c.config.configDir)
}

// —— helpers ——

func marshalBody(body any) ([]byte, error) {
	switch v := body.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	case nil:
		return nil, nil
	default:
		return json.Marshal(v)
	}
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func backoffSec(attempt int) int {
	pow := 1
	for i := 0; i < attempt%4; i++ {
		pow *= 5
	}
	return pow
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

func defaultIf(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

// 编译期保证 Client 实现 Invoker 接口。
var _ invoker.Invoker = (*Client)(nil)
