// Package types 定义 quark-go 的所有数据模型。
//
// 夸克网盘的字段使用 snake_case，但部分用驼峰（如 fid、pdir_fid 是 snake，
// 而 objCategory、objUid 是驼峰）——以夸克 API 实际返回为准。
// 本包是最底层，不依赖任何其它包。
package types

// File 夸克网盘文件对象。
type File struct {
	// 标识
	FID       string `json:"fid"`        // 文件唯一 ID（根目录是 "0"）
	PDirFID   string `json:"pdir_fid"`   // 父目录 ID
	Category  int    `json:"category"`   // 0=文件夹, 1=文件
	FileType  int    `json:"file_type"`  // 0=文件夹 1=文件（部分接口用这个）
	FileName  string `json:"file_name"`  // 文件名
	FormatType string `json:"format_type"` // 文件格式类型
	Size      int64  `json:"size"`        // 字节数（文件夹为0）
	ObjCategory string `json:"objCategory"` // 对象分类（图片/视频/文档等）

	// 时间（毫秒时间戳）
	CreatedAt    int64 `json:"created_at"`
	UpdatedAt    int64 `json:"updated_at"`
	LUpdatedTime int64 `json:"l_updated_at"` // 最后更新

	// 状态
	FileStatus  int    `json:"file_status"`  // 文件状态
	SaveType    int    `json:"save_type"`    // 保存类型
	RiskType    int    `json:"risk_type"`    // 风险类型（0=正常）
	Deleted     bool   `json:"deleted"`      // 是否已删除
	Dir         bool   `json:"dir"`          // 是否目录（冗余字段，部分接口返回）

	// 哈希/校验
	Hash        string `json:"hash"`         // 文件哈希（sha1）
	HashAlgo    string `json:"hash_algo"`    // 哈希算法

	// 缩略图
	Thumbnail   string `json:"thumbnail"`    // 缩略图 URL
	RawUploadID string `json:"raw_upload_id"`

	// 其它
	ObjOrigin   string `json:"obj_origin"`
	ObjUname    string `json:"obj_uname"`
	TaskID      string `json:"task_id"`
}

// IsFolder 判断是否文件夹。
func (f *File) IsFolder() bool {
	if f == nil {
		return false
	}
	return f.Category == 0 || f.FileType == 0 || f.Dir
}

// IsFile 判断是否普通文件。
func (f *File) IsFile() bool {
	if f == nil {
		return false
	}
	return !f.IsFolder()
}

// FileInfo 文件简要信息（部分接口返回精简结构）。
type FileInfo struct {
	FID      string `json:"fid"`
	FileName string `json:"file_name"`
	Size     int64  `json:"size"`
	Category int    `json:"category"`
}

// ShareInfo 分享信息。
type ShareInfo struct {
	ShareID    string `json:"share_id"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	FirstFID   string `json:"first_file_fid"`
	FileCount  int64  `json:"file_count"`
	Expired    int    `json:"expired"`     // 1=已过期
	ExpiredType int   `json:"expired_type"`
	ShareType  int    `json:"share_type"`
}

// ShareDetail 分享详情（detail 接口返回）。
type ShareDetail struct {
	ShareID   string `json:"share_id"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Expired   int    `json:"expired"`
	FidList   []string `json:"fid_list"`
	ObjCount  int64   `json:"obj_count"`
	// 文件列表在 detail 的 list 字段。
}
