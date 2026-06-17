// Package file 实现百度网盘文件列表查询。
//
// 接口：GET /xpan/file?method=list
// 参数：dir（目录绝对路径，默认 /）、order、desc、start、limit（必须配 start）、web、folder
package file

import (
	"context"
	"strconv"

	"github.com/langhuachuanshi/panbaidu-go/baidu/invoker"
	"github.com/langhuachuanshi/panbaidu-go/baidu/types"
)

// Service 文件查询入口。
type Service struct {
	inv invoker.Invoker
}

// New 创建 file Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// ListRequest 文件列表请求参数。
type ListRequest struct {
	Dir   string // 目录绝对路径，默认 "/"
	Order string // 排序：time/name/size，默认 time
	Desc  bool   // 是否降序
	Start int    // 起始偏移，默认 0
	Limit int    // 数量，默认 1000（必须 >0 才生效，否则服务端忽略）
	Web   bool   // 是否返回缩略图
	Folder bool  // 是否仅返回文件夹
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
	pageSize := req.Limit
	if pageSize <= 0 {
		pageSize = 1000
	}

	var all []*types.File
	start := req.Start
	for {
		// 按需传参：百度对部分参数传空串敏感，只传有值的。
		params := map[string]string{
			"method": "list",
			"dir":    dir,
			"start":  strconv.Itoa(start),
			"limit":  strconv.Itoa(pageSize),
		}
		if req.Order != "" {
			params["order"] = req.Order
			params["desc"] = boolToStr(req.Desc) // desc 必须配 order
		}
		if req.Web {
			params["web"] = "1"
		}
		if req.Folder {
			params["folder"] = "1"
		}
		var resp listResponse
		if err := invoker.GetAndDecode(ctx, s.inv, "/xpan/file", params, &resp); err != nil {
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
		start += pageSize
	}
	return all, nil
}

// listResponse 文件列表响应。
// 百度外层：{errno, list, request_id}。
type listResponse struct {
	Errno int           `json:"errno"`
	List  []*types.File `json:"list"`
}

// ErrMsg errno 非 0 时的描述（百度不返回 message，这里给通用文案）。
func (r *listResponse) ErrMsg() string {
	return errnoMsg(r.Errno)
}

// errnoMsg 常见 errno 的中文描述。
func errnoMsg(errno int) string {
	switch errno {
	case -6:
		return "access_token 失效或权限不足"
	case -7:
		return "路径不存在或无权访问"
	case 31064:
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
