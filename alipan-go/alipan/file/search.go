package file

import (
	"context"
	"fmt"
	"strings"

	"github.com/langhuachuanshi/alipan-go/alipan/invoker"
	"github.com/langhuachuanshi/alipan-go/alipan/types"
)

// SearchRequest 搜索请求。
type SearchRequest struct {
	Query   string            `json:"query"`
	DriveID string            `json:"drive_id,omitempty"`
	Limit   int               `json:"limit,omitempty"`
	Marker  string            `json:"marker,omitempty"`
	OrderBy types.ListOrderBy `json:"order_by,omitempty"`
}

// Search 搜索文件，自动分页。
func (s *Service) Search(ctx context.Context, req *SearchRequest) ([]*types.BaseFile, error) {
	if req == nil || req.Query == "" {
		return nil, invoker.NewAPIError(0, "InvalidArgument", "query is required")
	}
	var all []*types.BaseFile
	for {
		page, next, err := s.searchPage(ctx, req)
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

func (s *Service) searchPage(ctx context.Context, req *SearchRequest) ([]*types.BaseFile, string, error) {
	body := map[string]any{
		"query":                   req.Query,
		"drive_id":                req.DriveID,
		"limit":                   defaultInt(req.Limit, 100),
		"image_thumbnail_process": "image/resize,w_160/format,jpeg",
		"image_url_process":       "image/resize,w_1920/format,jpeg",
		"url_expire_sec":          14400,
		"video_thumbnail_process": "video/snapshot,t_0,f_jpg,ar_auto,w_800",
	}
	if req.Marker != "" {
		body["marker"] = req.Marker
	}
	if req.OrderBy != "" {
		body["order_by"] = req.OrderBy
	}
	var resp fileListResponse
	if err := invoker.PostAndDecode(ctx, s.inv, pathFileSearch, body, &resp, []int{200}); err != nil {
		return nil, "", err
	}
	return resp.Items, resp.NextMarker, nil
}

// SearchByName 按文件名模糊搜索。
func (s *Service) SearchByName(ctx context.Context, keyword, driveID string) ([]*types.BaseFile, error) {
	keyword = strings.ReplaceAll(keyword, `"`, `\"`)
	query := fmt.Sprintf(`name match "%s"`, keyword)
	return s.Search(ctx, &SearchRequest{Query: query, DriveID: driveID})
}
