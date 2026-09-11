// Package auth 实现夸克网盘的 cookie 管理。
//
// 夸克网盘基于 cookie 鉴权，无 token、无签名。
// cookie 来源：扫码登录（quark/qrcode 换发）或从浏览器（pan.quark.cn 登录后
// F12）复制完整 cookie 字符串。
// 最关键的 cookie 字段是 __puus/__pus（登录态凭证，扫码换发的只有 __pus）。
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config auth 配置。
type Config struct {
	Cookie    string // 完整 cookie 字符串（浏览器复制）
	CookieFile string // cookie 文件路径（二选一）
}

// Result 登录产物。
type Result struct {
	Cookie string // 完整 cookie
}

// Load 加载 cookie：优先 Cookie，其次 CookieFile，再查默认 ~/.quark/cookie.json。
func Load(cfg *Config) (*Result, error) {
	// 1. 直接传 cookie。
	if cfg.Cookie != "" {
		return &Result{Cookie: cfg.Cookie}, nil
	}
	// 2. 指定 cookie 文件。
	if cfg.CookieFile != "" {
		c, err := loadCookieFile(cfg.CookieFile)
		if err != nil {
			return nil, err
		}
		return &Result{Cookie: c}, nil
	}
	// 3. 默认 ~/.quark/cookie.json。
	c, err := loadCookieFile(defaultCookiePath())
	if err != nil {
		return nil, fmt.Errorf("quark: 未提供 cookie，且默认位置 %s 无可用 cookie: %w", defaultCookiePath(), err)
	}
	return &Result{Cookie: c}, nil
}

// SaveCookie 把 cookie 保存到默认文件。
func SaveCookie(cookie string) error {
	dir := defaultDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(map[string]string{"cookie": cookie}, "", "  ")
	return os.WriteFile(filepath.Join(dir, "cookie.json"), data, 0o600)
}

// DeleteCookieFile 删除 cookie 文件。
func DeleteCookieFile() error {
	p := defaultCookiePath()
	err := os.Remove(p)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func defaultDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".quark")
}

func defaultCookiePath() string {
	return filepath.Join(defaultDir(), "cookie.json")
}

func loadCookieFile(p string) (string, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	// 支持 {"cookie":"..."} 或纯 cookie 文本。
	var m map[string]string
	if json.Unmarshal(data, &m) == nil {
		if c, ok := m["cookie"]; ok && c != "" {
			return c, nil
		}
	}
	// 当作纯文本。
	c := strings.TrimSpace(string(data))
	if c == "" {
		return "", errors.New("quark: empty cookie file")
	}
	return c, nil
}

// IsValid 粗略检查 cookie 是否含必要字段。
// 实测（2026-09-12 扫码登录）：ST 换发的 cookie 只带 __pus 不带 __puus，
// drive 接口同样认（200）；网页登录则两者都有。任一存在即视为有效。
func IsValid(cookie string) bool {
	return strings.Contains(cookie, "__puus=") || strings.Contains(cookie, "__pus=")
}
