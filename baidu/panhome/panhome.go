// Package panhome 从百度网盘首页 (/disk/home) 提取签名参数。
//
// 百度网页端 /api/download 接口需要三个签名参数：
//   - sign1、sign3：从 /disk/home 的 HTML 中通过正则提取
//   - timestamp：同上
//
// 然后用 RC4 (sign2) 对 sign1 用 sign3 加密，得到最终签名 sign。
// 签名缓存 1 小时有效。
package panhome

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/langhuachuanshi/baidupan-go/baidu/sign"
)

// signInfoRE 从 /disk/home HTML 中提取 sign1、sign3、timestamp。
var signInfoRE = regexp.MustCompile(`"sign1":"(.*?)"[\s\S]*"sign3":"(.*?)","timestamp":(\d*?),`)

// SignRes 签名结果。
type SignRes struct {
	Sign      string // Base64 编码后的签名
	Timestamp string // 提取的时间戳
}

// Cache 签名缓存（线程安全）。
type Cache struct {
	mu       sync.Mutex
	client   *http.Client
	signRes  *SignRes
	expires  time.Time
}

// NewCache 创建签名缓存。
// httpClient 应携带 BDUSS + STOKEN cookie（用于访问 /disk/home）。
func NewCache(httpClient *http.Client) *Cache {
	return &Cache{client: httpClient}
}

// CacheSignature 在有效期内返回缓存的签名；过期则重新抓取。
func (c *Cache) CacheSignature() (*SignRes, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.signRes != nil && time.Now().Before(c.expires) {
		return c.signRes, nil
	}

	res, err := c.fetch()
	if err != nil {
		return nil, err
	}
	c.signRes = res
	c.expires = time.Now().Add(1 * time.Hour)
	return res, nil
}

// Reset 强制过期缓存，下次调用重新抓取。
func (c *Cache) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.signRes = nil
}

// fetch 从 /disk/home 抓取并计算签名。
func (c *Cache) fetch() (*SignRes, error) {
	req, err := http.NewRequest(http.MethodGet, "https://pan.baidu.com/disk/home", nil)
	if err != nil {
		return nil, fmt.Errorf("panhome: 创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("panhome: 请求 /disk/home 失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查重定向
	loc := resp.Header.Get("Location")
	if loc == "/" {
		return nil, fmt.Errorf("panhome: BDUSS 已失效（重定向到 /）")
	}
	if strings.Contains(loc, "passport.baidu.com") {
		return nil, fmt.Errorf("panhome: BDUSS 已失效（重定向到 passport）")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("panhome: 读取响应失败: %w", err)
	}

	match := signInfoRE.FindSubmatch(body)
	if len(match) <= 3 {
		return nil, fmt.Errorf("panhome: 未能从 /disk/home 提取 sign1/sign3/timestamp")
	}

	sign1 := []rune(string(match[1]))
	sign3 := []rune(string(match[2]))
	timestamp := string(match[3])

	// RC4 加密 + Base64 编码
	encrypted := sign.Sign2(sign3, sign1)
	encoded := base64.StdEncoding.EncodeToString(encrypted)

	return &SignRes{
		Sign:      encoded,
		Timestamp: timestamp,
	}, nil
}
