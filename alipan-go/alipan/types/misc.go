package types

// 本文件定义用户、网盘相关数据结构。
// 注意：BaseUser 的时间字段是 int 时间戳（毫秒），与 BaseFile 的 ISO8601 字符串不同。

// BaseUser 用户信息对象。
type BaseUser struct {
	UserName        string         `json:"user_name"`
	UserID          string         `json:"user_id"`
	DefaultDriveID  string         `json:"default_drive_id"`
	Description     string         `json:"description"`
	NickName        string         `json:"nick_name"`
	Email           string         `json:"email"`
	Phone           string         `json:"phone"`
	Role            string         `json:"role"`
	Status          string         `json:"status"`
	DomainID        string         `json:"domain_id"`
	Avatar          string         `json:"avatar"`
	UserData        map[string]any `json:"user_data"`
	Permission      string         `json:"permission"`
	Creator         string         `json:"creator"`
	DefaultLocation string         `json:"default_location"`
	PhoneRegion     string         `json:"phone_region"`
	PathStatus      string         `json:"path_status"`

	CreatedAt     int64 `json:"created_at"`
	UpdatedAt     int64 `json:"updated_at"`
	LastLoginTime int64 `json:"last_login_time"`
	ExpiredAt     int64 `json:"expired_at"`

	DenyChangePasswordBySelf    bool `json:"deny_change_password_by_self"`
	NeedChangePasswordNextLogin bool `json:"need_change_password_next_login"`
}

// BaseDrive 网盘对象。
type BaseDrive struct {
	DriveID           string `json:"drive_id"`
	DriveName         string `json:"drive_name"`
	DriveType         string `json:"drive_type"`
	UsedSize          int64  `json:"used_size"`
	TotalSize         int64  `json:"total_size"`
	Owner             string `json:"owner"`
	OwnerType         string `json:"owner_type"`
	Description       string `json:"description"`
	Creator           string `json:"creator"`
	DomainID          string `json:"domain_id"`
	Status            string `json:"status"`
	StoreID           string `json:"store_id"`
	RelativePath      string `json:"relative_path"`
	EncryptMode       string `json:"encrypt_mode"`
	EncryptDataAccess bool   `json:"encrypt_data_access"`
	Permission        string `json:"permission"`
	SubdomainID       string `json:"subdomain_id"`
	Category          string `json:"category"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

// DriveCapacityDetail 网盘容量详情。
type DriveCapacityDetail struct {
	DriveUsedSize           int64 `json:"drive_used_size"`
	DriveTotalSize          int64 `json:"drive_total_size"`
	DefaultDriveUsedSize    int64 `json:"default_drive_used_size"`
	AlbumDriveUsedSize      int64 `json:"album_drive_used_size"`
	ShareAlbumDriveUsedSize int64 `json:"share_album_drive_used_size"`
	NoteDriveUsedSize       int64 `json:"note_drive_used_size"`
	SboxDriveUsedSize       int64 `json:"sbox_drive_used_size"`
}

// DriveFile drive+file 标识对。
type DriveFile struct {
	DriveID string `json:"drive_id"`
	FileID  string `json:"file_id"`
}
