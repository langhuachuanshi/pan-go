package types

// 本文件定义夸克分享相关数据模型。

// CreateShareResponse 创建分享响应。
// 完整创建需 3 步，最终在 /share/password 返回 share_url。
type CreateShareResponse struct {
	ShareID    string `json:"share_id"`     // 32位内部 ID
	PwdID      string `json:"pwd_id"`       // 12位短链 ID（分享链接用的）
	ShareURL   string `json:"share_url"`    // 完整分享链接 https://pan.quark.cn/s/<pwd_id>
	Title      string `json:"title"`
	URLType    int    `json:"url_type"`     // 1=公开 2=私密
	ExpiredType int   `json:"expired_type"` // 1=永久
	ExpiredAt  int64  `json:"expired_at"`   // 过期时间戳(ms)，4102416000000≈永久
	Passcode   string `json:"passcode"`     // 提取码（私密时）
}

// ExpiredType 常量。
const (
	ExpiredForever int = 1 // 永久有效
	Expired1Day    int = 2 // 1天
	Expired7Day    int = 3 // 7天
	Expired30Day   int = 4 // 30天
)
