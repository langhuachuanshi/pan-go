package lanzou

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// get 发 GET 请求
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

// post 发表单 POST 请求
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

// postMultipart 发送 multipart POST 请求（文件上传，一次性载入内存）
func (c *Client) postMultipart(rawURL string, fields map[string]string, fileField, fileName string, fileReader io.Reader, headers map[string]string) ([]byte, http.Header, error) {
	body := &bytes.Buffer{}
	boundary := "----WebKitFormBoundary" + randomString(16)

	// 写入普通字段
	for k, v := range fields {
		fmt.Fprintf(body, "--%s\r\n", boundary)
		fmt.Fprintf(body, "Content-Disposition: form-data; name=\"%s\"\r\n\r\n", k)
		fmt.Fprintf(body, "%s\r\n", v)
	}
	// 写入文件字段
	fmt.Fprintf(body, "--%s\r\n", boundary)
	fmt.Fprintf(body, "Content-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\n", fileField, fileName)
	fmt.Fprintf(body, "Content-Type: application/octet-stream\r\n\r\n")
	if _, err := io.Copy(body, fileReader); err != nil {
		return nil, nil, err
	}
	fmt.Fprintf(body, "\r\n--%s--\r\n", boundary)

	req, err := http.NewRequest("POST", rawURL, bytes.NewReader(body.Bytes()))
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

// getCookieValue 获取指定名称的 cookie 值
func (c *Client) getCookieValue(name string) string {
	for _, cookie := range c.cookies {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

// initUID 从 cookie 中提取 uid
func (c *Client) initUID() {
	if c.uid == "" {
		c.uid = c.getCookieValue("ylogin")
	}
}

// initUIDAndVei 初始化 uid 和 vei（vei 是 anti-CSRF token，从 mydisk.php 页面 JS 中提取）
// vei 是页面 JS 中的静态值，形如 'V1NTUQ1fAg4PDVdW'，会随会话变化
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

// apiURL 构建 API URL（自动附加 uid）
func (c *Client) apiURL(path string) string {
	u := baseURLPC + path
	if c.uid != "" {
		u += "?uid=" + c.uid
	}
	return u
}

// randomString 生成指定长度的随机字符串（multipart boundary 用）
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
