// Package file 实现夸克网盘文件相关操作。
//
// 夸克文件列表接口：GET /file/sort
// 参数：pdir_fid（父目录，根目录"0"）、_page（页码从1开始）、_size（每页数，默认50）、
//
//	_fetch_total（是否返回总数，1）、_sort（排序，如 "file_type:asc,updated_at:desc"）
package file

import (
	"context"
	"fmt"
	"time"

	"github.com/langhuachuanshi/pan-go/quark/invoker"
	"github.com/langhuachuanshi/pan-go/quark/types"
)

// Service 文件相关操作入口。
type Service struct {
	inv invoker.Invoker
}

// New 创建 file Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// ListRequest 文件列表请求参数。
type ListRequest struct {
	PDirFID string // 父目录 fid，根目录用 "0"
	Page    int    // 页码，从 1 开始（0 视为 1）
	Size    int    // 每页数量，默认 50
	Sort    string // 排序，如 "file_type:asc,updated_at:desc"（空用默认）
}

// listResponse 文件列表响应（夸克外层结构：{metadata,data:{list,total}}）。
type listResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
	Data struct {
		List  []*types.File `json:"list"`
		Total int           `json:"total"`
	} `json:"data"`
	Status int `json:"status"`
}

// List 列出指定目录下的文件。自动分页，返回所有。
func (s *Service) List(ctx context.Context, req *ListRequest) ([]*types.File, error) {
	if req == nil {
		req = &ListRequest{}
	}
	pdir := req.PDirFID
	if pdir == "" {
		pdir = "0"
	}
	page := req.Page
	if page == 0 {
		page = 1
	}
	size := req.Size
	if size == 0 {
		size = 50
	}

	var all []*types.File
	for {
		params := map[string]any{
			"pdir_fid":             pdir,
			"_page":                page,
			"_size":                size,
			"_fetch_total":         1,
			"_sort":                req.Sort,
			"fetch_all_file":       1,
			"fetch_risk_file_name": 1,
		}
		var resp listResponse
		if err := invoker.GetAndDecode(ctx, s.inv, "/file/sort", params, &resp); err != nil {
			return nil, err
		}
		if resp.Code != 0 {
			return nil, invoker.NewAPIError(resp.Code, resp.Msg)
		}
		if len(resp.Data.List) == 0 {
			break
		}
		all = append(all, resp.Data.List...)
		// 够了或没下一页。
		if len(resp.Data.List) < size || len(all) >= resp.Data.Total {
			break
		}
		page++
	}
	return all, nil
}

// ListPage 列出单页文件（手动分页）。返回 (文件, 总数)。
func (s *Service) ListPage(ctx context.Context, req *ListRequest) ([]*types.File, int, error) {
	if req == nil {
		req = &ListRequest{}
	}
	pdir := req.PDirFID
	if pdir == "" {
		pdir = "0"
	}
	page := req.Page
	if page == 0 {
		page = 1
	}
	size := req.Size
	if size == 0 {
		size = 50
	}
	params := map[string]any{
		"pdir_fid":             pdir,
		"_page":                page,
		"_size":                size,
		"_fetch_total":         1,
		"_sort":                req.Sort,
		"fetch_all_file":       1,
		"fetch_risk_file_name": 1,
	}
	var resp listResponse
	if err := invoker.GetAndDecode(ctx, s.inv, "/file/sort", params, &resp); err != nil {
		return nil, 0, err
	}
	if resp.Code != 0 {
		return nil, 0, invoker.NewAPIError(resp.Code, resp.Msg)
	}
	return resp.Data.List, resp.Data.Total, nil
}

// Get 获取文件详情。
// 夸克接口：GET /file/info，参数 fid。
func (s *Service) Get(ctx context.Context, fid string) (*types.File, error) {
	params := map[string]any{"fid": fid}
	var resp struct {
		Code int        `json:"code"`
		Msg  string     `json:"message"`
		Data types.File `json:"data"`
	}
	if err := invoker.GetAndDecode(ctx, s.inv, "/file/info", params, &resp); err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, invoker.NewAPIError(resp.Code, resp.Msg)
	}
	f := resp.Data
	return &f, nil
}

// —— 文件管理 ——
//
// 夸克这几个接口都是 POST + JSON body，外层 {code,message,data}，code!=0 即失败。
// Move/Delete 底层就是批量的（filelist 传多个 fid）。

// MakeDir 在 pdirFID 下创建名为 name 的文件夹，返回新文件夹的 fid。
//
// 夸克 /file 接口的响应不含新 fid，所以创建成功后会 List 父目录按名匹配取 fid。
// （夸克建目录有秒级延迟，AList 实现里 MakeDir 后也 sleep 1s。）
// pdirFID 留空或 "0" 表示根目录。
func (s *Service) MakeDir(ctx context.Context, pdirFID, name string) (string, error) {
	if name == "" {
		return "", invoker.NewAPIError(0, "name is required")
	}
	if pdirFID == "" {
		pdirFID = "0"
	}
	body := map[string]any{
		"dir_init_lock": false,
		"dir_path":      "",
		"file_name":     name,
		"pdir_fid":      pdirFID,
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"message"`
	}
	if err := invoker.PostAndDecode(ctx, s.inv, "/file", body, nil, &resp); err != nil {
		return "", err
	}
	if resp.Code != 0 {
		return "", invoker.NewAPIError(resp.Code, resp.Msg)
	}

	// 建目录有秒级延迟，按名查 fid 需重试：实测连续建多级目录时（父目录刚建好
	// 立刻建子目录），1 秒后 List 可能还看不到新目录。每秒查一次，最多 5 次。
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		time.Sleep(time.Second)
		files, err := s.List(ctx, &ListRequest{PDirFID: pdirFID, Size: 200})
		if err != nil {
			lastErr = fmt.Errorf("makdir 成功但查 fid 失败: %w", err)
			continue
		}
		for _, f := range files {
			if f.IsFolder() && f.FileName == name {
				return f.FID, nil
			}
		}
		lastErr = fmt.Errorf("makdir 成功但在父目录未找到名为 %q 的文件夹", name)
	}
	return "", lastErr
}

// Rename 重命名单个文件/文件夹。
// 夸克接口：POST /file/rename，body {fid, file_name}。
func (s *Service) Rename(ctx context.Context, fid, newName string) error {
	if fid == "" || newName == "" {
		return invoker.NewAPIError(0, "fid and newName are required")
	}
	body := map[string]any{"fid": fid, "file_name": newName}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"message"`
	}
	if err := invoker.PostAndDecode(ctx, s.inv, "/file/rename", body, nil, &resp); err != nil {
		return err
	}
	if resp.Code != 0 {
		return invoker.NewAPIError(resp.Code, resp.Msg)
	}
	return nil
}

// Move 把 fids（一个或多个）移动到 toDirFID 目录下。
// 夸克接口：POST /file/move，底层批量。
func (s *Service) Move(ctx context.Context, fids []string, toDirFID string) error {
	if len(fids) == 0 {
		return invoker.NewAPIError(0, "fids is required")
	}
	if toDirFID == "" {
		toDirFID = "0"
	}
	body := map[string]any{
		"action_type":  1,
		"exclude_fids": []string{},
		"filelist":     fids,
		"to_pdir_fid":  toDirFID,
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"message"`
	}
	if err := invoker.PostAndDecode(ctx, s.inv, "/file/move", body, nil, &resp); err != nil {
		return err
	}
	if resp.Code != 0 {
		return invoker.NewAPIError(resp.Code, resp.Msg)
	}
	return nil
}

// Delete 删除 fids（一个或多个）。
// 夸克接口：POST /file/delete，底层批量。删除进回收站。
func (s *Service) Delete(ctx context.Context, fids []string) error {
	if len(fids) == 0 {
		return invoker.NewAPIError(0, "fids is required")
	}
	body := map[string]any{
		"action_type":  1,
		"exclude_fids": []string{},
		"filelist":     fids,
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"message"`
	}
	if err := invoker.PostAndDecode(ctx, s.inv, "/file/delete", body, nil, &resp); err != nil {
		return err
	}
	if resp.Code != 0 {
		return invoker.NewAPIError(resp.Code, resp.Msg)
	}
	return nil
}
