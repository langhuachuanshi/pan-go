// Package auth 实现百度网盘开放平台的 OAuth 2.0 鉴权。
//
// 流程（手动换码模式）：
//  1. AuthorizeURL() 生成授权链接，用户浏览器打开并同意授权
//  2. 百度重定向到 redirect_uri?code=xxx（用 oob 时百度页面直接显示 code）
//  3. ExchangeToken(code) 用 code 换 access_token + refresh_token
//  4. token 持久化到本地文件，access_token 有效期 30 天，过期用 refresh_token 自动刷新
//
// OAuth 端点域名是 openapi.baidu.com（不是 pan.baidu.com）。
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// OAuth 端点。
const (
	authorizeURL = "https://openapi.baidu.com/oauth/2.0/authorize"
	tokenURL     = "https://openapi.baidu.com/oauth/2.0/token"
)

// 网盘读写权限（逗号分隔）。
const defaultScope = "basic,netdisk"

// Config auth 配置。
type Config struct {
	AppKey     string // 应用 AppKey（client_id）
	SecretKey  string // 应用 SecretKey（client_secret）
	RedirectURI string // 授权回调地址，默认 oob
	TokenFile  string // token 持久化文件，默认 ~/.panbaidu/token.json
}

// Token OAuth 令牌。
type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`      // 有效期秒数（通常 30 天）
	ExpiresAt    int64  `json:"expires_at"`      // 绝对过期时间戳（秒），刷新前判断用
	Scope        string `json:"scope"`
}

// IsExpired 判断 access_token 是否已过期（提前 60s 容错）。
func (t *Token) IsExpired() bool {
	if t == nil || t.AccessToken == "" {
		return true
	}
	return time.Now().Unix()+60 >= t.ExpiresAt
}

// Manager 管理 token 的获取、持久化、刷新。线程安全。
type Manager struct {
	cfg   *Config
	mu    sync.Mutex
	token *Token
	http  *http.Client
}

// New 创建 Manager。
func New(cfg *Config) *Manager {
	if cfg.RedirectURI == "" {
		cfg.RedirectURI = "oob"
	}
	if cfg.TokenFile == "" {
		cfg.TokenFile = defaultTokenPath()
	}
	return &Manager{cfg: cfg, http: &http.Client{Timeout: 30 * time.Second}}
}

// AuthorizeURL 生成用户授权链接（浏览器打开，授权后拿 code）。
func (m *Manager) AuthorizeURL() string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", m.cfg.AppKey)
	q.Set("redirect_uri", m.cfg.RedirectURI)
	q.Set("scope", defaultScope)
	return authorizeURL + "?" + q.Encode()
}

// ExchangeToken 用授权码换 token，并持久化。
func (m *Manager) ExchangeToken(ctx context.Context, code string) (*Token, error) {
	t, err := m.requestToken(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {m.cfg.AppKey},
		"client_secret": {m.cfg.SecretKey},
		"redirect_uri":  {m.cfg.RedirectURI},
	})
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.token = t
	m.mu.Unlock()
	if err := m.save(); err != nil {
		return t, fmt.Errorf("token 获取成功但持久化失败: %w", err)
	}
	return t, nil
}

// Token 返回有效 token：优先内存/文件缓存，过期则自动刷新。
// 没有任何 token 时返回错误（需先 ExchangeToken）。
func (m *Manager) GetToken(ctx context.Context) (*Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.token == nil {
		// 从文件加载。
		t, err := loadToken(m.cfg.TokenFile)
		if err != nil {
			return nil, fmt.Errorf("无可用 token，请先调用 ExchangeToken 完成授权: %w", err)
		}
		m.token = t
	}

	if !m.token.IsExpired() {
		return m.token, nil
	}

	// 过期：用 refresh_token 刷新。
	if m.token.RefreshToken == "" {
		return nil, fmt.Errorf("token 已过期且无 refresh_token，请重新授权")
	}
	t, err := m.requestToken(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {m.token.RefreshToken},
		"client_id":     {m.cfg.AppKey},
		"client_secret": {m.cfg.SecretKey},
	})
	if err != nil {
		return nil, fmt.Errorf("刷新 token 失败（refresh_token 可能已失效，需重新授权）: %w", err)
	}
	m.token = t
	if err := m.save(); err != nil {
		return t, fmt.Errorf("token 刷新成功但持久化失败: %w", err)
	}
	return t, nil
}

// save 持久化 token 到文件。
func (m *Manager) save() error {
	if m.cfg.TokenFile == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(m.cfg.TokenFile), 0o700); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(m.token, "", "  ")
	return os.WriteFile(m.cfg.TokenFile, data, 0o600)
}

// requestToken 请求 token 端点（GET + query）。
func (m *Manager) requestToken(ctx context.Context, form url.Values) (*Token, error) {
	reqURL := tokenURL + "?" + form.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("解析 token 响应失败: %w", err)
	}
	if raw.Error != "" || raw.AccessToken == "" {
		return nil, fmt.Errorf("获取 token 失败: %s %s", raw.Error, raw.ErrorDesc)
	}
	return &Token{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		ExpiresIn:    raw.ExpiresIn,
		ExpiresAt:    time.Now().Unix() + raw.ExpiresIn,
		Scope:        raw.Scope,
	}, nil
}

// loadToken 从文件加载 token。
func loadToken(path string) (*Token, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t Token
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	if t.AccessToken == "" {
		return nil, fmt.Errorf("token 文件无 access_token")
	}
	return &t, nil
}

func defaultTokenPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".panbaidu", "token.json")
}
