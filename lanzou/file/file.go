// Package file 文件操作：列表 / 分享链接 / 移动 / 删除 / 设密码。
package file

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/langhuachuanshi/pan-go/lanzou/invoker"
)

// FileInfo 文件列表中的文件信息
type FileInfo struct {
	ID      string `json:"id"`       // 文件ID
	NameAll string `json:"name_all"` // 文件名
	Size    string `json:"size"`     // 大小
	Time    string `json:"time"`     // 上传时间
	Icon    string `json:"icon"`     // 图标类型
	Downs   string `json:"downs"`    // 下载次数
	Onof    string `json:"onof"`     // 密码开关
	IsNewd  string `json:"is_newd"`  // 分享URL前缀
	FID     string `json:"f_id"`     // 分享ID
}

// FileList 文件列表响应
type FileList struct {
	Zt   int             `json:"zt"`
	Info json.RawMessage `json:"info"`
	Text []*FileInfo     `json:"text"`
}

// Service 文件服务。
type Service struct{ inv invoker.Invoker }

// New 创建 file Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// List 获取文件夹内的文件列表（自动翻页）。
// fid: 文件夹ID，根目录传 -1（蓝奏云根目录的 folder_id 是 -1）。
func (s *Service) List(ctx context.Context, fid int) (*FileList, error) {
	if !s.inv.LoggedIn() {
		return nil, invoker.ErrNotLoggedIn
	}
	result := &FileList{Zt: 1}
	for pg := 1; ; pg++ {
		var resp FileList
		err := s.inv.PostForm(ctx, invoker.PathTaskAPI, map[string]string{
			"task":      "5",
			"folder_id": fmt.Sprintf("%d", fid),
			"pg":        fmt.Sprintf("%d", pg),
			"vei":       s.inv.Vei(),
		}, &resp)
		if err != nil {
			return nil, fmt.Errorf("get file list failed: %w", err)
		}
		// zt=1 正常, zt=2 也是成功（空列表时蓝奏云返回 zt=2）
		if resp.Zt != 1 && resp.Zt != 2 {
			return nil, fmt.Errorf("%w: zt=%d", invoker.ErrAPIError, resp.Zt)
		}
		result.Text = append(result.Text, resp.Text...)
		if len(resp.Text) == 0 {
			break
		}
	}
	return result, nil
}

// ShareURL 获取文件的分享链接（task=22）。
// fileID: 文件 ID（List 返回的 FileInfo.ID）。
func (s *Service) ShareURL(ctx context.Context, fileID string) (*FileShareInfo, error) {
	if !s.inv.LoggedIn() {
		return nil, invoker.ErrNotLoggedIn
	}
	var resp struct {
		Zt   int           `json:"zt"`
		Info FileShareInfo `json:"info"`
	}
	err := s.inv.PostForm(ctx, invoker.PathTaskAPI, map[string]string{
		"task":    "22",
		"file_id": fileID,
		"vei":     s.inv.Vei(),
	}, &resp)
	if err != nil {
		return nil, fmt.Errorf("get share url failed: %w", err)
	}
	if resp.Zt != 1 {
		return nil, fmt.Errorf("%w: get share url failed zt=%d", invoker.ErrAPIError, resp.Zt)
	}
	return &resp.Info, nil
}

// Move 移动文件到指定文件夹。
// fids: 文件ID列表，fid: 目标文件夹ID。
func (s *Service) Move(ctx context.Context, fids []string, fid int) error {
	if !s.inv.LoggedIn() {
		return invoker.ErrNotLoggedIn
	}
	var resp struct {
		Zt   int    `json:"zt"`
		Info string `json:"info"`
	}
	err := s.inv.PostForm(ctx, invoker.PathTaskAPI, map[string]string{
		"task":      "17",
		"folder_id": fmt.Sprintf("%d", fid),
		"file_id":   strings.Join(fids, "-"),
		"vei":       s.inv.Vei(),
	}, &resp)
	if err != nil {
		return fmt.Errorf("move files failed: %w", err)
	}
	if resp.Zt == 0 {
		return fmt.Errorf("%w: %s", invoker.ErrAPIError, resp.Info)
	}
	return nil
}

// Delete 删除文件。
// fids: 文件ID列表。
func (s *Service) Delete(ctx context.Context, fids []string) error {
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
		"vei":     s.inv.Vei(),
	}, &resp)
	if err != nil {
		return fmt.Errorf("delete files failed: %w", err)
	}
	if resp.Zt == 0 {
		return fmt.Errorf("%w: %s", invoker.ErrAPIError, resp.Info)
	}
	return nil
}

// SetPassword 设置文件或文件夹密码。
// entityID: 文件或文件夹ID。
func (s *Service) SetPassword(ctx context.Context, entityID int, pwd string) error {
	if !s.inv.LoggedIn() {
		return invoker.ErrNotLoggedIn
	}
	var resp struct {
		Zt   int    `json:"zt"`
		Info string `json:"info"`
	}
	err := s.inv.PostForm(ctx, invoker.PathTaskAPI, map[string]string{
		"task":      "5",
		"file_id":   fmt.Sprintf("%d", entityID),
		"shows":     "2",
		"shownames": pwd,
	}, &resp)
	if err != nil {
		return fmt.Errorf("set password failed: %w", err)
	}
	if resp.Zt == 0 {
		return fmt.Errorf("%w: %s", invoker.ErrAPIError, resp.Info)
	}
	return nil
}

// FileShareInfo 文件分享信息（task=22 返回）
type FileShareInfo struct {
	IsNewd string `json:"is_newd"` // 分享 URL 前缀，如 https://wwa.lanzoui.com
	FID    string `json:"f_id"`    // 分享 ID，拼接在 is_newd 后面
	Pwd    string `json:"pwd"`     // 提取密码（"" 表示无密码）
	Onof   string `json:"onof"`    // 是否有密码 "1"=有 "2"=无
}
