package lanzou

import "encoding/json"

// apiResp 蓝奏云 API 通用响应结构
type apiResp struct {
	Zt   int             `json:"zt"`
	Info json.RawMessage `json:"info"`
	Dom  string          `json:"dom"`
	Text json.RawMessage `json:"text"`
}

// FolderInfo 文件夹信息
type FolderInfo struct {
	FolID      string `json:"fol_id"`
	Name       string `json:"name"`
	FolderDesc string `json:"folder_des"`
	Onof       string `json:"onof"`
	IsLock     string `json:"is_lock"`
}

// FileInfo 文件列表中的文件信息
type FileInfo struct {
	ID      string `json:"id"`
	NameAll string `json:"name_all"`
	Size    string `json:"size"`
	Icon    string `json:"icon"`
	Time    string `json:"time"`
	Downs   string `json:"downs"`
	Onof    string `json:"onof"`
	IsNewd  string `json:"is_newd"`
	FID     string `json:"f_id"`
}

// FileDetail 文件详情（含直链）
type FileDetail struct {
	FileID      string `json:"file_id"`
	NameAll     string `json:"name_all"`
	Size        string `json:"size"`
	UploadTime  string `json:"time"`
	DownloadURL string `json:"url"`
	DURL        string `json:"durl"`
	Description string `json:"des"`
	IsNewd      int    `json:"is_newd"`
}

// FolderList 文件夹列表响应
type FolderList struct {
	Zt   int             `json:"zt"`
	Info json.RawMessage `json:"info"`
	Text []*FolderInfo   `json:"text"`
}

// FileList 文件列表响应
type FileList struct {
	Zt   int             `json:"zt"`
	Info json.RawMessage `json:"info"`
	Text []*FileInfo     `json:"text"`
}

// RecycleList 回收站列表响应
type RecycleList struct {
	Zt   int             `json:"zt"`
	Info string          `json:"info"`
	Text []*RecycledFile `json:"text"`
}

// RecycledFile 回收站中的文件
type RecycledFile struct {
	FileID     string `json:"id"`
	FileName   string `json:"name_all"`
	FilePath   string `json:"path"`
	UploadTime string `json:"time"`
}

// UploadResult 上传结果
type UploadResult struct {
	Zt       int    `json:"zt"`
	Info     string `json:"info"`
	FileID   string `json:"file_id"`
	FileName string `json:"name_all"`
}

// UserInfo 用户信息
type UserInfo struct {
	UserID   int    `json:"user_id"`
	UserName string `json:"nickname"`
	VIP      int    `json:"vip"`
}

// AccountInfo 帐号详细信息
type AccountInfo struct {
	UserInfo
	TotalSize string `json:"total_size"`
	UsedSize  string `json:"used_size"`
}
