package lanzou

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
)

// reArg1 提取 arg1 的正则
var reArg1 = regexp.MustCompile(`var\s+arg1\s*=\s*'([A-F0-9]{40})'`)

// solveAcwScV2 计算 acw_sc__v2 cookie 值
func solveAcwScV2(html string, cfg *ChallengeConfig) (string, error) {
	if cfg == nil {
		cfg = DefaultChallengeConfig()
	}

	matches := reArg1.FindStringSubmatch(html)
	if len(matches) < 2 {
		return "", fmt.Errorf("arg1 not found in challenge page")
	}
	arg1 := matches[1]

	var q [40]byte
	for x := 0; x < len(arg1); x++ {
		for z := 0; z < len(cfg.Perm); z++ {
			if cfg.Perm[z] == x+1 {
				q[z] = arg1[x]
			}
		}
	}
	u := string(q[:])

	xorKey := cfg.XORKey
	var v strings.Builder
	for x := 0; x < len(u) && x < len(xorKey); x += 2 {
		a, _ := strconv.ParseUint(u[x:x+2], 16, 8)
		b, _ := strconv.ParseUint(xorKey[x:x+2], 16, 8)
		v.WriteString(fmt.Sprintf("%02x", a^b))
	}
	return v.String(), nil
}

// isChallengePage 检查是否为 JS 挑战页面
func isChallengePage(html string) bool {
	return strings.Contains(html, "var arg1=") && strings.Contains(html, "acw_sc__v2")
}

// ===== 页面解析正则 =====

var reWpSign = regexp.MustCompile(`wp_sign\s*=\s*'([^']+)'`)
var reIframeSrc = regexp.MustCompile(`<iframe[^>]+src="(/fn\?[^"]+)"`)
var reFid = regexp.MustCompile(`var\s+fid\s*=\s*(\d+)`)
var reTitle = regexp.MustCompile(`<title>([^<]+)</title>`)
var reFileSizeDesc = regexp.MustCompile(`文件大小[：:]\s*([^<\s]+(?:\s*[A-Za-z]+)?)`)
var reFileSizeMeta = regexp.MustCompile(`文件大小[：:]\s*(\d+\.?\d*\s*[A-Za-z]+)`)

// ===== 数据提取 =====

type pageData struct {
	Fid      int
	Title    string
	FileSize string
	Iframe   string
}

func extractPageData(html string) (*pageData, error) {
	data := &pageData{}

	if m := reFid.FindStringSubmatch(html); len(m) > 1 {
		data.Fid, _ = strconv.Atoi(m[1])
	}
	if m := reTitle.FindStringSubmatch(html); len(m) > 1 {
		title := strings.TrimSpace(m[1])
		title = strings.TrimSuffix(title, " - 蓝奏云")
		title = strings.TrimSuffix(title, " - 蓝奏云盘")
		data.Title = title
	}
	if m := reFileSizeDesc.FindStringSubmatch(html); len(m) > 1 {
		data.FileSize = strings.TrimSpace(m[1])
	} else if m := reFileSizeMeta.FindStringSubmatch(html); len(m) > 1 {
		data.FileSize = strings.TrimSpace(m[1])
	}
	if m := reIframeSrc.FindStringSubmatch(html); len(m) > 1 {
		data.Iframe = m[1]
	}

	if data.Fid == 0 && data.Iframe == "" {
		return nil, fmt.Errorf("%w: no fid or iframe found", ErrExtractFailed)
	}
	return data, nil
}

type iframeData struct {
	WpSign   string
	Fid      int
	AjaxmURL string
}

func extractIframeData(html string) (*iframeData, error) {
	data := &iframeData{}

	if m := reWpSign.FindStringSubmatch(html); len(m) > 1 {
		data.WpSign = m[1]
	} else {
		return nil, fmt.Errorf("%w: wp_sign not found", ErrExtractFailed)
	}

	reAjaxm := regexp.MustCompile(`url\s*:\s*'(/ajaxm\.php\?file=(\d+))'`)
	if m := reAjaxm.FindStringSubmatch(html); len(m) > 2 {
		data.AjaxmURL = m[1]
		data.Fid, _ = strconv.Atoi(m[2])
	}
	return data, nil
}

type ajaxmResp struct {
	Zt   int             `json:"zt"`
	Dom  string          `json:"dom"`
	Url  string          `json:"url"`
	Inf  json.RawMessage `json:"inf"`
	Mess string          `json:"mess"`
}

// ===== 通用工具 =====

func isLanzouURL(url string) bool {
	for _, domain := range lanzouDomains {
		if strings.Contains(url, domain) {
			return true
		}
	}
	return strings.Contains(url, "lanzou") || strings.Contains(url, "woozooo")
}

func normalizeURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	return strings.TrimSuffix(rawURL, "/")
}

func getBaseHost(rawURL string) string {
	rawURL = strings.TrimPrefix(rawURL, "https://")
	rawURL = strings.TrimPrefix(rawURL, "http://")
	if idx := strings.Index(rawURL, "/"); idx > 0 {
		return rawURL[:idx]
	}
	return rawURL
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
