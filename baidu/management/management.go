// Package management 实现百度网盘文件管理（网页端 / BDUSS 方案）。
//
// 接口（实测确认）：
//   - 建目录：POST pan.baidu.com/api/create，body {path, isdir=1}
//   - 移动：POST pan.baidu.com/api/filemanager?opera=move，body {filelist:[{path,dest,newname}]}
//   - 重命名：POST pan.baidu.com/api/filemanager?opera=rename，body {filelist:[{path,newname}]}
//   - 删除：POST pan.baidu.com/api/filemanager?opera=delete，body {filelist:[path,...]}
//
// 鉴权靠 BDUSS+STOKEN cookie + bdstoken（由 client 自动注入）。
// 注意 filemanager 的 opera 放 query（不是 method=），实测 method=filemanager 会报 errno=2。
package management

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/langhuachuanshi/baidupan-go/baidu/invoker"
	"github.com/langhuachuanshi/baidupan-go/baidu/types"
)

// Service 文件管理入口。
type Service struct {
	inv invoker.Invoker
}

// New 创建 management Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// MakeDir 创建目录，返回新目录信息。
// dirPath 是绝对路径（如 /apps/mydir）。
func (s *Service) MakeDir(ctx context.Context, dirPath string) (*types.File, error) {
	if dirPath == "" {
		return nil, invoker.NewAPIError(0, "dirPath is required")
	}
	body := map[string]string{"path": dirPath, "isdir": "1"}
	// /api/create 响应是扁平结构（errno/fs_id/path/ctime/mtime/... 直接在顶层，非 data 嵌套）。
	var resp struct {
		Errno    int    `json:"errno"`
		FsID     int64  `json:"fs_id"`
		Path     string `json:"path"`
		Ctime    int64  `json:"ctime"`
		Mtime    int64  `json:"mtime"`
		Category int    `json:"category"`
		Isdir    int    `json:"isdir"`
	}
	if err := invoker.PostFormAndDecode(ctx, s.inv, "/api/create", body, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Errno != 0 {
		return nil, invoker.NewAPIError(resp.Errno, errnoMsg(resp.Errno))
	}
	// /api/create 响应是扁平结构（非 data 嵌套），手动组装 File。
	return &types.File{
		FSID:        resp.FsID,
		Path:        resp.Path,
		Size:        0,
		IsDir:       resp.Isdir,
		Category:    resp.Category,
		ServerCTime: resp.Ctime,
		ServerMTime: resp.Mtime,
	}, nil
}

// —— 移动/重命名（filemanager）/ 删除（filemanager）——

// Copy 把 sourcePaths（一个或多个）复制到 destDir 目录下。
func (s *Service) Copy(ctx context.Context, sourcePaths []string, destDir string) error {
	items := make([]map[string]string, 0, len(sourcePaths))
	for _, src := range sourcePaths {
		items = append(items, map[string]string{
			"path":    src,
			"dest":    destDir,
			"newname": baseName(src),
		})
	}
	return s.filemanager(ctx, "copy", items, nil)
}

// Move 把 sourcePaths（一个或多个）移动到 destDir 目录下。
func (s *Service) Move(ctx context.Context, sourcePaths []string, destDir string) error {
	items := make([]map[string]string, 0, len(sourcePaths))
	for _, src := range sourcePaths {
		items = append(items, map[string]string{
			"path":    src,
			"dest":    destDir,
			"newname": baseName(src),
		})
	}
	return s.filemanager(ctx, "move", items, nil)
}

// Rename 重命名单个文件/目录（改路径末段名）。
// newPath 是完整的新路径（如 /apps/oldname → /apps/newname）。
func (s *Service) Rename(ctx context.Context, oldPath, newPath string) error {
	items := []map[string]string{{"path": oldPath, "newname": baseName(newPath)}}
	return s.filemanager(ctx, "rename", items, nil)
}

// Delete 删除 paths（一个或多个），进回收站。
func (s *Service) Delete(ctx context.Context, paths []string) error {
	// delete 的 filelist 是字符串数组。
	filelistJSON, _ := json.Marshal(paths)
	body := map[string]string{"filelist": string(filelistJSON)}
	var resp struct {
		Errno int `json:"errno"`
	}
	if err := invoker.PostFormAndDecode(ctx, s.inv, "/api/filemanager", body, map[string]string{"opera": "delete"}, &resp); err != nil {
		return err
	}
	if resp.Errno != 0 {
		return invoker.NewAPIError(resp.Errno, errnoMsg(resp.Errno))
	}
	return nil
}

// filemanager 批量操作（move/rename）。filelist 是对象数组，序列化后放 body 的 filelist 字段。
func (s *Service) filemanager(ctx context.Context, opera string, items []map[string]string, _ map[string]string) error {
	filelistJSON, _ := json.Marshal(items)
	body := map[string]string{"filelist": string(filelistJSON)}
	// opera 放 params（由 client 拼进 query），不要放 path，否则会和 client 的 ? 拼接冲突。
	var resp struct {
		Errno int `json:"errno"`
	}
	if err := invoker.PostFormAndDecode(ctx, s.inv, "/api/filemanager", body, map[string]string{"opera": opera}, &resp); err != nil {
		return err
	}
	if resp.Errno != 0 {
		return invoker.NewAPIError(resp.Errno, errnoMsg(resp.Errno))
	}
	return nil
}

// baseName 取路径末段（文件名）。
func baseName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}

// errnoMsg 常见 errno 的中文描述。
func errnoMsg(errno int) string {
	switch errno {
	case -6:
		return "BDUSS/STOKEN 失效（请重新登录网页端获取新凭证）"
	case -7:
		return "路径不存在或无权访问"
	case -8:
		return "路径已存在"
	case 2:
		return "参数格式错误"
	case 8001:
		return "参数错误或路径不存在"
	default:
		return "请求失败"
	}
}

// —— 回收站操作 ——

// RecycleFile 回收站文件信息。
type RecycleFile struct {
	FSID     int64  `json:"fs_id"`
	Path     string `json:"path"`
	Filename string `json:"server_filename"`
	Size     int64  `json:"size"`
	IsDir    int    `json:"isdir"`
	CTime    int64  `json:"server_ctime"`
	MTime    int64  `json:"server_mtime"`
}

// RecycleList 获取回收站文件列表。
// page: 页码，从 1 开始，每页 100 条。
func (s *Service) RecycleList(ctx context.Context, page int) ([]*RecycleFile, error) {
	data, _, err := s.inv.Get(ctx, "/api/recycle/list", map[string]string{
		"num":  "100",
		"page": strconv.Itoa(page),
	})
	if err != nil {
		return nil, fmt.Errorf("获取回收站列表失败: %w", err)
	}
	var resp struct {
		Errno int             `json:"errno"`
		List  []*RecycleFile  `json:"list"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("解析回收站列表失败: %w", err)
	}
	if resp.Errno != 0 {
		return nil, invoker.NewAPIError(resp.Errno, "获取回收站列表失败")
	}
	return resp.List, nil
}

// RecycleRestore 从回收站还原文件（通过 fs_id）。
func (s *Service) RecycleRestore(ctx context.Context, fsIDs []int64) error {
	ids := make([]string, len(fsIDs))
	for i, id := range fsIDs {
		ids[i] = strconv.FormatInt(id, 10)
	}
	fidList := "[" + stringsJoin(ids, ",") + "]"
	body := map[string]string{"fidlist": fidList}

	var resp struct {
		Errno int `json:"errno"`
	}
	if err := invoker.PostFormAndDecode(ctx, s.inv, "/api/recycle/restore", body, nil, &resp); err != nil {
		return err
	}
	if resp.Errno != 0 {
		return invoker.NewAPIError(resp.Errno, "还原失败")
	}
	return nil
}

// RecycleDelete 从回收站彻底删除文件（通过 fs_id）。
func (s *Service) RecycleDelete(ctx context.Context, fsIDs []int64) error {
	ids := make([]string, len(fsIDs))
	for i, id := range fsIDs {
		ids[i] = strconv.FormatInt(id, 10)
	}
	fidList := "[" + stringsJoin(ids, ",") + "]"
	body := map[string]string{"fidlist": fidList}

	var resp struct {
		Errno int `json:"errno"`
	}
	if err := invoker.PostFormAndDecode(ctx, s.inv, "/api/recycle/delete", body, nil, &resp); err != nil {
		return err
	}
	if resp.Errno != 0 {
		return invoker.NewAPIError(resp.Errno, "彻底删除失败")
	}
	return nil
}

// stringsJoin 连接字符串数组。
func stringsJoin(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
