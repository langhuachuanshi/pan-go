package lanzou

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

	"github.com/langhuachuanshi/pan-go/core/httpx"
	"github.com/langhuachuanshi/pan-go/lanzou/account"
	"github.com/langhuachuanshi/pan-go/lanzou/download"
	"github.com/langhuachuanshi/pan-go/lanzou/file"
	"github.com/langhuachuanshi/pan-go/lanzou/folder"
	"github.com/langhuachuanshi/pan-go/lanzou/invoker"
	"github.com/langhuachuanshi/pan-go/lanzou/recycle"
	"github.com/langhuachuanshi/pan-go/lanzou/resolve"
	"github.com/langhuachuanshi/pan-go/lanzou/upload"
)

// Client 蓝奏云客户端（实现 core/invoker + lanzou 方言扩展）。
// 业务能力经 Account / Files / Folders / Upload / Download / Recycle / Resolve 访问器提供。
type Client struct {
	exec        *httpx.Executor
	hc          *http.Client // 自定义或内置；exec 与之共享
	timeout     int
	cookies     []*http.Cookie
	logged      bool
	maxsize     int
	maxDLCount  int
	uploadDelay [2]int
	challenge   *ChallengeConfig
	uid         string
	vei         string
}

// Option 函数式配置选项
type Option func(*Client)

// WithTimeout 设置 HTTP 超时时间
func WithTimeout(seconds int) Option {
	return func(c *Client) {
		c.timeout = seconds
		c.hc.Timeout = time.Duration(seconds) * time.Second
	}
}

// WithMaxSize 设置最大文件大小限制(字节)
func WithMaxSize(size int) Option {
	return func(c *Client) { c.maxsize = size }
}

// WithHTTPClient 自定义 http.Client
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.hc = hc
		c.rebuildExec()
	}
}

// WithMaxDownloadCount 设置最大下载并发数
func WithMaxDownloadCount(n int) Option {
	return func(c *Client) {
		if n > 0 {
			c.maxDLCount = n
		}
	}
}

// WithUploadDelay 设置上传延迟范围(毫秒)
func WithUploadDelay(min, max int) Option {
	return func(c *Client) { c.uploadDelay = [2]int{min, max} }
}

// WithChallengeConfig 自定义 acw_sc__v2 挑战参数（蓝奏云换混淆时更新）
func WithChallengeConfig(cfg *ChallengeConfig) Option {
	return func(c *Client) {
		if cfg != nil {
			c.challenge = cfg
		}
	}
}

// NewClient 创建蓝奏云客户端
func NewClient(opts ...Option) *Client {
	c := &Client{
		hc: &http.Client{
			Timeout: time.Duration(defaultTimeout) * time.Second,
			// 禁用自动重定向，手动处理（登录中转跳转要吸收 Set-Cookie）
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		timeout:     defaultTimeout,
		maxsize:     defaultMaxSize,
		maxDLCount:  defaultMaxDLCount,
		uploadDelay: [2]int{0, 0},
		challenge:   DefaultChallengeConfig(),
		vei:         defaultVei, // 占位值，initUIDAndVei 动态获取
	}
	for _, opt := range opts {
		opt(c)
	}
	c.rebuildExec()
	return c
}

func (c *Client) rebuildExec() {
	c.exec = httpx.New(httpx.Config{
		UserAgent:  defaultUA,
		Timeout:    time.Duration(c.timeout) * time.Second,
		HTTPClient: c.hc,
	})
}

// ===== 会话生命周期 =====

// Login 登录蓝奏云帐号：过 acw_sc__v2 挑战 → POST accounts.woozooo.com（task=uselogin）
// → 跟随 msgs 中转跳转链落登录态 cookie。
func (c *Client) Login(user, pwd string) error {
	loginPageURL := baseURLAccount + "/accounts.php?action=login&ref=pc.woozooo.com"
	if _, err := c.FetchPageWithChallenge(loginPageURL); err != nil {
		return fmt.Errorf("login page request failed: %w", err)
	}

	body, _, err := c.post(baseURLAccount+pathAccountLogin, map[string]string{
		"task":     "uselogin",
		"username": user,
		"password": pwd,
		"ref":      "pc.woozooo.com",
	}, map[string]string{
		"Referer":          loginPageURL,
		"X-Requested-With": "XMLHttpRequest",
	})
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}

	// zt 前端用宽松比较（== '1'），可能为数字或字符串
	var resp struct {
		Zt   interface{} `json:"zt"`
		Msgs string      `json:"msgs"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("%w: invalid login response: %s", ErrAPIError, string(body))
	}
	if !ztIsOne(resp.Zt) {
		if strings.Contains(resp.Msgs, "密码") {
			return ErrPasswordWrong
		}
		return fmt.Errorf("%w: login failed: %s", ErrAPIError, resp.Msgs)
	}

	if strings.HasPrefix(resp.Msgs, "http") {
		if err := c.followLoginRedirect(resp.Msgs); err != nil {
			return fmt.Errorf("login redirect failed: %w", err)
		}
	}
	c.logged = true
	c.initUID()
	return nil
}

// Logout 登出并清空会话
func (c *Client) Logout() error {
	if _, _, err := c.get(baseURLPC+pathLogout, nil); err != nil {
		return fmt.Errorf("logout request failed: %w", err)
	}
	c.logged = false
	c.cookies = nil
	return nil
}

// ztIsOne 登录响应 zt 是否表示成功（兼容数字 1 与字符串 "1"）。
func ztIsOne(v interface{}) bool {
	switch t := v.(type) {
	case float64:
		return t == 1
	case string:
		return t == "1"
	case json.Number:
		return t.String() == "1"
	}
	return false
}

// followLoginRedirect 跟随中转鉴权跳转链（最多 5 跳），落登录态 cookie。
func (c *Client) followLoginRedirect(next string) error {
	for i := 0; i < 5 && strings.HasPrefix(next, "http"); i++ {
		_, hdr, err := c.get(next, nil)
		if err != nil {
			return err
		}
		loc := hdr.Get("Location")
		if loc == "" {
			return nil
		}
		if !strings.HasPrefix(loc, "http") {
			u, err := url.Parse(next)
			if err != nil {
				return err
			}
			u = u.ResolveReference(&url.URL{Path: loc})
			loc = u.String()
		}
		next = loc
	}
	return nil
}

// ===== 运行时配置 =====

// SetTimeout 设置HTTP超时
func (c *Client) SetTimeout(seconds int) {
	c.timeout = seconds
	c.hc.Timeout = time.Duration(seconds) * time.Second
}

// SetMaxSize 设置最大文件大小限制
func (c *Client) SetMaxSize(size int) { c.maxsize = size }

// SetMaxDownloadCount 设置最大下载并发数
func (c *Client) SetMaxDownloadCount(n int) {
	if n > 0 {
		c.maxDLCount = n
	}
}

// SetUploadDelay 设置上传延迟范围(毫秒)
func (c *Client) SetUploadDelay(min, max int) { c.uploadDelay = [2]int{min, max} }

// SetChallengeConfig 运行时更新 acw_sc__v2 挑战参数
func (c *Client) SetChallengeConfig(cfg *ChallengeConfig) {
	if cfg != nil {
		c.challenge = cfg
	}
}

// GetChallengeConfig 获取当前挑战参数
func (c *Client) GetChallengeConfig() *ChallengeConfig { return c.challenge }

// ===== cookie 会话 =====

// SetCookies 注入 Cookie（用于直接使用浏览器 Cookie）
func (c *Client) SetCookies(cookies []*http.Cookie) {
	c.mergeCookies(cookies)
	c.logged = true
}

// SetCookiesFromMap 从 map 注入 Cookie
func (c *Client) SetCookiesFromMap(cookieMap map[string]string) {
	cookies := make([]*http.Cookie, 0, len(cookieMap))
	for k, v := range cookieMap {
		cookies = append(cookies, &http.Cookie{Name: k, Value: v})
	}
	c.mergeCookies(cookies)
	c.logged = true
}

// GetCookieString 导出当前会话 cookie（"k1=v1; k2=v2"），可持久化后 SetCookiesFromMap 免登恢复。
func (c *Client) GetCookieString() string {
	parts := make([]string, 0, len(c.cookies))
	for _, ck := range c.cookies {
		parts = append(parts, ck.Name+"="+ck.Value)
	}
	return strings.Join(parts, "; ")
}

// ===== Service 访问器 =====

// Account 返回账号信息服务。
func (c *Client) Account() *account.Service { return account.New(c) }

// Files 返回文件服务。
func (c *Client) Files() *file.Service { return file.New(c) }

// Folders 返回文件夹服务。
func (c *Client) Folders() *folder.Service { return folder.New(c) }

// Upload 返回上传服务。
func (c *Client) Upload() *upload.Service { return upload.New(c) }

// Download 返回下载服务。
func (c *Client) Download() *download.Service { return download.New(c) }

// Recycle 返回回收站服务。
func (c *Client) Recycle() *recycle.Service { return recycle.New(c) }

// Resolve 返回直链解析服务（无需登录）。
func (c *Client) Resolve() *resolve.Service { return resolve.New(c) }

// ===== core/invoker 标准菜单实现 =====

// Get 发 GET，成功时把 JSON 响应解到 out（nil 忽略）。
func (c *Client) Get(ctx context.Context, path string, query map[string]string, out any) error {
	if strings.Contains(path, "doupload.php") {
		c.initUIDAndVei()
	}
	body, _, err := c.get(c.resolveURL(path, query), nil)
	if err != nil {
		return err
	}
	return c.decode(body, out)
}

// Post 发 POST：body 为 map[string]string 时走表单，否则 JSON。
func (c *Client) Post(ctx context.Context, path string, body any, query map[string]string, out any) error {
	if form, ok := body.(map[string]string); ok {
		return c.PostForm(ctx, path, form, out)
	}
	if strings.Contains(path, "doupload.php") {
		c.initUIDAndVei()
	}
	resp, err := c.exec.Do(ctx, &httpx.Request{
		Method:  http.MethodPost,
		URL:     c.resolveURL(path, query),
		Body:    httpx.JSONBody{V: body},
		Headers: c.baseHeaders(nil),
	})
	if err != nil {
		return err
	}
	c.absorbCookies(resp.Header)
	return c.decode(resp.Body, out)
}

// PostForm 发 POST 表单，成功时把 JSON 响应解到 out。
func (c *Client) PostForm(ctx context.Context, path string, form map[string]string, out any) error {
	return c.PostFormHeaders(ctx, path, form, nil, out)
}

// PostFormHeaders 带自定义头的表单 POST（lanZou 方言扩展）。
func (c *Client) PostFormHeaders(ctx context.Context, path string, form map[string]string, headers map[string]string, out any) error {
	if strings.Contains(path, "doupload.php") {
		c.initUIDAndVei()
	} else {
		c.initUID()
	}
	body, _, err := c.post(c.resolveURL(path, nil), form, headers)
	if err != nil {
		return err
	}
	return c.decode(body, out)
}

// Multipart 单体 multipart 上传（html5up.php 方言头在此注入）。
func (c *Client) Multipart(ctx context.Context, path string, form map[string]string, field, filename string, file io.Reader, out any) error {
	body, _, err := c.postMultipart(c.resolveURL(path, nil), form, field, filename, file, map[string]string{
		"Referer": baseURLPC + "/mydisk.php",
		"Origin":  baseURLPC,
	})
	if err != nil {
		return err
	}
	return c.decode(body, out)
}

// PostMultipartStream 流式 multipart 上传（方言扩展，进度回调）。
func (c *Client) PostMultipartStream(ctx context.Context, path string, form map[string]string,
	field, filename string, file io.Reader, fileSize int64,
	onProgress func(uploaded, total int64), headers map[string]string, out any) error {

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	writeErr := make(chan error, 1)
	go func() {
		defer pw.Close()
		defer mw.Close()
		for k, v := range form {
			if err := mw.WriteField(k, v); err != nil {
				writeErr <- err
				return
			}
		}
		part, err := mw.CreateFormFile(field, filename)
		if err != nil {
			writeErr <- err
			return
		}
		if _, err := io.Copy(part, &progressReader{r: file, total: fileSize, onProgress: onProgress}); err != nil {
			writeErr <- err
			return
		}
		writeErr <- nil
	}()

	// 流式请求体走 RawBody：httpx 不缓冲，直接透传 pipe reader
	resp, err := c.exec.Do(ctx, &httpx.Request{
		Method:  http.MethodPost,
		URL:     c.resolveURL(path, nil),
		Body:    httpx.RawBody{MIME: mw.FormDataContentType(), R: pr},
		Headers: c.baseHeaders(headers),
	})
	if err != nil {
		return err
	}
	if err := <-writeErr; err != nil {
		return fmt.Errorf("write multipart body failed: %w", err)
	}
	return c.decode(resp.Body, out)
}

// DownloadHeaders 直链下载所需鉴权头。
func (c *Client) DownloadHeaders() map[string]string {
	return map[string]string{
		"User-Agent": defaultUA,
		"Referer":    baseURLPC + "/",
		"Cookie":     c.cookieHeader(),
	}
}

// ===== lanzou 方言扩展实现 =====

// LoggedIn 返回是否已登录。
func (c *Client) LoggedIn() bool { return c.logged }

// Vei 返回 anti-CSRF token（懒初始化）。
func (c *Client) Vei() string {
	c.initUIDAndVei()
	return c.vei
}

// UserID 返回用户 ID（ylogin cookie，懒提取）。
func (c *Client) UserID() string {
	c.initUID()
	return c.uid
}

// FetchPageWithChallenge 取页面 HTML，自动处理 acw_sc__v2 挑战（最多两轮）。
func (c *Client) FetchPageWithChallenge(pageURL string) (string, error) {
	body, _, err := c.get(pageURL, map[string]string{"Referer": pageURL})
	if err != nil {
		return "", err
	}
	html := string(body)
	if !isChallengePage(html) {
		return html, nil
	}
	cookieVal, err := solveAcwScV2(html, c.challenge)
	if err != nil {
		return "", fmt.Errorf("solve challenge failed: %w", err)
	}
	c.mergeCookies([]*http.Cookie{{Name: "acw_sc__v2", Value: cookieVal}})
	body, _, err = c.get(pageURL, map[string]string{"Referer": pageURL})
	if err != nil {
		return "", err
	}
	html = string(body)
	if isChallengePage(html) { // 有时需要两轮
		cookieVal, err = solveAcwScV2(html, c.challenge)
		if err != nil {
			return "", fmt.Errorf("solve challenge round 2 failed: %w", err)
		}
		c.mergeCookies([]*http.Cookie{{Name: "acw_sc__v2", Value: cookieVal}})
		body, _, err = c.get(pageURL, map[string]string{"Referer": pageURL})
		if err != nil {
			return "", err
		}
		html = string(body)
	}
	return html, nil
}

// HTTPClient 返回底层客户端（大文件流式下载用）。
func (c *Client) HTTPClient() *http.Client { return c.hc }

// MaxSize 返回单文件大小上限（字节）。
func (c *Client) MaxSize() int { return c.maxsize }

// MaxDownloadCount 返回下载并发上限。
func (c *Client) MaxDownloadCount() int { return c.maxDLCount }

// UploadDelay 返回上传延迟范围（毫秒）。
func (c *Client) UploadDelay() (minMs, maxMs int) { return c.uploadDelay[0], c.uploadDelay[1] }

// 编译期保证 Client 实现core 标准菜单与本地方言接口。
var (
	_ invoker.Invoker = (*Client)(nil)
)
