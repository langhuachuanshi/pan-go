// Package file 实现文件相关操作，对应该 aligo 的 apis+core/File.py、Copy.py、Move.py、
// Update.py、Search.py、Create.py（上传）、Download.py。
//
// 依赖 invoker.Invoker 接口（由主包 Client 实现），不依赖主包本身，避免循环依赖。
package file

import (
	"context"

	"github.com/langhuachuanshi/alipan-go/alipan/invoker"
	"github.com/langhuachuanshi/alipan-go/alipan/types"
)

// Service 文件相关操作的入口。
type Service struct {
	inv invoker.Invoker
}

// New 创建 file Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// ListRequest 列文件请求参数。
type ListRequest struct {
	ParentFileID   string            `json:"parent_file_id"`
	DriveID        string            `json:"drive_id,omitempty"`
	Starred        *bool             `json:"starred,omitempty"`
	All            bool              `json:"all,omitempty"`
	Category       types.FileCategory `json:"category,omitempty"`
	Fields         string            `json:"fields,omitempty"`
	Limit          int               `json:"limit,omitempty"`
	Marker         string            `json:"marker,omitempty"`
	OrderBy        types.ListOrderBy `json:"order_by,omitempty"`
	OrderDirection types.OrderDirection `json:"order_direction,omitempty"`
	Type           types.FileType    `json:"type,omitempty"`
}

// URL path 常量（与主包 config 对应，这里就近定义避免跨包）。
const (
	pathFileList           = "/adrive/v3/file/list"
	pathFileGet            = "/v2/file/get"
	pathFileSearch         = "/v2/file/search"
	pathFileCopy           = "/v2/file/copy"
	pathFileMove           = "/v2/file/move"
	pathFileUpdate         = "/v3/file/update"
	pathFileCreateWithFolders = "/adrive/v2/file/createWithFolders"
	pathFileGetUploadURL   = "/v2/file/get_upload_url"
	pathFileComplete       = "/v2/file/complete"
	pathFilePath           = "/adrive/v1/file/get_path"
	pathFileGetDownloadURL = "/v2/file/get_download_url"
	pathRecyclebinTrash    = "/v2/recyclebin/trash"
	pathRecyclebinRestore  = "/v2/recyclebin/restore"
	pathRecyclebinList     = "/v2/recyclebin/list"
	pathBatch              = "/v3/batch"
	pathBatchV2            = "/adrive/v2/batch"

	subURLFileCopy          = "/file/copy"
	subURLFileMove          = "/file/move"
	subURLFileUpdate        = "/file/update"
	subURLFileGet           = "/file/get"
	subURLRecyclebinTrash   = "/recyclebin/trash"
	subURLRecyclebinRestore = "/recyclebin/restore"
)

const batchMaxPerRequest = 100

type fileListResponse struct {
	Items             []*types.BaseFile `json:"items"`
	NextMarker        string            `json:"next_marker"`
	PunishedFileCount int               `json:"punished_file_count"`
}

// List 列出指定目录下的文件，自动分页。
func (s *Service) List(ctx context.Context, req *ListRequest) ([]*types.BaseFile, error) {
	if req == nil {
		req = &ListRequest{}
	}
	var all []*types.BaseFile
	for {
		page, next, err := s.ListPage(ctx, req)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if next == "" {
			break
		}
		req.Marker = next
	}
	return all, nil
}

// ListPage 列出单页文件。
func (s *Service) ListPage(ctx context.Context, req *ListRequest) ([]*types.BaseFile, string, error) {
	if req == nil {
		req = &ListRequest{}
	}
	body := map[string]any{
		"parent_file_id":          defaultStr(req.ParentFileID, "root"),
		"drive_id":                req.DriveID,
		"fields":                  defaultStr(req.Fields, "*"),
		"limit":                   defaultInt(req.Limit, 200),
		"order_by":                defaultStr(string(req.OrderBy), "updated_at"),
		"order_direction":         defaultStr(string(req.OrderDirection), "DESC"),
		"image_thumbnail_process": "image/resize,w_400/format,jpeg",
		"image_url_process":       "image/resize,w_1920/format,jpeg",
		"url_expire_sec":          14400,
		"video_thumbnail_process": "video/snapshot,t_0,f_jpg,ar_auto,w_800",
	}
	if req.Marker != "" {
		body["marker"] = req.Marker
	}
	if req.Starred != nil {
		body["starred"] = *req.Starred
	}
	if req.All {
		body["all"] = true
	}
	if req.Category != "" {
		body["category"] = req.Category
	}
	if req.Type != "" {
		body["type"] = req.Type
	}
	var resp fileListResponse
	if err := invoker.PostAndDecode(ctx, s.inv, pathFileList, body, &resp, []int{200}); err != nil {
		return nil, "", err
	}
	return resp.Items, resp.NextMarker, nil
}

// Get 获取单个文件详情。
func (s *Service) Get(ctx context.Context, fileID, driveID string) (*types.BaseFile, error) {
	body := map[string]any{"file_id": fileID}
	if driveID != "" {
		body["drive_id"] = driveID
	}
	var f types.BaseFile
	if err := invoker.PostAndDecode(ctx, s.inv, pathFileGet, body, &f, []int{200}); err != nil {
		return nil, err
	}
	return &f, nil
}

// GetPath 获取文件完整路径。
func (s *Service) GetPath(ctx context.Context, fileID, driveID string) ([]*types.FilePathItem, error) {
	body := map[string]any{"file_id": fileID}
	if driveID != "" {
		body["drive_id"] = driveID
	}
	var resp struct {
		Items []*types.FilePathItem `json:"items"`
	}
	if err := invoker.PostAndDecode(ctx, s.inv, pathFilePath, body, &resp, []int{200}); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func defaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
