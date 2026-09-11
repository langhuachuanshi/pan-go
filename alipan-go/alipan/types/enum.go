// Package types 定义 alipan-go 的所有数据模型，对应该 aligo 的 src/aligo/types/。
//
// 本包是最底层，不依赖任何其它包。所有字段使用 snake_case 的 json tag，
// 与阿里云盘 API 一致。
package types

// 本文件定义所有枚举类型，对应该 aligo 的 src/aligo/types/Enum.py。
//
// 这些是 typing.Literal 字符串约束，运行时就是普通字符串，没有数值编码。
// 注意大小写：OrderDirection 是 ASC/DESC（大写）。

// FileType 文件类型。
type FileType string

const (
	FileTypeFile   FileType = "file"
	FileTypeFolder FileType = "folder"
)

// FileCategory 文件分类。
type FileCategory string

const (
	CategoryOthers FileCategory = "others"
	CategoryDoc    FileCategory = "doc"
	CategoryImage  FileCategory = "image"
	CategoryAudio  FileCategory = "audio"
	CategoryVideo  FileCategory = "video"
)

// FileContentHashName 内容哈希算法。
type FileContentHashName string

const (
	HashNameSHA1 FileContentHashName = "sha1"
)

// FileStatus 文件状态。
type FileStatus string

const (
	FileStatusUploading FileStatus = "uploading"
	FileStatusAvailable FileStatus = "available"
)

// OrderDirection 排序方向（注意大写）。
type OrderDirection string

const (
	OrderAsc  OrderDirection = "ASC"
	OrderDesc OrderDirection = "DESC"
)

// CheckNameMode 重名处理模式。
type CheckNameMode string

const (
	CheckNameAutoRename CheckNameMode = "auto_rename"
	CheckNameRefuse     CheckNameMode = "refuse"
	CheckNameOverwrite  CheckNameMode = "overwrite"
)

// ListOrderBy 列表排序字段。
type ListOrderBy string

const (
	OrderByCreatedAt ListOrderBy = "created_at"
	OrderByUpdatedAt ListOrderBy = "updated_at"
	OrderByName      ListOrderBy = "name"
	OrderBySize      ListOrderBy = "size"
)

// SharePolicy 分享策略。
type SharePolicy string

const (
	SharePolicyURL SharePolicy = "url"
	SharePolicyMsg SharePolicy = "msg"
)
