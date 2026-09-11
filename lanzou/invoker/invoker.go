// Package invoker 定义 lanzou 各业务子包共享的 HTTP 调用接口与哨兵错误。
//
// 设计同 alipan / baidu / quark：主包 Client 实现 Invoker，业务子包只依赖接口，
// 避免循环依赖。哨兵错误也定义在此处，主包lanzou以别名透出（调用方仍写 lanzou.ErrXxx）。
package invoker

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// 哨兵错误。
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

// Invoker 业务子包依赖的调用接口，由主包 Client 实现。
//
// TaskURL / UploadURL / AjaxmURL 由主包拼好（含 uid 参数），子包不感知域名细节；
// 需要 vei 参数的接口先调 Vei()（内部懒初始化 uid/vei）。
type Invoker interface {
	// LoggedIn 返回是否已登录。
	LoggedIn() bool
	// UserID 返回用户 ID（ylogin cookie，懒提取）。
	UserID() string
	// Vei 返回 anti-CSRF token（懒初始化：首次调用会抓取 mydisk.php 提取）。
	Vei() string
	// TaskURL 返回 doupload.php 完整地址（带 uid），并懒初始化 uid/vei。
	TaskURL() string
	// UploadURL 返回 html5up.php 上传入口。
	UploadURL() string
	// AjaxmURL 返回 ajaxm.php 完整地址（带 uid）。
	AjaxmURL() string

	// Get 发 GET 请求并注入会话 cookie。
	Get(rawURL string, headers map[string]string) ([]byte, http.Header, error)
	// Post 发表单 POST 请求并注入会话 cookie。
	Post(rawURL string, data map[string]string, headers map[string]string) ([]byte, http.Header, error)
	// PostMultipart 发 multipart POST（一次性载入内存）。
	PostMultipart(rawURL string, fields map[string]string, fileField, fileName string, fileReader io.Reader, headers map[string]string) ([]byte, http.Header, error)
	// PostMultipartStream 流式 multipart POST（io.Pipe 边读边发，onProgress 反映真实网络进度）。
	PostMultipartStream(rawURL string, fields map[string]string, fileField, fileName string, fileReader io.Reader, fileSize int64, onProgress func(uploaded, total int64), headers map[string]string) ([]byte, http.Header, error)
	// FetchPageWithChallenge 请求页面并自动处理 acw_sc__v2 JS 挑战。
	FetchPageWithChallenge(pageURL string) (string, error)

	// HTTPClient 返回底层客户端（下载直链等场景自建请求用）。
	HTTPClient() *http.Client
	// UserAgent 返回默认 UA（下载直链请求需带同款 UA）。
	UserAgent() string
	// MaxDownloadCount 返回下载并发上限。
	MaxDownloadCount() int
	// MaxSize 返回单文件大小上限（字节）。
	MaxSize() int
	// UploadDelay 返回上传延迟范围（毫秒）。
	UploadDelay() (minMs, maxMs int)
}

// CheckZT 检查 doupload.php 通用响应：zt=0 视为失败（info 为原因）。
func CheckZT(body []byte) error {
	var resp struct {
		Zt   int    `json:"zt"`
		Info string `json:"info"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("%w: invalid response", ErrAPIError)
	}
	if resp.Zt == 0 {
		return fmt.Errorf("%w: %s", ErrAPIError, resp.Info)
	}
	return nil
}
