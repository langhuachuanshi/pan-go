// Package invoker 定义 lanzou 业务子包依赖的调用接口与哨兵错误。
//
// 接口 = core 标准菜单 + lanzou 方言扩展（挑战页 / 流式上传 / 自定义头）；
// 实现方是主包 Client。lanzou 无数字码表，保留自有哨兵错误体系（errors.Is 判断）。
package invoker

import (
	"context"
	"errors"
	"io"
	"net/http"

	coreinvoker "github.com/langhuachuanshi/pan-go/core/invoker"
)

// 哨兵错误。调用方用 errors.Is 判断；API 层错误包装为 ErrAPIError。
var (
	ErrNotLoggedIn    = errors.New("lanzou: not logged in")
	ErrFileExpired    = errors.New("lanzou: file expired or deleted")
	ErrPasswordWrong  = errors.New("lanzou: wrong password")
	ErrFileSizeLimit  = errors.New("lanzou: file size exceeds limit")
	ErrInvalidURL     = errors.New("lanzou: invalid lanzou url")
	ErrExtractFailed  = errors.New("lanzou: failed to extract data from page")
	ErrUploadFailed   = errors.New("lanzou: upload failed")
	ErrDownloadFailed = errors.New("lanzou: download failed")
	ErrAPIError       = errors.New("lanzou: api error")
)

// 端点路径（相对 pc.woozooo.com；完整 URL 亦可传）。
const (
	PathTaskAPI  = "/doupload.php" // 文件/文件夹/回收站统一入口
	PathUpload   = "/html5up.php"  // 上传入口
	PathAjaxm    = "/ajaxm.php"    // 直链解析
)

// Invoker lanzou 业务子包依赖的接口：core 标准菜单 + lanzou 方言扩展。
type Invoker interface {
	coreinvoker.Invoker

	// —— 方言扩展 ——
	LoggedIn() bool
	Vei() string
	UserID() string
	// FetchPageWithChallenge 取页面 HTML，自动处理 acw_sc__v2 挑战。
	FetchPageWithChallenge(pageURL string) (string, error)
	// PostFormHeaders 带自定义头的表单 POST（ajaxm 需要 XHR 头）。
	PostFormHeaders(ctx context.Context, path string, form map[string]string, headers map[string]string, out any) error
	// PostMultipartStream 流式 multipart 上传（io.Pipe 边读边发，onProgress 为真实网络进度）。
	PostMultipartStream(ctx context.Context, path string, form map[string]string,
		field, filename string, file io.Reader, fileSize int64,
		onProgress func(uploaded, total int64), headers map[string]string, out any) error
	// HTTPClient 供大文件流式下载使用（core Get/Post 会整包缓存，不适合下载）。
	HTTPClient() *http.Client

	// —— 模块配置（上传/下载策略用） ——
	MaxSize() int
	MaxDownloadCount() int
	UploadDelay() (minMs, maxMs int)
}
