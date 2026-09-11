package alipan

import (
	"time"

	"github.com/langhuachuanshi/alipan-go/alipan/auth"
	"github.com/langhuachuanshi/alipan-go/alipan/types"
)

// options 是 New 的内部配置容器。
type options struct {
	name            string
	configDir       string
	refreshToken    string
	token           *types.Token
	defaultDriveID  string
	loginMethod     auth.LoginMethod
	webPort         int
	loginTimeout    time.Duration
	showQR          func(string) error
	requestInterval time.Duration
	retryMax        int
}

func defaultOptions() *options {
	return &options{
		name:        "aligo",
		loginMethod: auth.LoginTerminal,
	}
}

// Option 是 New 的配置函数。对应 aligo Aligo.__init__ 的参数。
type Option func(*options)

// WithName 设置配置文件名（默认 "aligo"，对应 ~/.aligo/aligo.json）。多账号用不同 name 区分。
func WithName(name string) Option { return func(o *options) { o.name = name } }

// WithConfigDir 设置配置目录（默认 ~/.aligo）。
func WithConfigDir(dir string) Option { return func(o *options) { o.configDir = dir } }

// WithRefreshToken 直接用 refresh_token 登录。
func WithRefreshToken(token string) Option { return func(o *options) { o.refreshToken = token } }

// WithToken 直接传入已构造的 Token 对象。
func WithToken(t *types.Token) Option { return func(o *options) { o.token = t } }

// WithTokenFile 指定从配置文件加载登录态。等价于 WithName。
func WithTokenFile(name string) Option { return func(o *options) { o.name = name } }

// WithDefaultDriveID 设置默认网盘 ID。
func WithDefaultDriveID(id string) Option { return func(o *options) { o.defaultDriveID = id } }

// WithQRCodeTerminal 启用终端二维码扫码登录（默认方式）。
func WithQRCodeTerminal() Option { return func(o *options) { o.loginMethod = auth.LoginTerminal } }

// WithQRCodeWeb 启用网页二维码扫码登录，起本地 HTTP 服务监听 port。
func WithQRCodeWeb(port int) Option {
	return func(o *options) {
		o.loginMethod = auth.LoginWeb
		o.webPort = port
	}
}

// WithRequestInterval 设置每次请求前的 sleep（防风控）。
func WithRequestInterval(d time.Duration) Option { return func(o *options) { o.requestInterval = d } }

// WithRetryMax 设置最大重试次数。
func WithRetryMax(n int) Option { return func(o *options) { o.retryMax = n } }

// WithLoginTimeout 设置扫码登录超时（默认 5 分钟）。
func WithLoginTimeout(d time.Duration) Option { return func(o *options) { o.loginTimeout = d } }

// WithShowQR 自定义二维码展示回调。回调接收二维码内容字符串，自行渲染/保存/展示。
// 适用于终端块状二维码扫不出的场景（如保存为 PNG 图片供手机扫描）。
func WithShowQR(fn func(content string) error) Option { return func(o *options) { o.showQR = fn } }
