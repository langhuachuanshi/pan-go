// Package folder 文件夹操作：列表 / 创建 / 删除 / 移动。
package folder

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/langhuachuanshi/pan-go/lanzou/invoker"
)

// FolderInfo 文件夹信息
type FolderInfo struct {
	FolID      string `json:"fol_id"`     // 文件夹ID
	Name       string `json:"name"`       // 名称
	FolderDesc string `json:"folder_des"` // 描述
	Onof       string `json:"onof"`       // 密码开关
	IsLock     string `json:"is_lock"`    // 锁定
}

// FolderList 文件夹列表响应。
// Info 用 RawMessage：成功时实测为 []（数组），失败时是错误文本（string）——
// 定型 string 会在成功场景反序列化失败（2026-09-12 实测 task=47 返回 "info":[]）。
type FolderList struct {
	Zt   int             `json:"zt"`
	Info json.RawMessage `json:"info"`
	Text []*FolderInfo   `json:"text"`
}

// Service 文件夹服务。
type Service struct{ inv invoker.Invoker }

// New 创建 folder Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// List 获取子文件夹列表。
// fid: 父文件夹ID，根目录传 -1（蓝奏云根目录的 folder_id 是 -1，不是 0）。
func (s *Service) List(ctx context.Context, fid int) (*FolderList, error) {
	if !s.inv.LoggedIn() {
		return nil, invoker.ErrNotLoggedIn
	}
	var resp FolderList
	err := s.inv.PostForm(ctx, invoker.PathTaskAPI, map[string]string{
		"task":      "47",
		"folder_id": fmt.Sprintf("%d", fid),
		"vei":       s.inv.Vei(),
	}, &resp)
	if err != nil {
		return nil, fmt.Errorf("get dir list failed: %w", err)
	}
	// zt=1 正常, zt=2 也是成功（空列表时蓝奏云返回 zt=2）
	if resp.Zt != 1 && resp.Zt != 2 {
		return nil, fmt.Errorf("%w: zt=%d info=%s", invoker.ErrAPIError, resp.Zt, resp.Info)
	}
	return &resp, nil
}

// Create 创建文件夹。
// name: 文件夹名称, parentID: 父文件夹ID（根目录传 -1）。
func (s *Service) Create(ctx context.Context, name string, parentID int) (*FolderInfo, error) {
	if !s.inv.LoggedIn() {
		return nil, invoker.ErrNotLoggedIn
	}
	var resp struct {
		Zt   int    `json:"zt"`
		Info string `json:"info"`
		Text string `json:"text"` // 直接返回文件夹ID字符串
	}
	err := s.inv.PostForm(ctx, invoker.PathTaskAPI, map[string]string{
		"task":               "2",
		"parent_id":          fmt.Sprintf("%d", parentID),
		"folder_name":        name,
		"folder_description": "",
		"vei":                s.inv.Vei(),
	}, &resp)
	if err != nil {
		return nil, fmt.Errorf("create folder failed: %w", err)
	}
	if resp.Zt != 1 {
		return nil, fmt.Errorf("%w: %s", invoker.ErrAPIError, resp.Info)
	}
	return &FolderInfo{FolID: resp.Text, Name: name}, nil
}

// Delete 删除文件夹。
// fids: 文件夹ID列表。
func (s *Service) Delete(ctx context.Context, fids []string) error {
	if !s.inv.LoggedIn() {
		return invoker.ErrNotLoggedIn
	}
	var resp struct {
		Zt   int    `json:"zt"`
		Info string `json:"info"`
	}
	err := s.inv.PostForm(ctx, invoker.PathTaskAPI, map[string]string{
		"task":      "3",
		"folder_id": strings.Join(fids, "-"),
		"vei":       s.inv.Vei(),
	}, &resp)
	if err != nil {
		return fmt.Errorf("delete folder failed: %w", err)
	}
	if resp.Zt == 0 {
		return fmt.Errorf("%w: %s", invoker.ErrAPIError, resp.Info)
	}
	return nil
}

// Move 移动文件夹到指定位置。
// fids: 文件夹ID列表, fid: 目标文件夹ID。
func (s *Service) Move(ctx context.Context, fids []string, fid int) error {
	if !s.inv.LoggedIn() {
		return invoker.ErrNotLoggedIn
	}
	var resp struct {
		Zt   int    `json:"zt"`
		Info string `json:"info"`
	}
	err := s.inv.PostForm(ctx, invoker.PathTaskAPI, map[string]string{
		"task":       "24",
		"folder_id":  fmt.Sprintf("%d", fid),
		"folder_ids": strings.Join(fids, "-"),
	}, &resp)
	if err != nil {
		return fmt.Errorf("move folder failed: %w", err)
	}
	if resp.Zt == 0 {
		return fmt.Errorf("%w: %s", invoker.ErrAPIError, resp.Info)
	}
	return nil
}
