package types

// 本文件定义 Token 数据类型，对应该 aligo 的 src/aligo/types/Token.py。
//
// x_device_id 是 aligo 自己塞进去的扩展字段（非阿里返回），用于持久化 device id。

// Token 表示阿里云盘账户凭证。
type Token struct {
	UserName           *string         `json:"user_name,omitempty"`
	NickName           *string         `json:"nick_name,omitempty"`
	UserID             *string         `json:"user_id,omitempty"`
	DefaultDriveID     *string         `json:"default_drive_id,omitempty"`
	DefaultSboxDriveID *string         `json:"default_sbox_drive_id,omitempty"`
	Role               *string         `json:"role,omitempty"`
	Status             *string         `json:"status,omitempty"`
	AccessToken        *string         `json:"access_token,omitempty"`
	RefreshToken       *string         `json:"refresh_token,omitempty"`
	ExpiresIn          *int64          `json:"expires_in,omitempty"`
	TokenType          *string         `json:"token_type,omitempty"`
	Avatar             *string         `json:"avatar,omitempty"`
	ExpireTime         *string         `json:"expire_time,omitempty"`
	State              *string         `json:"state,omitempty"`
	ExistLink          []any           `json:"exist_link,omitempty"`
	NeedLink           *bool           `json:"need_link,omitempty"`
	UserData           map[string]any  `json:"user_data,omitempty"`
	PinSetup           *bool           `json:"pin_setup,omitempty"`
	IsFirstLogin       *bool           `json:"is_first_login,omitempty"`
	NeedRpVerify       *bool           `json:"need_rp_verify,omitempty"`
	DeviceID           *string         `json:"device_id,omitempty"`
	DomainID           *string         `json:"domain_id,omitempty"`
	HloginURL          *string         `json:"hlogin_url,omitempty"`
	XDeviceID          *string         `json:"x_device_id,omitempty"`
	PathStatus         *string         `json:"path_status,omitempty"`
}

// GetAccessToken 返回 access_token（可能为空）。
func (t *Token) GetAccessToken() string {
	if t == nil || t.AccessToken == nil {
		return ""
	}
	return *t.AccessToken
}

// GetRefreshToken 返回 refresh_token（可能为空）。
func (t *Token) GetRefreshToken() string {
	if t == nil || t.RefreshToken == nil {
		return ""
	}
	return *t.RefreshToken
}

// GetDefaultDriveID 返回默认网盘 ID。
func (t *Token) GetDefaultDriveID() string {
	if t == nil || t.DefaultDriveID == nil {
		return ""
	}
	return *t.DefaultDriveID
}

// GetUserIDStr 返回 user_id。
func (t *Token) GetUserIDStr() string {
	if t == nil || t.UserID == nil {
		return ""
	}
	return *t.UserID
}
