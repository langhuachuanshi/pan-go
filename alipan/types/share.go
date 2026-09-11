package types

// 本文件定义分享相关数据结构，对应该 aligo 的 ShareLinkBaseFile / ShareLinkSchema / BaseShareFile / ShareItemInfo 等。

// BaseShareFile 分享内的文件对象（列分享文件/搜索/取文件返回）。
type BaseShareFile struct {
	ShareID    string `json:"share_id"`
	FileID     string `json:"file_id"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Creator    string `json:"creator"`
	Description string `json:"description"`
	Category   FileCategory `json:"category"`
	DownloadURL string `json:"download_url"`
	FileExtension string `json:"file_extension"`
	Thumbnail  string `json:"thumbnail"`
	Type       FileType `json:"type"`
	UpdatedAt  string `json:"updated_at"`
	CreatedAt  string `json:"created_at"`
	URL        string `json:"url"`
	ParentFileID string `json:"parent_file_id"`
	Selected   bool   `json:"selected"`
	PunishFlag int64  `json:"punish_flag"`
	ActionList []string `json:"action_list"`
	DriveID    string `json:"drive_id"`
	DomainID   string `json:"domain_id"`
	RevisionID string `json:"revision_id"`
	Starred    bool   `json:"starred"`
	ContentHash string `json:"content_hash"`
	TrashedAt  string `json:"trashed_at"`
	FromShareID string `json:"from_share_id"`
}

// ShareLinkBaseFile 创建分享响应里的 first_file（字段与 BaseFile 基本一致）。
type ShareLinkBaseFile = BaseFile

// ShareLinkSchema 列出"我的分享"返回项。
type ShareLinkSchema struct {
	ShareID         string   `json:"share_id"`
	ShareName       string   `json:"share_name"`
	SharePwd        string   `json:"share_pwd"`
	Expiration      string   `json:"expiration"` // RFC3339，空=永久
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	Creator         string   `json:"creator"`
	Description     string   `json:"description"`
	DownloadCount   int64    `json:"download_count"`
	PreviewCount    int64    `json:"preview_count"`
	SaveCount       int64    `json:"save_count"`
	DriveID         string   `json:"drive_id"`
	Expired         bool     `json:"expired"`
	FileID          string   `json:"file_id"`
	FileIDList      []string `json:"file_id_list"`
	ShareMsg        string   `json:"share_msg"`
	SharePolicy     SharePolicy `json:"share_policy"`
	ShareURL        string   `json:"share_url"`
	Status          string   `json:"status"`
	FirstFile       *ShareLinkBaseFile `json:"first_file"`
	IsSubscribed    bool     `json:"is_subscribed"`
	NumOfSubscribers int64 `json:"num_of_subscribers"`
	DisplayName      string `json:"display_name"`
	// sync_status 服务端可能返回 number 或 string，用 any 兼容。
	CurrentSyncStatus any `json:"current_sync_status"`
	NextSyncStatus    any `json:"next_sync_status"`
	FullShareMsg      string `json:"full_share_msg"`
	ExStatus          any `json:"ex_status"`
	Popularity        any `json:"popularity"`
	PopularityStr     string `json:"popularity_str"`
}

// ShareItemInfo 匿名查询分享信息返回的文件摘要。
type ShareItemInfo struct {
	Category      FileCategory `json:"category"`
	FileExtension string       `json:"file_extension"`
	FileID        string       `json:"file_id"`
	FileName      string       `json:"file_name"`
	Thumbnail     string       `json:"thumbnail"`
	Type          FileType     `json:"type"`
}

// ShareToken 表示从 get_share_token 拿到的令牌，后续访问他人分享需要放在 x-share-token header。
type ShareToken struct {
	Token      string `json:"share_token"`
	ExpireTime string `json:"expire_time"`
	ExpiresIn  int64  `json:"expires_in"`
	// 以下字段由库回填，方便后续调用直接复用。
	ShareID string `json:"-"`
	SharePwd string `json:"-"`
}
