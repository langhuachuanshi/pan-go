package share

import (
	"context"

	"github.com/langhuachuanshi/alipan-go/alipan/invoker"
	"github.com/langhuachuanshi/alipan-go/alipan/types"
)

// 本文件实现访问/保存他人分享的功能，对应该 aligo share_file_saveto_drive / get_share_file_list 等。
//
// 关键约定（来自 aligo 源码）：
//   - 访问他人分享内容（get_share_token / 列文件 / 搜索）是匿名请求，不带 access_token，
//     但带 x-share-token header。
//   - 保存到自己网盘（copy）同时带 access_token 和 x-share-token。
//   - 保存成功的状态码是 201 或 202（不是 200）。

// GetShareToken 用 share_id + share_pwd 换取 share_token。
// 匿名请求。后续访问该分享内容的接口需把返回的 Token 放到 x-share-token header。
func (s *Service) GetShareToken(ctx context.Context, shareID, sharePwd string) (*types.ShareToken, error) {
	body := map[string]any{
		"share_id":  shareID,
		"share_pwd": sharePwd,
	}
	var resp types.ShareToken
	if err := invoker.PostAndDecodeAnon(ctx, s.inv, pathShareGetToken, body, &resp, []int{200}); err != nil {
		return nil, err
	}
	// 回填 share_id / share_pwd 方便后续复用。
	resp.ShareID = shareID
	resp.SharePwd = sharePwd
	return &resp, nil
}

// ShareFileListRequest 列出分享内文件的请求。
type ShareFileListRequest struct {
	ShareToken   *types.ShareToken
	ParentFileID string            // 默认 root
	OrderBy      string            // name / updated_at
	OrderDirection types.OrderDirection
	Limit        int
	Marker       string
	Category     types.FileCategory
	Type         types.FileType
	Starred      *bool
}

type shareFileListResponse struct {
	Items             []*types.BaseShareFile `json:"items"`
	NextMarker        string                 `json:"next_marker"`
	PunishedFileCount int                    `json:"punished_file_count"`
}

// ListFiles 列出分享内的文件，自动分页。匿名 + x-share-token。
func (s *Service) ListFiles(ctx context.Context, req *ShareFileListRequest) ([]*types.BaseShareFile, error) {
	if req == nil || req.ShareToken == nil {
		return nil, invoker.NewAPIError(0, "InvalidArgument", "share_token is required")
	}
	var all []*types.BaseShareFile
	for {
		page, next, err := s.listFilesPage(ctx, req)
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

func (s *Service) listFilesPage(ctx context.Context, req *ShareFileListRequest) ([]*types.BaseShareFile, string, error) {
	headers := map[string]string{"x-share-token": req.ShareToken.Token}
	body := map[string]any{
		"share_id":                req.ShareToken.ShareID,
		"parent_file_id":          defaultStr(req.ParentFileID, "root"),
		"order_by":                defaultStr(req.OrderBy, "name"),
		"order_direction":         defaultStr(string(req.OrderDirection), "DESC"),
		"limit":                   defaultInt(req.Limit, 200),
		"url_expire_sec":          14400,
		"image_thumbnail_process": "image/resize,w_400/format,jpeg",
		"image_url_process":       "image/resize,w_1920/format,jpeg",
		"video_thumbnail_process": "video/snapshot,t_0,f_jpg,ar_auto,w_800",
	}
	if req.Marker != "" {
		body["marker"] = req.Marker
	}
	if req.Category != "" {
		body["category"] = req.Category
	}
	if req.Type != "" {
		body["type"] = req.Type
	}
	if req.Starred != nil {
		body["starred"] = *req.Starred
	}
	var resp shareFileListResponse
	if err := invoker.PostAndDecodeAnonWithHeaders(ctx, s.inv, pathShareFileList, body, &resp, headers, []int{200}); err != nil {
		return nil, "", err
	}
	return resp.Items, resp.NextMarker, nil
}

// SaveToDriveRequest 保存分享文件到自己网盘的请求。
type SaveToDriveRequest struct {
	ShareToken     *types.ShareToken
	FileID         string // 分享内的文件 id
	ToParentFileID string // 目标父目录，默认 root
	NewName        string
	AutoRename     bool
	Overwrite      bool
	ToDriveID      string
}

// SaveToDriveResponse 保存响应。
type SaveToDriveResponse struct {
	FileID      string `json:"file_id"`
	DriveID     string `json:"drive_id"`
	DomainID    string `json:"domain_id"`
	AsyncTaskID string `json:"async_task_id"`
}

// SaveToDrive 保存单个分享文件到自己网盘。
// 同时带 access_token 和 x-share-token，期望 201/202。
func (s *Service) SaveToDrive(ctx context.Context, req *SaveToDriveRequest) (*SaveToDriveResponse, error) {
	if req == nil || req.ShareToken == nil || req.FileID == "" {
		return nil, invoker.NewAPIError(0, "InvalidArgument", "share_token and file_id are required")
	}
	headers := map[string]string{"x-share-token": req.ShareToken.Token}
	body := map[string]any{
		"share_id":          req.ShareToken.ShareID,
		"file_id":           req.FileID,
		"to_parent_file_id": defaultStr(req.ToParentFileID, "root"),
		"auto_rename":       req.AutoRename, // aligo 默认 true，调用方按需传
		"overwrite":         req.Overwrite,
	}
	if req.NewName != "" {
		body["new_name"] = req.NewName
	}
	if req.ToDriveID != "" {
		body["to_drive_id"] = req.ToDriveID
	} else if d := s.inv.DefaultDriveID(); d != "" {
		body["to_drive_id"] = d
	}
	var resp SaveToDriveResponse
	if err := invoker.PostAndDecodeWithHeaders(ctx, s.inv, pathFileCopy, body, &resp, headers, []int{201, 202}); err != nil {
		return nil, err
	}
	return &resp, nil
}

// BatchItem 批量保存响应单项。
type BatchItem struct {
	ID     string          `json:"id"`
	Status int             `json:"status"`
	Body   map[string]any  `json:"body"`
}

// SaveToDriveBatch 批量保存分享文件到自己网盘。
// 走 /adrive/v2/batch，resource=file，子 url=/file/copy，带 x-share-token，100 一组。
func (s *Service) SaveToDriveBatch(ctx context.Context, shareToken *types.ShareToken, fileIDs []string, toParentFileID, toDriveID string) ([]BatchItem, error) {
	if shareToken == nil {
		return nil, invoker.NewAPIError(0, "InvalidArgument", "share_token is required")
	}
	if toParentFileID == "" {
		toParentFileID = "root"
	}
	if toDriveID == "" {
		toDriveID = s.inv.DefaultDriveID()
	}

	subs := make([]map[string]any, 0, len(fileIDs))
	for _, fid := range fileIDs {
		subs = append(subs, map[string]any{
			"body": map[string]any{
				"file_id":           fid,
				"share_id":          shareToken.ShareID,
				"to_parent_file_id": toParentFileID,
				"to_drive_id":       toDriveID,
				"auto_rename":       true,
			},
			"headers": map[string]string{"Content-Type": "application/json"},
			"id":      fid,
			"method":  "POST",
			"url":     subURLFileCopy,
		})
	}

	headers := map[string]string{"x-share-token": shareToken.Token}
	var allResults []BatchItem
	for start := 0; start < len(subs); start += batchMaxPerRequest {
		end := start + batchMaxPerRequest
		if end > len(subs) {
			end = len(subs)
		}
		payload := map[string]any{"requests": subs[start:end], "resource": "file"}
		var resp struct {
			Responses []BatchItem `json:"responses"`
		}
		if err := invoker.PostAndDecodeWithHeaders(ctx, s.inv, pathBatchV2, payload, &resp, headers, []int{200}); err != nil {
			return nil, err
		}
		allResults = append(allResults, resp.Responses...)
	}
	return allResults, nil
}

// SearchInShareRequest 分享内搜索请求。
type SearchInShareRequest struct {
	ShareToken *types.ShareToken
	Query      string // 搜索表达式
	Limit      int
	Marker     string
}

// SearchInShare 在他人分享内搜索文件。匿名 + x-share-token。
func (s *Service) SearchInShare(ctx context.Context, req *SearchInShareRequest) ([]*types.BaseShareFile, error) {
	if req == nil || req.ShareToken == nil || req.Query == "" {
		return nil, invoker.NewAPIError(0, "InvalidArgument", "share_token and query are required")
	}
	headers := map[string]string{"x-share-token": req.ShareToken.Token}
	var all []*types.BaseShareFile
	for {
		body := map[string]any{
			"share_id": req.ShareToken.ShareID,
			"query":    req.Query,
			"limit":    defaultInt(req.Limit, 100),
		}
		if req.Marker != "" {
			body["marker"] = req.Marker
		}
		var resp shareFileListResponse
		if err := invoker.PostAndDecodeAnonWithHeaders(ctx, s.inv, pathShareSearch, body, &resp, headers, []int{200}); err != nil {
			return nil, err
		}
		all = append(all, convertShareFiles(resp.Items)...)
		if resp.NextMarker == "" {
			break
		}
		req.Marker = resp.NextMarker
	}
	return all, nil
}

// convertShareFiles 类型转换辅助（BaseShareFile → BaseShareFile，预留扩展）。
func convertShareFiles(in []*types.BaseShareFile) []*types.BaseShareFile { return in }
