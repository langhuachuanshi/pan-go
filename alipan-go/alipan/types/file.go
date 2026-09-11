package types

// 本文件定义文件相关核心数据结构，对应该 aligo 的 src/aligo/types/BaseFile.py。
// 字段全部 snake_case；时间字段是 ISO8601 字符串。

// BaseFile 阿里云盘文件对象，所有列表/搜索/获取接口的统一返回类型。
type BaseFile struct {
	FileID        string        `json:"file_id"`
	DriveID       string        `json:"drive_id"`
	DomainID      string        `json:"domain_id"`
	ParentFileID  string        `json:"parent_file_id"`
	Name          string        `json:"name"`
	Type          FileType      `json:"type"`
	Category      FileCategory  `json:"category"`
	Size          int64         `json:"size"`
	FileExtension string        `json:"file_extension"`
	MimeType      string        `json:"mime_type"`
	MimeExtension string        `json:"mime_extension"`
	ContentType   string        `json:"content_type"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	TrashedAt string `json:"trashed_at"`

	Status      string `json:"status"`
	Trashed     bool   `json:"trashed"`
	Hidden      bool   `json:"hidden"`
	Starred     bool   `json:"starred"`
	EncryptMode string `json:"encrypt_mode"`

	ContentHash     string              `json:"content_hash"`
	ContentHashName FileContentHashName `json:"content_hash_name"`
	CRC64Hash       string              `json:"crc64_hash"`

	DownloadURL string `json:"download_url"`
	URL         string `json:"url"`
	Thumbnail   string `json:"thumbnail"`

	StreamsInfo    map[string]any `json:"streams_info"`
	StreamsURLInfo map[string]any `json:"streams_url_info"`

	ImageMediaMetadata   *ImageMedia   `json:"image_media_metadata"`
	VideoMediaMetadata   *VideoMedia   `json:"video_media_metadata"`
	VideoPreviewMetadata *VideoPreview `json:"video_preview_metadata"`

	Description string         `json:"description"`
	Meta        string         `json:"meta"`
	UserMeta    string         `json:"user_meta"`
	Labels      []string       `json:"labels"`
	UserTags    map[string]any `json:"user_tags"`
	Location    string         `json:"location"`

	UploadID        string `json:"upload_id"`
	RevisionID      string `json:"revision_id"`
	RevisionVersion int64  `json:"revision_version"`

	SyncFlag       bool   `json:"sync_flag"`
	SyncDeviceFlag bool   `json:"sync_device_flag"`
	SyncMeta       string `json:"sync_meta"`

	LastModifierType string `json:"last_modifier_type"`
	LastModifierID   string `json:"last_modifier_id"`
	LastModifierName string `json:"last_modifier_name"`
	CreatorType      string `json:"creator_type"`
	CreatorID        string `json:"creator_id"`
	CreatorName      string `json:"creator_name"`

	FromShareID                 string   `json:"from_share_id"`
	Channel                     string   `json:"channel"`
	PunishFlag                  int64    `json:"punish_flag"`
	MetaNamePunishFlag          int64    `json:"meta_name_punish_flag"`
	MetaNameInvestigationStatus int64    `json:"meta_name_investigation_status"`
	ActionList                  []string `json:"action_list"`
}

// ImageMedia 图片元数据。
type ImageMedia struct {
	Width    int64  `json:"width"`
	Height   int64  `json:"height"`
	Exif     string `json:"exif"`
	ImageName string `json:"image_name"`
}

// VideoMedia 视频元数据。
type VideoMedia struct {
	Duration    float64                 `json:"duration"`
	Width       int64                   `json:"width"`
	Height      int64                   `json:"height"`
	Title       string                  `json:"title"`
	// 注意：服务端返回的是数组（一个视频可能有多个流）。
	VideoStream []VideoMediaVideoStream `json:"video_media_video_stream"`
	AudioStream []VideoMediaAudioStream `json:"video_media_audio_stream"`
}

// VideoMediaVideoStream 视频流信息。
// 注意：阿里云盘对这些字段类型不固定（fps 可能是字符串也可能数字），
// 故数值字段用 any 兼容。
type VideoMediaVideoStream struct {
	Codec   any `json:"codec"`
	Bitrate any `json:"bitrate"`
	Fps     any `json:"fps"`
}

// VideoMediaAudioStream 音频流信息。
type VideoMediaAudioStream struct {
	Codec   any `json:"codec"`
	Bitrate any `json:"bitrate"`
}

// VideoPreview 视频预览元数据。
type VideoPreview struct {
	Category string `json:"category"`
	Status   string `json:"status"`
}

// IsFolder 判断是否文件夹。
func (f *BaseFile) IsFolder() bool { return f != nil && f.Type == FileTypeFolder }

// IsFile 判断是否普通文件。
func (f *BaseFile) IsFile() bool { return f != nil && f.Type == FileTypeFile }

// UploadPartInfo 上传分片信息。
type UploadPartInfo struct {
	PartNumber        int    `json:"part_number"`
	UploadURL         string `json:"upload_url,omitempty"`
	InternalUploadURL string `json:"internal_upload_url,omitempty"`
	ETag              string `json:"etag,omitempty"`
	PartSize          int64  `json:"part_size,omitempty"`
	ContentType       string `json:"content_type,omitempty"`
}

// FilePathItem 文件路径上的某一级。
type FilePathItem = BaseFile
