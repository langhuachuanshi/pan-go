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
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/langhuachuanshi/pan-go/baidu/invoker"
	"github.com/langhuachuanshi/pan-go/baidu/types"
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

// SearchRequest 文件搜索请求。
type SearchRequest struct {
	Dir       string // 搜索目录（绝对路径，默认 "/"）
	Keyword   string // 搜索关键词
	Recursive bool   // 是否递归搜索子目录
}

// Search 按文件名搜索文件（PCS 接口）。
// 接口：GET https://pcs.baidu.com/rest/2.0/pcs/file?method=search&app_id=250528&path=...&wd=...&re=...
func (s *Service) Search(ctx context.Context, req *SearchRequest) ([]*types.File, error) {
	if req.Dir == "" {
		req.Dir = "/"
	}
	reStr := "0"
	if req.Recursive {
		reStr = "1"
	}

	fullURL := "https://pcs.baidu.com/rest/2.0/pcs/file?method=search" +
		"&app_id=250528" +
		"&path=" + req.Dir +
		"&wd=" + req.Keyword +
		"&re=" + reStr

	data, _, err := s.inv.GetRaw(ctx, fullURL)
	if err != nil {
		return nil, err
	}

	// PCS search 返回 {errno, list: [...]}
	var resp struct {
		Errno int          `json:"errno"`
		List  []*types.File `json:"list"`
	}
	if err := invoker.Decode(data, &resp); err != nil {
		return nil, err
	}
	if resp.Errno != 0 {
		return nil, invoker.NewAPIError(resp.Errno, "搜索失败")
	}
	return resp.List, nil
}

// RecurseList 递归列出目录下所有文件（含子目录）。
// 基于 List 方法，自动递归进入子目录，返回扁平化的全部文件列表。
func (s *Service) RecurseList(ctx context.Context, dir string) ([]*types.File, error) {
	if dir == "" {
		dir = "/"
	}
	var all []*types.File
	if err := s.recurse(ctx, dir, &all); err != nil {
		return nil, err
	}
	return all, nil
}

func (s *Service) recurse(ctx context.Context, dir string, result *[]*types.File) error {
	files, err := s.List(ctx, &ListRequest{Dir: dir})
	if err != nil {
		return err
	}
	for _, f := range files {
		*result = append(*result, f)
		if f.IsFolder() {
			if err := s.recurse(ctx, f.Path, result); err != nil {
				return err
			}
		}
	}
	return nil
}

// Meta 获取单个文件的元信息（通过 /api/filemetas 接口，使用 fs_id）。
func (s *Service) Meta(ctx context.Context, fsid int64) (*types.File, error) {
	list, err := s.BatchMeta(ctx, []int64{fsid})
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, invoker.NewAPIError(0, "文件不存在")
	}
	return list[0], nil
}

// BatchMeta 批量获取文件元信息（通过 /api/filemetas 接口）。
// 接口：GET https://pan.baidu.com/api/filemetas?fsids=[fsid1,fsid2,...]&dlink=1
// 注意：/api/filemetas 返回的字段与 /api/list 格式略不同（部分数字字段为字符串），内部做转换。
func (s *Service) BatchMeta(ctx context.Context, fsids []int64) ([]*types.File, error) {
	fsidJSON, _ := json.Marshal(fsids)
	params := map[string]string{
		"fsids": string(fsidJSON),
		"dlink": "1",
	}

	// /api/filemetas 返回的字段类型与 /api/list 不完全一致
	// category/size/isdir 等可能是字符串，用专门的结构体接收后转换
	var resp struct {
		Errno int            `json:"errno"`
		Info  []*fileMetaRaw `json:"info"`
	}
	if err := s.inv.Get(ctx, "/api/filemetas", params, &resp); err != nil {
		return nil, fmt.Errorf("获取元信息失败: %w", err)
	}
	if resp.Errno != 0 {
		return nil, invoker.NewAPIError(resp.Errno, "获取元信息失败")
	}

	files := make([]*types.File, len(resp.Info))
	for i, raw := range resp.Info {
		files[i] = raw.toFile()
	}
	return files, nil
}

// MatchPath 通配符匹配文件路径（支持 * 和 ? 通配符）。
// pattern 如 /temp/*.zip、/video/*/*.mp4。
// 自动从 pattern 前缀提取起始目录，避免全盘遍历。
// 底层用 RecurseList + path.Match 过滤。
func (s *Service) MatchPath(ctx context.Context, pattern string) ([]*types.File, error) {
	// 从 pattern 提取不含通配符的最长前缀作为起始目录
	root := extractDir(pattern)
	if root == "" {
		root = "/"
	}

	all, err := s.RecurseList(ctx, root)
	if err != nil {
		return nil, err
	}
	var matched []*types.File
	for _, f := range all {
		ok, err := path.Match(pattern, f.Path)
		if err != nil {
			return nil, fmt.Errorf("通配符匹配错误: %w", err)
		}
		if ok {
			matched = append(matched, f)
		}
	}
	return matched, nil
}

// extractDir 从通配符路径中提取不含通配符的最长前缀。
// 如 /temp/*.zip → /temp，/video/**/*.mp4 → /video。
func extractDir(pattern string) string {
	for i, c := range pattern {
		if c == '*' || c == '?' || c == '[' {
			base := pattern[:i]
			// 取上一级目录
			if idx := strings.LastIndex(base, "/"); idx >= 0 {
				return base[:idx]
			}
			return "/"
		}
	}
	return "/"
}


// fileMetaRaw 兼容 /api/filemetas 的响应格式（字段可能是字符串）。
type fileMetaRaw struct {
	FSID           json.Number `json:"fs_id"`
	Path           string      `json:"path"`
	ServerFilename string      `json:"server_filename"`
	Size           json.Number `json:"size"`
	IsDir          json.Number `json:"isdir"`
	Category       json.Number `json:"category"`
	MD5            string      `json:"md5"`
	ServerCTime    json.Number `json:"server_ctime"`
	ServerMTime    json.Number `json:"server_mtime"`
	LocalCTime     json.Number `json:"local_ctime"`
	LocalMTime     json.Number `json:"local_mtime"`
}

func (r *fileMetaRaw) toFile() *types.File {
	return &types.File{
		FSID:           numInt64(r.FSID),
		Path:           r.Path,
		ServerFilename: r.ServerFilename,
		Size:           numInt64(r.Size),
		IsDir:          numInt(r.IsDir),
		Category:       numInt(r.Category),
		MD5:            r.MD5,
		ServerCTime:    numInt64(r.ServerCTime),
		ServerMTime:    numInt64(r.ServerMTime),
		LocalCTime:     numInt64(r.LocalCTime),
		LocalMTime:     numInt64(r.LocalMTime),
	}
}

func numInt64(n json.Number) int64 {
	v, _ := n.Int64()
	return v
}

func numInt(n json.Number) int {
	v, _ := n.Int64()
	return int(v)
}
