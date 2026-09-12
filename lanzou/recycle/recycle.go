// Package recycle 回收站：列表 / 移入 / 恢复 / 清空。
package recycle

import (
	"context"
	"fmt"
	"strings"

	"github.com/langhuachuanshi/pan-go/lanzou/invoker"
)

// RecycleList 回收站列表响应
type RecycleList struct {
	Zt   int             `json:"zt"`
	Info string          `json:"info"`
	Text []*RecycledFile `json:"text"`
}

// RecycledFile 回收站中的文件
type RecycledFile struct {
	FileID     string `json:"id"`
	FileName   string `json:"name_all"`
	FilePath   string `json:"path"`
	UploadTime string `json:"time"`
}

// Service 回收站服务。
type Service struct{ inv invoker.Invoker }

// New 创建 recycle Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// List 获取回收站文件列表。
// page: 页码，从1开始。
func (s *Service) List(ctx context.Context, page int) (*RecycleList, error) {
	if !s.inv.LoggedIn() {
		return nil, invoker.ErrNotLoggedIn
	}
	var resp RecycleList
	err := s.inv.PostForm(ctx, invoker.PathTaskAPI, map[string]string{
		"task": "7",
		"pg":   fmt.Sprintf("%d", page),
	}, &resp)
	if err != nil {
		return nil, fmt.Errorf("get recycle list failed: %w", err)
	}
	if resp.Zt == 0 {
		return nil, fmt.Errorf("%w: %s", invoker.ErrAPIError, resp.Info)
	}
	return &resp, nil
}

// MoveToTrash 将文件移入回收站。
// fids: 文件ID列表。
func (s *Service) MoveToTrash(ctx context.Context, fids []string) error {
	if !s.inv.LoggedIn() {
		return invoker.ErrNotLoggedIn
	}
	var resp struct {
		Zt   int    `json:"zt"`
		Info string `json:"info"`
	}
	err := s.inv.PostForm(ctx, invoker.PathTaskAPI, map[string]string{
		"task":    "6",
		"file_id": strings.Join(fids, "-"),
	}, &resp)
	if err != nil {
		return fmt.Errorf("move to trash failed: %w", err)
	}
	if resp.Zt == 0 {
		return fmt.Errorf("%w: %s", invoker.ErrAPIError, resp.Info)
	}
	return nil
}

// RestoreFiles 从回收站恢复文件。
// fids: 文件ID列表。
func (s *Service) RestoreFiles(ctx context.Context, fids []string) error {
	if !s.inv.LoggedIn() {
		return invoker.ErrNotLoggedIn
	}
	var resp struct {
		Zt   int    `json:"zt"`
		Info string `json:"info"`
	}
	err := s.inv.PostForm(ctx, invoker.PathTaskAPI, map[string]string{
		"task":    "8",
		"file_id": strings.Join(fids, "-"),
	}, &resp)
	if err != nil {
		return fmt.Errorf("restore files failed: %w", err)
	}
	if resp.Zt == 0 {
		return fmt.Errorf("%w: %s", invoker.ErrAPIError, resp.Info)
	}
	return nil
}

// CleanRecycle 清空回收站。
func (s *Service) CleanRecycle(ctx context.Context) error {
	if !s.inv.LoggedIn() {
		return invoker.ErrNotLoggedIn
	}
	var resp struct {
		Zt   int    `json:"zt"`
		Info string `json:"info"`
	}
	err := s.inv.PostForm(ctx, invoker.PathTaskAPI, map[string]string{"task": "9"}, &resp)
	if err != nil {
		return fmt.Errorf("clean recycle failed: %w", err)
	}
	if resp.Zt == 0 {
		return fmt.Errorf("%w: %s", invoker.ErrAPIError, resp.Info)
	}
	return nil
}
