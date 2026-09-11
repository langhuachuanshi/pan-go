package lanzou

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/langhuachuanshi/pan-go/lanzou/account"
	"github.com/langhuachuanshi/pan-go/lanzou/download"
	"github.com/langhuachuanshi/pan-go/lanzou/file"
	"github.com/langhuachuanshi/pan-go/lanzou/folder"
	"github.com/langhuachuanshi/pan-go/lanzou/invoker"
	"github.com/langhuachuanshi/pan-go/lanzou/recycle"
	"github.com/langhuachuanshi/pan-go/lanzou/resolve"
	"github.com/langhuachuanshi/pan-go/lanzou/upload"
)

// Client 蓝奏云客户端（实现 invoker.Invoker）。线程安全边界同 http.Client。
// 业务能力经 Account / Files / Folders / Upload / Download / Recycle / Resolve 访问器提供。
type Client struct {
	httpClient  *http.Client
	cookies     []*http.Cookie
	logged      bool
	maxsize     int                // 最大文件大小限制(字节)
	timeout     int                // HTTP超时(秒)
	maxDLCount  int                // 最大下载并发数
	uploadDelay [2]int             // 上传延迟范围(ms)
	challenge   *ChallengeConfig   // acw_sc__v2 挑战参数
	uid         string             // 用户ID（用于API URL参数）
	vei         string             // vei 参数（anti-CSRF token）
}

// Option 函数式配置选项
type Option func(*Client)

// WithTimeout 设置 HTTP 超时时间
func WithTimeout(seconds int) Option {
	return func(c *Client) {
		c.timeout = seconds
		c.httpClient.Timeout = time.Duration(seconds) * time.Second
	}
}

// WithMaxSize 设置最大文件大小限制(字节)
func WithMaxSize(size int) Option {
	return func(c *Client) {
		c.maxsize = size
	}
}

// WithHTTPClient 自定义 http.Client
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
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
	return func(c *Client) {
		c.uploadDelay = [2]int{min, max}
	}
}

// WithChallengeConfig 自定义 acw_sc__v2 挑战参数
// 当蓝奏云更换JS混淆时，只需更新此配置即可适配
func WithChallengeConfig(cfg *ChallengeConfig) Option {
	return func(c *Client) {
		if cfg != nil {
			c.challenge = cfg
		}
	}
}

// NewClient 创建新的蓝奏云客户端
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: time.Duration(defaultTimeout) * time.Second,
			// 禁用自动重定向，手动处理
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		maxsize:     defaultMaxSize,
		timeout:     defaultTimeout,
		maxDLCount:  defaultMaxDLCount,
		uploadDelay: [2]int{0, 0},
		challenge:   DefaultChallengeConfig(),
		vei:         defaultVei, // 默认占位值，initUIDAndVei() 会动态获取
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ===== 会话生命周期 =====

// Login 登录蓝奏云帐号。
// 蓝奏已把账号系统从 pc.woozooo.com/account/loginajax 迁到 accounts.woozooo.com：
// 先过 acw_sc__v2 JS 反爬挑战拿会话 cookie，再 POST /accounts.php（task=uselogin）。
func (c *Client) Login(user, pwd string) error {
	// Step 1: 请求登录页并自动过 acw_sc__v2 挑战（复用直链解析同款挑战处理）
	loginPageURL := baseURLAccount + "/accounts.php?action=login&ref=pc.woozooo.com"
	if _, err := c.FetchPageWithChallenge(loginPageURL); err != nil {
		return fmt.Errorf("login page request failed: %w", err)
	}

	// Step 2: POST 登录（字段与新版登录页 uselogin() 一致）
	data := map[string]string{
		"task":     "uselogin",
		"username": user,
		"password": pwd,
		"ref":      "pc.woozooo.com",
	}
	body, _, err := c.Post(baseURLAccount+pathAccountLogin, data, map[string]string{
		"Referer":          loginPageURL,
		"X-Requested-With": "XMLHttpRequest",
	})
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}

	// zt 前端用宽松比较（== '1'），可能为数字或字符串，用 interface{} 兼容两种
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

	// Step 3: 成功时 msgs 是中转鉴权跳转 URL，跟随它把最终登录态 cookie 落到 pc.woozooo.com 域
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
	if _, _, err := c.Get(baseURLPC+pathLogout, nil); err != nil {
		return fmt.Errorf("logout request failed: %w", err)
	}
	c.logged = false
	c.cookies = nil
	return nil
}

// ztIsOne 判断登录响应 zt 是否表示成功（兼容数字 1 与字符串 "1"）。
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

// followLoginRedirect 跟随中转鉴权跳转链（最多 5 跳），用于把登录态 cookie 落在最终域。
func (c *Client) followLoginRedirect(next string) error {
	for i := 0; i < 5 && strings.HasPrefix(next, "http"); i++ {
		_, hdr, err := c.Get(next, nil)
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
	c.httpClient.Timeout = time.Duration(seconds) * time.Second
}

// SetMaxSize 设置最大文件大小限制
func (c *Client) SetMaxSize(size int) {
	c.maxsize = size
}

// SetMaxDownloadCount 设置最大下载并发数
func (c *Client) SetMaxDownloadCount(n int) {
	if n > 0 {
		c.maxDLCount = n
	}
}

// SetUploadDelay 设置上传延迟范围(毫秒)
func (c *Client) SetUploadDelay(min, max int) {
	c.uploadDelay = [2]int{min, max}
}

// SetChallengeConfig 运行时更新 acw_sc__v2 挑战参数
// 当蓝奏云更换JS混淆导致直链解析失败时，抓取新的置换表和密钥后调用此方法即可恢复
func (c *Client) SetChallengeConfig(cfg *ChallengeConfig) {
	if cfg != nil {
		c.challenge = cfg
	}
}

// GetChallengeConfig 获取当前挑战参数（可用于序列化保存）
func (c *Client) GetChallengeConfig() *ChallengeConfig {
	return c.challenge
}

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

// GetCookieString 导出当前会话的 cookie（"k1=v1; k2=v2" 单行格式）。
// 账号密码 Login 或注入浏览器 Cookie 后，调用方可用它把会话持久化，
// 下次免登直接 SetCookiesFromMap 恢复。
func (c *Client) GetCookieString() string {
	parts := make([]string, 0, len(c.cookies))
	for _, ck := range c.cookies {
		parts = append(parts, ck.Name+"="+ck.Value)
	}
	return strings.Join(parts, "; ")
}

// ===== Service 访问器 =====

// Account 返回账号信息服务（用户信息/帐号详情）。
func (c *Client) Account() *account.Service { return account.New(c) }

// Files 返回文件服务（列表/分享链接/移动/删除/设密码）。
func (c *Client) Files() *file.Service { return file.New(c) }

// Folders 返回文件夹服务（列表/创建/删除/移动）。
func (c *Client) Folders() *folder.Service { return folder.New(c) }

// Upload 返回上传服务。
func (c *Client) Upload() *upload.Service { return upload.New(c) }

// Download 返回下载服务。
func (c *Client) Download() *download.Service { return download.New(c) }

// Recycle 返回回收站服务。
func (c *Client) Recycle() *recycle.Service { return recycle.New(c) }

// Resolve 返回直链解析服务（无需登录）。
func (c *Client) Resolve() *resolve.Service { return resolve.New(c) }

// ===== invoker.Invoker 实现 =====

// LoggedIn 返回是否已登录。
func (c *Client) LoggedIn() bool { return c.logged }

// UserID 返回用户 ID（ylogin cookie，懒提取）。
func (c *Client) UserID() string {
	c.initUID()
	return c.uid
}

// Vei 返回 anti-CSRF token（懒初始化）。
func (c *Client) Vei() string {
	c.initUIDAndVei()
	return c.vei
}

// TaskURL 返回 doupload.php 完整地址（带 uid），并懒初始化 uid/vei。
func (c *Client) TaskURL() string {
	c.initUIDAndVei()
	return c.apiURL(pathTaskAPI)
}

// UploadURL 返回 html5up.php 上传入口。
func (c *Client) UploadURL() string { return baseURLPC + pathUpload }

// AjaxmURL 返回 ajaxm.php 完整地址（带 uid）。
func (c *Client) AjaxmURL() string { return c.apiURL(pathAjaxm) }

// FetchPageWithChallenge 请求页面并自动处理 acw_sc__v2 JS 挑战（最多两轮）。
func (c *Client) FetchPageWithChallenge(pageURL string) (string, error) {
	body, _, err := c.get(pageURL, map[string]string{
		"Referer": pageURL,
	})
	if err != nil {
		return "", err
	}
	html := string(body)

	// 检查是否为JS挑战页面
	if isChallengePage(html) {
		// 解出 acw_sc__v2 cookie
		cookieVal, err := solveAcwScV2(html, c.challenge)
		if err != nil {
			return "", fmt.Errorf("solve challenge failed: %w", err)
		}

		// 注入cookie并重新请求
		c.mergeCookies([]*http.Cookie{{
			Name:  "acw_sc__v2",
			Value: cookieVal,
		}})

		body, _, err = c.get(pageURL, map[string]string{
			"Referer": pageURL,
		})
		if err != nil {
			return "", err
		}
		html = string(body)

		// 二次检查（有时需要两轮）
		if isChallengePage(html) {
			cookieVal, err = solveAcwScV2(html, c.challenge)
			if err != nil {
				return "", fmt.Errorf("solve challenge round 2 failed: %w", err)
			}
			c.mergeCookies([]*http.Cookie{{
				Name:  "acw_sc__v2",
				Value: cookieVal,
			}})
			body, _, err = c.get(pageURL, map[string]string{
				"Referer": pageURL,
			})
			if err != nil {
				return "", err
			}
			html = string(body)
		}
	}

	return html, nil
}

// Get 发 GET 请求并注入会话 cookie。
func (c *Client) Get(rawURL string, headers map[string]string) ([]byte, http.Header, error) {
	return c.get(rawURL, headers)
}

// Post 发表单 POST 请求并注入会话 cookie。
func (c *Client) Post(rawURL string, data map[string]string, headers map[string]string) ([]byte, http.Header, error) {
	return c.post(rawURL, data, headers)
}

// PostMultipart 发 multipart POST（一次性载入内存）。
func (c *Client) PostMultipart(rawURL string, fields map[string]string, fileField, fileName string, fileReader io.Reader, headers map[string]string) ([]byte, http.Header, error) {
	return c.postMultipart(rawURL, fields, fileField, fileName, fileReader, headers)
}

// HTTPClient 返回底层客户端。
func (c *Client) HTTPClient() *http.Client { return c.httpClient }

// UserAgent 返回默认 UA。
func (c *Client) UserAgent() string { return defaultUA }

// MaxDownloadCount 返回下载并发上限。
func (c *Client) MaxDownloadCount() int { return c.maxDLCount }

// MaxSize 返回单文件大小上限（字节）。
func (c *Client) MaxSize() int { return c.maxsize }

// UploadDelay 返回上传延迟范围（毫秒）。
func (c *Client) UploadDelay() (minMs, maxMs int) { return c.uploadDelay[0], c.uploadDelay[1] }

// 编译期保证 Client 实现 invoker.Invoker。
var _ invoker.Invoker = (*Client)(nil)
