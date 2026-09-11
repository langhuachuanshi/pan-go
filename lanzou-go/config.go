package lanzou

// API 域名常量
const (
	baseURLPC      = "https://pc.woozooo.com"
	baseURLUpload  = "https://up.woozooo.com"
	baseURLAccount = "https://accounts.woozooo.com" // 账号系统（登录已从 pc.woozooo.com 迁出）
)

// API 端点路径（新版统一用 /doupload.php）
const (
	pathLogin        = "/account/loginajax" // 已废弃：蓝奏账号系统迁移，登录改走 accounts.woozooo.com
	pathAccountLogin = "/accounts.php"      // 新账号系统登录入口（POST task=uselogin）
	pathLogout       = "/account/logout"
	pathUpload       = "/html5up.php" // 上传入口（fileup.php 已弃用，返回 HTML，见 issue #110）
	pathTaskAPI      = "/doupload.php" // 文件/文件夹/回收站操作统一入口
	pathAjaxm        = "/ajaxm.php"    // 直链解析
)

// 默认 User-Agent
const defaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

// 蓝奏云分享链接域名模式
var lanzouDomains = []string{
	"lanzoul.com",
	"lanzoui.com",
	"lanzout.com",
	"lanzouw.com",
	"lanzoux.com",
	"lanzouy.com",
	"lanzouf.com",
	"lanzouh.com",
	"lanzouj.com",
	"lanzouk.com",
	"lanzoup.com",
	"lanzouq.com",
	"lanzous.com",
}

// 默认配置
const (
	defaultMaxSize    = 100 * 1024 * 1024
	defaultTimeout    = 30
	defaultMaxDLCount = 3
	defaultVei        = "UFRQUlBWVggGBAdX" // 默认 vei 占位值（运行时会从页面动态获取）
)

// ChallengeConfig acw_sc__v2 JS 挑战参数配置
// 蓝奏云会定期更换混淆JS，届时需要更新这些参数
type ChallengeConfig struct {
	Perm   [40]int `json:"perm"`
	XORKey string  `json:"xor_key"`
}

// DefaultChallengeConfig 返回默认的挑战参数（2025年有效）
func DefaultChallengeConfig() *ChallengeConfig {
	return &ChallengeConfig{
		Perm: [40]int{
			15, 35, 29, 24, 33, 16, 1, 38, 10, 9,
			19, 31, 40, 27, 22, 23, 25, 13, 6, 11,
			39, 18, 20, 8, 14, 21, 32, 26, 2, 30,
			7, 4, 17, 5, 3, 28, 34, 37, 12, 36,
		},
		XORKey: "3000176000856006061501533003690027800375",
	}
}
