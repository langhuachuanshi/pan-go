package lanzou

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/langhuachuanshi/pan-go/core/httpx"
)

// ===== HTTP 底层（执行走 core/httpx；cookie/挑战是 lanzou 方言，留在此处）=====

// cookieHeader 当前会话 cookie 的单行格式（请求头用）。
func (c *Client) cookieHeader() string { return c.GetCookieString() }

// baseHeaders 默认头 + 会话 cookie；overrides 覆盖同名键。
func (c *Client) baseHeaders(overrides map[string]string) map[string]string {
	h := map[string]string{
		"Accept":          "application/json, text/javascript, */*; q=0.01",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"Referer":         baseURLPC + "/",
		"Cookie":          c.cookieHeader(),
	}
	for k, v := range overrides {
		h[k] = v
	}
	return h
}

// absorbCookies 把响应 Set-Cookie 合并进会话（蓝奏手动管理 cookie，不用 jar）。
func (c *Client) absorbCookies(h http.Header) {
	for _, line := range h.Values("Set-Cookie") {
		if ck, err := http.ParseSetCookie(line); err == nil {
			c.mergeCookies([]*http.Cookie{ck})
		}
	}
}

// get GET 并合并 Set-Cookie。
func (c *Client) get(rawURL string, headers map[string]string) ([]byte, http.Header, error) {
	resp, err := c.exec.Do(context.Background(), &httpx.Request{
		URL:     rawURL,
		Headers: c.baseHeaders(headers),
	})
	if err != nil {
		return nil, nil, err
	}
	c.absorbCookies(resp.Header)
	return resp.Body, resp.Header, nil
}

// post 表单 POST 并合并 Set-Cookie。
func (c *Client) post(rawURL string, data map[string]string, headers map[string]string) ([]byte, http.Header, error) {
	resp, err := c.exec.Do(context.Background(), &httpx.Request{
		Method:  http.MethodPost,
		URL:     rawURL,
		Body:    httpx.FormBody(data),
		Headers: c.baseHeaders(headers),
	})
	if err != nil {
		return nil, nil, err
	}
	c.absorbCookies(resp.Header)
	return resp.Body, resp.Header, nil
}

// postMultipart 单体 multipart 上传（一次性载入内存），构建后走 httpx RawBody。
func (c *Client) postMultipart(rawURL string, fields map[string]string, fileField, fileName string, fileReader io.Reader, headers map[string]string) ([]byte, http.Header, error) {
	body := &bytes.Buffer{}
	boundary := "----WebKitFormBoundary" + randomString(16)

	for k, v := range fields {
		fmt.Fprintf(body, "--%s\r\n", boundary)
		fmt.Fprintf(body, "Content-Disposition: form-data; name=\"%s\"\r\n\r\n", k)
		fmt.Fprintf(body, "%s\r\n", v)
	}
	fmt.Fprintf(body, "--%s\r\n", boundary)
	fmt.Fprintf(body, "Content-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\n", fileField, fileName)
	fmt.Fprintf(body, "Content-Type: application/octet-stream\r\n\r\n")
	if _, err := io.Copy(body, fileReader); err != nil {
		return nil, nil, err
	}
	fmt.Fprintf(body, "\r\n--%s--\r\n", boundary)

	resp, err := c.exec.Do(context.Background(), &httpx.Request{
		Method:  http.MethodPost,
		URL:     rawURL,
		Body:    httpx.RawBody{MIME: "multipart/form-data; boundary=" + boundary, R: bytes.NewReader(body.Bytes())},
		Headers: c.baseHeaders(headers),
	})
	if err != nil {
		return nil, nil, err
	}
	c.absorbCookies(resp.Header)
	return resp.Body, resp.Header, nil
}

// mergeCookies 合并 cookies（按名称去重）。
func (c *Client) mergeCookies(newCookies []*http.Cookie) {
	cookieMap := make(map[string]*http.Cookie)
	for _, old := range c.cookies {
		cookieMap[old.Name] = old
	}
	for _, ck := range newCookies {
		cookieMap[ck.Name] = ck
	}
	c.cookies = make([]*http.Cookie, 0, len(cookieMap))
	for _, ck := range cookieMap {
		c.cookies = append(c.cookies, ck)
	}
}

// getCookieValue 取指定名称的 cookie 值。
func (c *Client) getCookieValue(name string) string {
	for _, ck := range c.cookies {
		if ck.Name == name {
			return ck.Value
		}
	}
	return ""
}

// initUID 从 cookie 提取 uid。
func (c *Client) initUID() {
	if c.uid == "" {
		c.uid = c.getCookieValue("ylogin")
	}
}

// initUIDAndVei 初始化 uid 和 vei（vei 是 anti-CSRF token，从 mydisk.php 页面提取）。
func (c *Client) initUIDAndVei() {
	c.initUID()
	if c.vei != "" && c.vei != defaultVei {
		return // 已初始化过动态 vei
	}
	pageURL := baseURLPC + "/mydisk.php?item=files&action=index"
	body, _, err := c.get(pageURL, nil)
	if err != nil {
		return
	}
	for _, pat := range []string{`'vei':'([^']+)'`, `"vei":"([^"]+)"`} {
		re := regexp.MustCompile(pat)
		if m := re.FindStringSubmatch(string(body)); len(m) >= 2 {
			c.vei = m[1]
			return
		}
	}
}

// resolveURL 统一地址解析：相对路径补 base，doupload/ajaxm 附加 uid，合并 query。
func (c *Client) resolveURL(path string, query map[string]string) string {
	u := path
	if !strings.HasPrefix(path, "http") {
		u = baseURLPC + path
	}
	q := url.Values{}
	for k, v := range query {
		q.Set(k, v)
	}
	c.initUID()
	if c.uid != "" && (strings.Contains(path, "doupload.php") || strings.Contains(path, "ajaxm.php")) {
		q.Set("uid", c.uid)
	}
	if len(q) > 0 {
		sep := "?"
		if strings.Contains(u, "?") {
			sep = "&"
		}
		u += sep + q.Encode()
	}
	return u
}

// decode JSON 解到 out（nil 忽略）。
func (c *Client) decode(body []byte, out any) error {
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%w: invalid json response: %s", ErrAPIError, truncate(string(body), 200))
	}
	return nil
}

// randomString 生成随机串（multipart boundary 用）。
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// truncate 截断字符串用于错误信息。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
