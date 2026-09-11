// Package share 实现阿里云盘分享功能，对应该 aligo 的 apis+core/Share.py、CustomShare.py。
//
// 分两部分：
//   1. 管理自己的分享（创建/取消/更新/列出）—— 需登录，带 access_token。
//   2. 访问/保存他人分享 —— 需 x-share-token（通过 share_id+share_pwd 换取，匿名）。
//   3. CustomShare —— aligo 自定义的 aligo:// 协议分享（与官方 API 无关）。
package share

import (
	"context"
	"encoding/json"

	"github.com/langhuachuanshi/alipan-go/alipan/invoker"
	"github.com/langhuachuanshi/alipan-go/alipan/types"
)

// Service 分享相关操作入口。
type Service struct {
	inv invoker.Invoker
}

// New 创建 share Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// —— URL 常量 ——

const (
	pathShareLinkCreate        = "/adrive/v2/share_link/create"
	pathShareLinkUpdate        = "/v2/share_link/update"
	pathShareLinkCancel        = "/adrive/v2/share_link/cancel"
	pathShareLinkList          = "/adrive/v3/share_link/list"
	pathShareGetByAnonymous    = "/adrive/v2/share_link/get_share_by_anonymous"
	pathShareGetToken          = "/v2/share_link/get_share_token"
	pathShareFileList          = "/adrive/v3/file/list" // 复用文件列表接口，带 x-share-token
	pathFileCopy               = "/v2/file/copy"
	pathShareSearch            = "/recommend/v1/shareLink/search"
	pathBatch                  = "/v3/batch"
	pathBatchV2                = "/adrive/v2/batch"

	subURLShareLinkCancel = "/share_link/cancel"
	subURLFileCopy        = "/file/copy"

	batchMaxPerRequest = 100

	// ShareURLBase 是阿里云盘分享链接前缀。
	ShareURLBase = "https://www.alipan.com/s/"
)

// CreateShareLinkRequest 创建分享链接请求。
type CreateShareLinkRequest struct {
	DriveID    string   `json:"drive_id,omitempty"`
	FileIDList []string `json:"file_id_list"`        // 必填
	SharePwd   string   `json:"share_pwd,omitempty"` // 提取码，空=无密码
	Expiration string   `json:"expiration,omitempty"` // RFC3339，空=永久
}

// CreateShareLinkResponse 创建分享链接响应。
type CreateShareLinkResponse struct {
	ShareID    string `json:"share_id"`
	ShareURL   string `json:"share_url"`
	SharePwd   string `json:"share_pwd"`
	ShareMsg   string `json:"share_msg"`
	FullShareMsg string `json:"full_share_msg"`
	ShareName  string `json:"share_name"`
	Expiration string `json:"expiration"`
	Expired    bool   `json:"expired"`
	Status     string `json:"status"`
	DriveID    string `json:"drive_id"`
	FileIDList []string `json:"file_id_list"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	DownloadCount int64 `json:"download_count"`
	PreviewCount  int64 `json:"preview_count"`
	SaveCount     int64 `json:"save_count"`
	SharePolicy types.SharePolicy `json:"share_policy"`
	FirstFile   *types.ShareLinkBaseFile `json:"first_file"`
}

// Create 创建分享链接，返回官方 /s/ 分享链接（多人可访问转存）。
//
// 重要：分享要求文件在【资源盘】，备份盘的文件不允许分享（返回 403）。
// 若 DriveID 为空，自动使用资源盘 ID。
func (s *Service) Create(ctx context.Context, req *CreateShareLinkRequest) (*CreateShareLinkResponse, error) {
	if req == nil || len(req.FileIDList) == 0 {
		return nil, invoker.NewAPIError(0, "InvalidArgument", "file_id_list is required")
	}
	// drive_id 为空时自动用资源盘（备份盘不允许分享）。
	if req.DriveID == "" {
		req.DriveID = s.inv.ResourceDriveID()
		if req.DriveID == "" {
			return nil, invoker.NewAPIError(0, "NoResourceDrive", "分享需要资源盘，但账号未找到资源盘 drive")
		}
	}
	var resp CreateShareLinkResponse
	if err := invoker.PostAndDecode(ctx, s.inv, pathShareLinkCreate, req, &resp, []int{200}); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateShareLinkRequest 更新分享链接请求。
type UpdateShareLinkRequest struct {
	ShareID    string `json:"share_id"`
	SharePwd   string `json:"share_pwd,omitempty"`
	Expiration string `json:"expiration,omitempty"`
}

// Update 更新分享链接（修改密码/过期时间）。
func (s *Service) Update(ctx context.Context, req *UpdateShareLinkRequest) error {
	if req == nil || req.ShareID == "" {
		return invoker.NewAPIError(0, "InvalidArgument", "share_id is required")
	}
	return invoker.PostAndDecode(ctx, s.inv, pathShareLinkUpdate, req, nil, []int{200})
}

// Cancel 取消分享。POST /adrive/v2/share_link/cancel，空响应体。
func (s *Service) Cancel(ctx context.Context, shareID string) error {
	body := map[string]any{"share_id": shareID}
	return invoker.PostAndDecode(ctx, s.inv, pathShareLinkCancel, body, nil, []int{200})
}

// CancelBatch 批量取消分享（走 /v3/batch，resource=file，子 url=/share_link/cancel）。
func (s *Service) CancelBatch(ctx context.Context, shareIDs []string) error {
	subs := make([]map[string]any, 0, len(shareIDs))
	for _, sid := range shareIDs {
		subs = append(subs, map[string]any{
			"body":    map[string]any{"share_id": sid},
			"id":      sid,
			"url":     subURLShareLinkCancel,
			"method":  "POST",
			"headers": map[string]string{"Content-Type": "application/json"},
		})
	}
	// 按 100 切片。
	for start := 0; start < len(subs); start += batchMaxPerRequest {
		end := start + batchMaxPerRequest
		if end > len(subs) {
			end = len(subs)
		}
		payload := map[string]any{"requests": subs[start:end], "resource": "file"}
		if err := invoker.PostAndDecode(ctx, s.inv, pathBatch, payload, nil, []int{200}); err != nil {
			return err
		}
	}
	return nil
}

// ListMyShareRequest 列出我的分享请求。
type ListMyShareRequest struct {
	Limit      int               `json:"limit,omitempty"`
	Marker     string            `json:"marker,omitempty"`
	OrderBy    string            `json:"order_by,omitempty"`     // share_name/created_at/description/updated_at
	OrderDirection types.OrderDirection `json:"order_direction,omitempty"`
}

// ListMyShare 列出我的分享，自动分页。
//
// 注意：阿里云盘服务端要求此接口必须带 creator=user_id，否则返回 403。
// （接口注释说"不传查自己"，但实测不传会 403，故这里自动填充当前用户 ID。）
func (s *Service) ListMyShare(ctx context.Context, req *ListMyShareRequest) ([]*types.ShareLinkSchema, error) {
	if req == nil {
		req = &ListMyShareRequest{}
	}
	body := map[string]any{
		"creator":           s.inv.UserID(), // 必填，否则 403
		"limit":             defaultInt(req.Limit, 100),
		"order_by":          defaultStr(req.OrderBy, "created_at"),
		"order_direction":   defaultStr(string(req.OrderDirection), "DESC"),
		"include_canceled":  false,
	}
	if req.Marker != "" {
		body["marker"] = req.Marker
	}
	var all []*types.ShareLinkSchema
	for {
		var resp struct {
			Items      []*types.ShareLinkSchema `json:"items"`
			NextMarker string                   `json:"next_marker"`
		}
		if err := invoker.PostAndDecode(ctx, s.inv, pathShareLinkList, body, &resp, []int{200}); err != nil {
			return nil, err
		}
		all = append(all, resp.Items...)
		if resp.NextMarker == "" {
			break
		}
		body["marker"] = resp.NextMarker
	}
	return all, nil
}

// ShareURL 把 share_id 拼成完整的分享 URL。
func ShareURL(shareID string) string {
	return ShareURLBase + shareID
}

// ShareIDFromURL 从分享 URL（https://www.alipan.com/s/xxx）提取 share_id。
func ShareIDFromURL(shareURL string) string {
	const marker = "/s/"
	idx := indexLast(shareURL, marker)
	if idx < 0 {
		return shareURL
	}
	id := shareURL[idx+len(marker):]
	// 去掉 query 和 fragment。
	for i := 0; i < len(id); i++ {
		if id[i] == '?' || id[i] == '#' {
			id = id[:i]
			break
		}
	}
	return id
}

// GetShareInfoResponse 匿名查询他人分享信息响应。
type GetShareInfoResponse struct {
	Avatar         string                `json:"avatar"`
	CreatorID      string                `json:"creator_id"`
	CreatorName    string                `json:"creator_name"`
	CreatorPhone   string                `json:"creator_phone"`
	Expiration     string                `json:"expiration"`
	FileCount      int64                 `json:"file_count"`
	FileInfos      []types.ShareItemInfo `json:"file_infos"`
	ShareName      string                `json:"share_name"`
	UpdatedAt      string                `json:"updated_at"`
	Vip            bool                  `json:"vip"`
	DisplayName    string                `json:"display_name"`
	IsFollowingCreator bool              `json:"is_following_creator"`
}

// GetShareInfo 匿名查询他人分享的基本信息（不需要登录）。
func (s *Service) GetShareInfo(ctx context.Context, shareID string) (*GetShareInfoResponse, error) {
	body := map[string]any{"share_id": shareID}
	var resp GetShareInfoResponse
	if err := invoker.PostAndDecode(ctx, s.inv, pathShareGetByAnonymous, body, &resp, []int{200}); err != nil {
		return nil, err
	}
	return &resp, nil
}

// —— helpers ——

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

func indexLast(s, sub string) int {
	// 不引入 strings.IndexLast（老版本无），简单实现。
	n, last := len(sub), -1
	for i := 0; i+n <= len(s); i++ {
		if s[i:i+n] == sub {
			last = i
		}
	}
	return last
}

// 确保 json 包被引用（部分方法后续用到）。
var _ = json.Marshal
