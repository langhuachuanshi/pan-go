// Package account 账号信息服务：用户信息 / 帐号详情。
// 登录登出属会话生命周期，在主包 Client 上（Login / Logout）。
package account

import (
	"context"
	"fmt"
	"regexp"

	"github.com/langhuachuanshi/pan-go/lanzou/invoker"
)

// 个人中心页面（容量/用户名来源）
const profileURL = "https://pc.woozooo.com/mydisk.php?item=profile&action=mypower"

// UserInfo 用户信息
type UserInfo struct {
	UserID   int    `json:"user_id"`
	UserName string `json:"nickname"`
	VIP      int    `json:"vip"`
}

// AccountInfo 帐号详细信息（含容量）
type AccountInfo struct {
	UserInfo
	TotalSize string `json:"total_size"`
	UsedSize  string `json:"used_size"`
}

// Service 账号信息服务。
type Service struct{ inv invoker.Invoker }

// New 创建 account Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// Info 返回用户信息（取自会话 uid）。
func (s *Service) Info(ctx context.Context) (*UserInfo, error) {
	if !s.inv.LoggedIn() {
		return nil, invoker.ErrNotLoggedIn
	}
	return &UserInfo{UserName: s.inv.UserID()}, nil
}

// Detail 返回帐号详细信息（从个人中心页面提取用户名与容量）。
func (s *Service) Detail(ctx context.Context) (*AccountInfo, error) {
	if !s.inv.LoggedIn() {
		return nil, invoker.ErrNotLoggedIn
	}
	html, err := s.inv.FetchPageWithChallenge(profileURL)
	if err != nil {
		return nil, fmt.Errorf("get account info failed: %w", err)
	}

	// 从页面提取用户名
	reName := regexp.MustCompile(`(\d{11,})`)
	info := &AccountInfo{UserInfo: UserInfo{UserName: s.inv.UserID()}}
	if m := reName.FindStringSubmatch(html); len(m) > 1 {
		info.UserName = m[1]
	}

	// 提取容量信息
	reSize := regexp.MustCompile(`(\d+\.?\d*)\s*(GB|MB|KB|TB)`)
	matches := reSize.FindAllStringSubmatch(html, 2)
	if len(matches) >= 2 {
		info.TotalSize = matches[0][0]
		info.UsedSize = matches[1][0]
	} else if len(matches) >= 1 {
		info.TotalSize = matches[0][0]
	}

	return info, nil
}
