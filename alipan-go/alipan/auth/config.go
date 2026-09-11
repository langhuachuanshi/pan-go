// Package auth 实现阿里云盘的登录认证，对应该 aligo 的 src/aligo/core/Auth.py。
//
// 职责：扫码登录（终端/网页）、refresh_token 直登、token 刷新、token 持久化。
// 不持有长期 HTTP 状态——它产出 *types.Token，由主包 Client 负责后续请求。
package auth

import (
	"time"

	"github.com/langhuachuanshi/alipan-go/alipan/types"
)

// 阿里云盘 host 与常量（对应该 aligo Config.py）。
const (
	HostAPI     = "https://api.aliyundrive.com"
	HostAuth    = "https://auth.aliyundrive.com"
	HostPassport = "https://passport.aliyundrive.com"

	ClientID      = "25dzX3vbYqktVxyX"
	OAuthRedirect = "https://www.aliyundrive.com/sign/callback"
	AppName       = "aliyun_drive"

	PathAccountToken   = "/v2/account/token"
	PathOAuthAuthorize = "/v2/oauth/authorize"
	PathQrcodeGenerate = "/newlogin/qrcode/generate.do"
	PathQrcodeQuery    = "/newlogin/qrcode/query.do"
)

// 固定 header（对应该 aligo UNI_HEADERS）。
const (
	UserAgent = "AliApp(AYSD/5.8.0) com.alicloud.databox/37029260 Channel/36176927979800@rimet_android_5.8.0 language/zh-CN /Android Mobile/Xiaomi Redmi"
	XCanary   = "client=Android,app=adrive,version=v5.8.0"
	// XSignature 取自 aligo 6.2.8 的 _X_SIGNATURE 常量（两段拼接）。
	// 注意：旧版 aligo 源码值不同，必须用这个精确值，否则分享等接口会 403。
	XSignature = "f4b7bed5d8524a04051bd2da876dd79afe922b8205226d65855d02b267422adb1e0d8a816b021eaf5c36d101892180f79df655c5712b348c2a540ca136e6b22001"
)

// LoginMethod 扫码登录的展示方式。
type LoginMethod int

const (
	LoginTerminal LoginMethod = iota // 终端打印二维码
	LoginWeb                         // 网页二维码
)

// Config auth 的配置。
type Config struct {
	Name          string        // 配置文件名（~/.aligo/<name>.json）
	ConfigDir     string        // 配置目录，默认 ~/.aligo
	RefreshToken  string        // 直接用 refresh_token 登录
	Token         *types.Token  // 直接传入 token
	DefaultDrive  string        // 默认网盘 ID
	LoginMethod   LoginMethod
	WebPort       int
	LoginTimeout  time.Duration // 扫码登录超时，默认 5 分钟
	// ShowQR 自定义二维码展示回调。为 nil 时按 LoginMethod 走终端/网页默认实现。
	// content 是二维码原始内容字符串（阿里自定义 qr 协议），由回调负责渲染/保存/展示。
	ShowQR func(content string) error
}

// Result 是登录后的产物。
type Result struct {
	Token    *types.Token
	DeviceID string
}
