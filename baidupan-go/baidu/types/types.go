// Package types 定义 panbaidu-go 的所有数据模型。
//
// 百度网盘开放平台的特点：
//   - 文件用 fs_id（int64）唯一标识，不是 fid 字符串
//   - 路径用绝对路径（如 /apps/xxx/file.txt），不是目录 id
//   - 响应外层统一 {errno, ...}，errno==0 才成功
package types

import "path"

// File 百度网盘文件对象。
type File struct {
	FSID           int64  `json:"fs_id"`            // 文件唯一 ID（int64）
	Path           string `json:"path"`             // 绝对路径
	ServerFilename string `json:"server_filename"`  // 文件名
	Size           int64  `json:"size"`             // 字节数（目录为 0）
	IsDir          int    `json:"isdir"`            // 0=文件 1=目录
	Category       int    `json:"category"`         // 6=目录
	MD5            string `json:"md5"`              // 文件 md5
	ServerCTime    int64  `json:"server_ctime"`     // 创建时间（秒）
	ServerMTime    int64  `json:"server_mtime"`     // 修改时间（秒）
	LocalCTime     int64  `json:"local_ctime"`
	LocalMTime     int64  `json:"local_mtime"`
}

// IsFolder 判断是否文件夹。
func (f *File) IsFolder() bool {
	if f == nil {
		return false
	}
	return f.IsDir == 1 || f.Category == 6
}

// Name 返回文件名（优先 ServerFilename，兜底从 Path 取末段）。
func (f *File) Name() string {
	if f == nil {
		return ""
	}
	if f.ServerFilename != "" {
		return f.ServerFilename
	}
	return path.Base(f.Path)
}
