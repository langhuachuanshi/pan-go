// Package auth 保存百度网盘网页端的 BDUSS + STOKEN 凭证，并获取 bdstoken。
//
// 网页端鉴权三件套（实测确认）：
//   - BDUSS：网页登录长效 cookie（domain=.baidu.com），所有接口必需
//   - STOKEN：写操作必需（list 只需 BDUSS，但 precreate/create/filemanager 需 STOKEN）
//   - bdstoken：写操作的 CSRF 令牌，从 /api/gettemplatevariable 获取，放 query
//
// 三者获取方式（手动）：浏览器登录 pan.baidu.com → F12 → Application → Cookies →
// pan.baidu.com → 复制 BDUSS 和 STOKEN。bdstoken 由本包自动获取（依赖 BDUSS+STOKEN）。
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
)

// 通用 query 参数（网页端所有接口都要带，实测确认）。
const (
	channel    = "chunlei"
	web        = "1"
	clienttype = "0"
	appID      = "250528"
)

// bdstoken 获取接口。
const templateVarURL = "https://pan.baidu.com/api/gettemplatevariable"

// Config auth 配置。
type Config struct {
	BDUSS  string // 必填
	STOKEN string // 写操作必填（list 可空）
}

// Manager 管理凭证和 bdstoken。线程安全。
type Manager struct {
	bduss    string
	stoken   string
	http     *http.Client
	mu       sync.Mutex
	bdstoken string
}

// New 创建 Manager。
func New(cfg *Config) *Manager {
	return &Manager{
		bduss:  cfg.BDUSS,
		stoken: cfg.STOKEN,
		http:   &http.Client{Timeout: 30 * 1e9},
	}
}

// BDUSSCookie 返回 BDUSS 的 cookie 值。
func (m *Manager) BDUSSCookie() string { return m.bduss }

// STOKENCookie 返回 STOKEN 的 cookie 值。
func (m *Manager) STOKENCookie() string { return m.stoken }

// HasSTOKEN 是否配置了 STOKEN（写操作前置检查用）。
func (m *Manager) HasSTOKEN() bool { return m.stoken != "" }

// CommonQuery 返回网页端通用 query 参数（channel/web/app_id/clienttype）。
// 业务层把它和自己的参数合并。
func CommonQuery() map[string]string {
	return map[string]string{
		"channel":    channel,
		"web":        web,
		"app_id":     appID,
		"clienttype": clienttype,
	}
}

// BDstoken 获取并缓存 bdstoken（写操作必需）。需要 BDUSS+STOKEN。
func (m *Manager) BDstoken(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.bdstoken != "" {
		return m.bdstoken, nil
	}
	if m.bduss == "" || m.stoken == "" {
		return "", fmt.Errorf("bdstoken 获取需要 BDUSS 和 STOKEN")
	}
	q := url.Values{}
	q.Set("clienttype", clienttype)
	q.Set("app_id", appID)
	q.Set("web", web)
	q.Set("fields", `["bdstoken"]`)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, templateVarURL+"?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Cookie", fmt.Sprintf("BDUSS=%s; STOKEN=%s", m.bduss, m.stoken))
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := m.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var raw struct {
		Errno  int `json:"errno"`
		Result struct {
			BDSToken string `json:"bdstoken"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return "", fmt.Errorf("解析 bdstoken 响应失败: %w", err)
	}
	if raw.Errno != 0 || raw.Result.BDSToken == "" {
		return "", fmt.Errorf("获取 bdstoken 失败: errno=%d", raw.Errno)
	}
	m.bdstoken = raw.Result.BDSToken
	return m.bdstoken, nil
}
