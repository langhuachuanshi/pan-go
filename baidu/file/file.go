// Package file 实现百度网盘文件列表查询（网页端 / BDUSS 方案）。
//
// 接口：GET pan.baidu.com/api/list
// 参数：dir（目录绝对路径）、order（排序字段 time/name/size）、desc（0/1）、num、page
//
// 与开放平台（openapi 分支）的差异：
//   - 路径 /api/list（不是 /xpan/file）
//   - 排序参数 order+desc（不是 by+order，也不是 order+desc 的开放平台变体）
//   - 分页用 num+page（不是 start+limit）
//   - 鉴权靠 BDUSS cookie（无需 STOKEN、无需 bdstoken）
package file

import (
	"context"
	"strconv"

	"github.com/langhuachuanshi/baidupan-go/baidu/invoker"
	"github.com/langhuachuanshi/baidupan-go/baidu/types"
)

// Service 文件查询入口。
type Service struct {
	inv invoker.Invoker
}

// New 创建 file Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// ListRequest 文件列表请求参数。
type ListRequest struct {
	Dir    string // 目录绝对路径，默认 "/"
	Order  string // 排序字段：time/name/size，默认 time
	Desc   bool   // 是否降序，默认 false（升序）
	Num    int    // 每页数量，默认 1000
	Web    bool   // 是否返回缩略图等扩展字段（web=1）
}

// List 列出指定目录下的文件。自动分页，返回全部。
func (s *Service) List(ctx context.Context, req *ListRequest) ([]*types.File, error) {
	if req == nil {
		req = &ListRequest{}
	}
	dir := req.Dir
	if dir == "" {
		dir = "/"
	}
	pageSize := req.Num
	if pageSize <= 0 {
		pageSize = 1000
	}
	order := req.Order
	if order == "" {
		order = "time"
	}

	var all []*types.File
	page := 1
	for {
		params := map[string]string{
			"order": order,
			"desc":  boolToStr(req.Desc),
			"dir":   dir,
			"num":   strconv.Itoa(pageSize),
			"page":  strconv.Itoa(page),
		}
		if req.Web {
			params["web"] = "1"
		}
		var resp listResponse
		if err := invoker.GetAndDecode(ctx, s.inv, "/api/list", params, &resp); err != nil {
			return nil, err
		}
		if resp.Errno != 0 {
			return nil, invoker.NewAPIError(resp.Errno, resp.ErrMsg())
		}
		if len(resp.List) == 0 {
			break
		}
		all = append(all, resp.List...)
		if len(resp.List) < pageSize {
			break
		}
		page++
	}
	return all, nil
}

// listResponse 文件列表响应。
// 百度外层：{errno, list, guid, guid_info, request_id}。
type listResponse struct {
	Errno int           `json:"errno"`
	List  []*types.File `json:"list"`
}

// ErrMsg errno 非 0 时的描述。
func (r *listResponse) ErrMsg() string {
	return errnoMsg(r.Errno)
}

// errnoMsg 常见 errno 的中文描述。
func errnoMsg(errno int) string {
	switch errno {
	case -6:
		return "BDUSS 失效（请重新登录网页端获取新 BDUSS）"
	case -7:
		return "路径不存在或无权访问"
	case -9:
		return "文件不存在"
	default:
		return "请求失败"
	}
}

func boolToStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
