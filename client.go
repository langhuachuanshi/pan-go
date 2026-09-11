package lanzou

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Client 蓝奏云客户端
type Client struct {
	httpClient   *http.Client
	cookies      []*http.Cookie
	logged       bool
	maxsize      int           // 最大文件大小限制(字节)
	timeout      int           // HTTP超时(秒)
	maxDLCount   int           // 最大下载并发数
	uploadDelay  [2]int        // 上传延迟范围(ms)
	challenge    *ChallengeConfig // acw_sc__v2 挑战参数
	uid          string         // 用户ID（用于API URL参数）
	vei          string         // vei 参数（anti-CSRF token）
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
		maxsize:    defaultMaxSize,
		timeout:    defaultTimeout,
		maxDLCount: defaultMaxDLCount,
		uploadDelay: [2]int{0, 0},
		challenge:  DefaultChallengeConfig(),
		vei:        defaultVei, // 默认占位值，initUIDAndVei() 会动态获取
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// get 发送 GET 请求
func (c *Client) get(rawURL string, headers map[string]string) ([]byte, http.Header, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, nil, err
	}
	c.setCommonHeaders(req)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	// 注入 cookies
	for _, cookie := range c.cookies {
		req.AddCookie(cookie)
	}
	return c.doRequest(req)
}

// post 发送 POST 请求（表单）
func (c *Client) post(rawURL string, data map[string]string, headers map[string]string) ([]byte, http.Header, error) {
	form := url.Values{}
	for k, v := range data {
		form.Set(k, v)
	}
	req, err := http.NewRequest("POST", rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, nil, err
	}
	c.setCommonHeaders(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for _, cookie := range c.cookies {
		req.AddCookie(cookie)
	}
	return c.doRequest(req)
}

// postMultipart 发送 multipart POST 请求（文件上传）
func (c *Client) postMultipart(rawURL string, fields map[string]string, fileField, fileName string, fileReader io.Reader, headers map[string]string) ([]byte, http.Header, error) {
	body := &bytes.Buffer{}
	boundary := "----WebKitFormBoundary" + randomString(16)
	writer := io.Writer(body)

	// 写入普通字段
	for k, v := range fields {
		fmt.Fprintf(writer, "--%s\r\n", boundary)
		fmt.Fprintf(writer, "Content-Disposition: form-data; name=\"%s\"\r\n\r\n", k)
		fmt.Fprintf(writer, "%s\r\n", v)
	}
	// 写入文件字段
	fmt.Fprintf(writer, "--%s\r\n", boundary)
	fmt.Fprintf(writer, "Content-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\n", fileField, fileName)
	fmt.Fprintf(writer, "Content-Type: application/octet-stream\r\n\r\n")
	if _, err := io.Copy(writer, fileReader); err != nil {
		return nil, nil, err
	}
	fmt.Fprintf(writer, "\r\n--%s--\r\n", boundary)

	req, err := http.NewRequest("POST", rawURL, body)
	if err != nil {
		return nil, nil, err
	}
	c.setCommonHeaders(req)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for _, cookie := range c.cookies {
		req.AddCookie(cookie)
	}
	return c.doRequest(req)
}

// doRequest 执行请求并保存 cookies
func (c *Client) doRequest(req *http.Request) ([]byte, http.Header, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	// 保存 cookies
	if len(resp.Cookies()) > 0 {
		c.mergeCookies(resp.Cookies())
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return body, resp.Header, nil
}

// setCommonHeaders 设置通用请求头
func (c *Client) setCommonHeaders(req *http.Request) {
	req.Header.Set("User-Agent", defaultUA)
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", baseURLPC+"/")
}

// mergeCookies 合并 cookies（按名称去重）
func (c *Client) mergeCookies(newCookies []*http.Cookie) {
	cookieMap := make(map[string]*http.Cookie)
	for _, old := range c.cookies {
		cookieMap[old.Name] = old
	}
	for _, new := range newCookies {
		cookieMap[new.Name] = new
	}
	c.cookies = make([]*http.Cookie, 0, len(cookieMap))
	for _, cookie := range cookieMap {
		c.cookies = append(c.cookies, cookie)
	}
}

// apiURL 构建 API URL（自动附加 uid）
func (c *Client) apiURL(path string) string {
	url := baseURLPC + path
	if c.uid != "" {
		url += "?uid=" + c.uid
	}
	return url
}

// initUID 从 cookie 中提取 uid
func (c *Client) initUID() {
	if c.uid == "" {
		c.uid = c.getCookieValue("ylogin")
	}
}

// initUIDAndVei 初始化 uid 和 vei（vei 是 anti-CSRF token，从 mydisk.php 页面 JS 中提取）
// vei 是页面 JS 中硬编码的静态值，形如 'V1NTUQ1fAg4PDVdW'，会随会话变化
func (c *Client) initUIDAndVei() {
	c.initUID()
	if c.vei != "" && c.vei != defaultVei {
		return // 已经初始化过动态 vei
	}
	// 从 mydisk.php 页面提取 vei
	pageURL := baseURLPC + "/mydisk.php?item=files&action=index"
	body, _, err := c.get(pageURL, nil)
	if err != nil {
		return
	}
	html := string(body)
	// 匹配 JS 中的 vei 值: 'vei':'XXXXXXX'
	for _, pat := range []string{`'vei':'([^']+)'`, `"vei":"([^"]+)"`} {
		re := regexp.MustCompile(pat)
		if m := re.FindStringSubmatch(html); len(m) >= 2 {
			c.vei = m[1]
			return
		}
	}
}

// getCookieValue 获取指定名称的 cookie 值
func (c *Client) getCookieValue(name string) string {
	for _, cookie := range c.cookies {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

// isLoggedIn 检查是否已登录
func (c *Client) isLoggedIn() bool {
	return c.logged
}

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
