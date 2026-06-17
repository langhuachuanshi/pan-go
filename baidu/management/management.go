// Package management 实现百度网盘文件管理（建目录/移动/重命名/删除）。
//
// 接口：
//   - 建目录：POST /xpan/file?method=create，body {path, isdir=1}
//   - 移动/重命名/删除：POST /xpan/file?method=filemanager&opera={move|rename|delete}
//     body {filelist: JSON数组, async, ondup}
//
// filemanager 是批量操作，filelist 是 JSON 字符串数组。
package management

import (
	"context"
	"encoding/json"

	"github.com/langhuachuanshi/panbaidu-go/baidu/invoker"
	"github.com/langhuachuanshi/panbaidu-go/baidu/types"
)

// Service 文件管理入口。
type Service struct {
	inv invoker.Invoker
}

// New 创建 management Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// —— 建目录 ——
// 接口：POST /xpan/file?method=create，复用上传第三步的 URL，靠 isdir=1 区分。

// MakeDir 创建目录，返回新目录信息。
// dirPath 是绝对路径（如 /apps/mydir）。
func (s *Service) MakeDir(ctx context.Context, dirPath string) (*types.File, error) {
	if dirPath == "" {
		return nil, invoker.NewAPIError(0, "dirPath is required")
	}
	body := map[string]string{"path": dirPath, "isdir": "1"}
	params := map[string]string{"method": "create"}
	var resp struct {
		Errno int         `json:"errno"`
		Data  types.File  `json:"data"`
	}
	if err := invoker.PostFormAndDecode(ctx, s.inv, "/xpan/file", body, params, &resp); err != nil {
		return nil, err
	}
	if resp.Errno != 0 {
		return nil, invoker.NewAPIError(resp.Errno, errnoMsg(resp.Errno))
	}
	f := resp.Data
	return &f, nil
}

// —— 移动/重命名/删除（filemanager）——

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
	return s.filemanager(ctx, "move", items, "fail")
}

// Rename 重命名单个文件/目录（改路径末段名）。
// newPath 是完整的新路径（如 /apps/oldname → /apps/newname）。
func (s *Service) Rename(ctx context.Context, oldPath, newPath string) error {
	items := []map[string]string{{"path": oldPath, "newname": baseName(newPath)}}
	return s.filemanager(ctx, "rename", items, "fail")
}

// Delete 删除 paths（一个或多个），进回收站。
func (s *Service) Delete(ctx context.Context, paths []string) error {
	// delete 的 filelist 是字符串数组（不是对象数组）。
	filelistJSON, _ := json.Marshal(paths)
	return s.filemanagerRaw(ctx, "delete", string(filelistJSON))
}

// filemanager 移动/重命名的通用调用（filelist 是对象数组）。
func (s *Service) filemanager(ctx context.Context, opera string, items []map[string]string, ondup string) error {
	filelistJSON, _ := json.Marshal(items)
	return s.filemanagerRawWithOndup(ctx, opera, string(filelistJSON), ondup)
}

func (s *Service) filemanagerRaw(ctx context.Context, opera, filelistJSON string) error {
	return s.filemanagerRawWithOndup(ctx, opera, filelistJSON, "")
}

func (s *Service) filemanagerRawWithOndup(ctx context.Context, opera, filelistJSON, ondup string) error {
	body := map[string]string{
		"filelist": filelistJSON,
		"async":    "1", // 1=自适应（小批量同步，大批量异步）
	}
	if ondup != "" {
		body["ondup"] = ondup
	}
	params := map[string]string{"method": "filemanager", "opera": opera}
	var resp struct {
		Errno  int `json:"errno"`
		TaskID int `json:"taskid"` // 异步时非0
	}
	if err := invoker.PostFormAndDecode(ctx, s.inv, "/xpan/file", body, params, &resp); err != nil {
		return err
	}
	if resp.Errno != 0 {
		return invoker.NewAPIError(resp.Errno, errnoMsg(resp.Errno))
	}
	return nil
}

// baseName 取路径末段（文件名）。百度 move/rename 的 newname 只要名字，不要完整路径。
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
		return "access_token 失效或权限不足"
	case -7:
		return "路径不存在或无权访问"
	case -8:
		return "路径已存在"
	case 31064:
		return "文件不存在"
	default:
		return "请求失败"
	}
}
