// Package resolve 直链解析（无需登录）：分享链接 → 真实下载直链 / 文件详情。
package resolve

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/langhuachuanshi/pan-go/lanzou/invoker"
)

// 蓝奏云分享链接域名
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

// FileDetail 文件详情（含直链）
type FileDetail struct {
	FileID      string `json:"file_id"`
	NameAll     string `json:"name_all"` // 文件名
	Size        string `json:"size"`     // 文件大小
	UploadTime  string `json:"time"`     // 上传时间
	DownloadURL string `json:"url"`      // 分享链接
	DURL        string `json:"durl"`     // 直链
	Description string `json:"des"`      // 描述
	IsNewd      int    `json:"is_newd"`
}

// 页面解析正则
var (
	reWpSign       = regexp.MustCompile(`wp_sign\s*=\s*'([^']+)'`)
	reIframeSrc    = regexp.MustCompile(`<iframe[^>]+src="(/fn\?[^"]+)"`)
	reFid          = regexp.MustCompile(`var\s+fid\s*=\s*(\d+)`)
	reTitle        = regexp.MustCompile(`<title>([^<]+)</title>`)
	reFileSizeDesc = regexp.MustCompile(`文件大小[：:]\s*([^<\s]+(?:\s*[A-Za-z]+)?)`)
	reFileSizeMeta = regexp.MustCompile(`文件大小[：:]\s*(\d+\.?\d*\s*[A-Za-z]+)`)
)

// pageData 分享主页解析结果
type pageData struct {
	Fid      int
	Title    string
	FileSize string
	Iframe   string
}

// iframeData iframe 页解析结果
type iframeData struct {
	WpSign   string
	Fid      int
	AjaxmURL string
}

// ajaxmResp ajaxm.php 响应
type ajaxmResp struct {
	Zt   int    `json:"zt"`
	Dom  string `json:"dom"`
	Url  string `json:"url"`
	Mess string `json:"mess"`
}

// Service 直链解析服务。
type Service struct{ inv invoker.Invoker }

// New 创建 resolve Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// GetDurlByURL 通过分享链接获取直链（无需登录）。
// shareURL: 蓝奏云分享链接, pwd: 访问密码（可空）。
func (s *Service) GetDurlByURL(ctx context.Context, shareURL, pwd string) (string, error) {
	shareURL = normalizeURL(shareURL)
	if !isLanzouURL(shareURL) {
		return "", invoker.ErrInvalidURL
	}

	// Step 1: 请求分享页面（处理JS挑战）
	mainHTML, err := s.inv.FetchPageWithChallenge(shareURL)
	if err != nil {
		return "", fmt.Errorf("fetch share page failed: %w", err)
	}

	// Step 2: 提取页面数据（fid、文件名、iframe src）
	page, err := extractPageData(mainHTML)
	if err != nil {
		return "", fmt.Errorf("extract page data failed: %w", err)
	}

	// Step 3: 请求iframe页面获取 wp_sign
	iframeURL := "https://" + getBaseHost(shareURL) + page.Iframe
	iframeHTML, err := s.inv.FetchPageWithChallenge(iframeURL)
	if err != nil {
		return "", fmt.Errorf("fetch iframe failed: %w", err)
	}

	iframe, err := extractIframeData(iframeHTML)
	if err != nil {
		return "", fmt.Errorf("extract iframe data failed: %w", err)
	}

	// Step 4: POST ajaxm.php 获取直链
	return s.requestDownloadURL(ctx, shareURL, iframe, page.Fid, pwd)
}

// GetDurlByFolderURL 通过文件夹分享链接获取所有文件的直链。
func (s *Service) GetDurlByFolderURL(ctx context.Context, folderURL, pwd string, subdir bool) ([]string, error) {
	folderURL = normalizeURL(folderURL)
	if !isLanzouURL(folderURL) {
		return nil, invoker.ErrInvalidURL
	}

	// 文件夹页面逻辑与单文件不同，先尝试直接解析
	durl, err := s.GetDurlByURL(ctx, folderURL, pwd)
	if err != nil {
		return nil, err
	}
	return []string{durl}, nil
}

// GetDurlByURLAndFolder 带文件夹参数的直链解析。
// 注意：folderID 参数当前未使用（历史兼容），行为与 GetDurlByURL 一致。
func (s *Service) GetDurlByURLAndFolder(ctx context.Context, shareURL, pwd, folderID string) (string, error) {
	return s.GetDurlByURL(ctx, shareURL, pwd)
}

// GetFileInfo 通过分享链接获取文件详细信息（含直链）。
func (s *Service) GetFileInfo(ctx context.Context, shareURL, pwd string) (*FileDetail, error) {
	shareURL = normalizeURL(shareURL)
	if !isLanzouURL(shareURL) {
		return nil, invoker.ErrInvalidURL
	}

	mainHTML, err := s.inv.FetchPageWithChallenge(shareURL)
	if err != nil {
		return nil, fmt.Errorf("fetch share page failed: %w", err)
	}

	page, err := extractPageData(mainHTML)
	if err != nil {
		return nil, fmt.Errorf("extract page data failed: %w", err)
	}

	detail := &FileDetail{
		NameAll:     page.Title,
		Size:        page.FileSize,
		DownloadURL: shareURL,
	}

	iframeURL := "https://" + getBaseHost(shareURL) + page.Iframe
	iframeHTML, err := s.inv.FetchPageWithChallenge(iframeURL)
	if err != nil {
		return detail, nil // 返回部分信息
	}

	iframe, err := extractIframeData(iframeHTML)
	if err != nil {
		return detail, nil // 返回部分信息
	}
	detail.FileID = fmt.Sprintf("%d", page.Fid)

	durl, err := s.requestDownloadURL(ctx, shareURL, iframe, page.Fid, pwd)
	if err != nil {
		return detail, nil // 返回部分信息
	}
	detail.DURL = durl

	return detail, nil
}

// FileInfoByURL 兼容别名，等价于 GetFileInfo。
func (s *Service) FileInfoByURL(ctx context.Context, shareURL, pwd string) (*FileDetail, error) {
	return s.GetFileInfo(ctx, shareURL, pwd)
}

// ===== 内部实现 =====

// requestDownloadURL POST ajaxm.php 获取真实下载链接。
func (s *Service) requestDownloadURL(ctx context.Context, referer string, iframe *iframeData, fid int, pwd string) (string, error) {
	ajaxmURL := "https://" + getBaseHost(referer) + iframe.AjaxmURL

	data := map[string]string{
		"action":     "downprocess",
		"websignkey": "erCO",
		"signs":      "erCO",
		"sign":       iframe.WpSign,
		"websign":    "",
		"kd":         "0",
		"ves":        "1",
	}
	if pwd != "" {
		data["p"] = pwd
	}

	var resp ajaxmResp
	err := s.inv.PostFormHeaders(ctx, ajaxmURL, data, map[string]string{
		"Referer":          referer,
		"X-Requested-With": "XMLHttpRequest",
	}, &resp)
	if err != nil {
		return "", fmt.Errorf("ajaxm request failed: %w", err)
	}

	if resp.Zt != 1 {
		if strings.Contains(resp.Mess, "密码") {
			return "", invoker.ErrPasswordWrong
		}
		if resp.Zt == 2 {
			return "", invoker.ErrFileExpired
		}
		return "", fmt.Errorf("%w: %s", invoker.ErrAPIError, resp.Mess)
	}

	// 拼接直链: dom + /file/ + url
	return resp.Dom + "/file/" + resp.Url, nil
}

// ===== 页面解析 =====

// extractPageData 解析分享主页（fid / 文件名 / iframe src）
func extractPageData(html string) (*pageData, error) {
	data := &pageData{}

	if m := reFid.FindStringSubmatch(html); len(m) > 1 {
		fmt.Sscanf(m[1], "%d", &data.Fid)
	}
	if m := reTitle.FindStringSubmatch(html); len(m) > 1 {
		title := strings.TrimSpace(m[1])
		title = strings.TrimSuffix(title, " - 蓝奏云")
		title = strings.TrimSuffix(title, " - 蓝奏云盘")
		data.Title = title
	}
	if m := reFileSizeDesc.FindStringSubmatch(html); len(m) > 1 {
		data.FileSize = strings.TrimSpace(m[1])
	} else if m := reFileSizeMeta.FindStringSubmatch(html); len(m) > 1 {
		data.FileSize = strings.TrimSpace(m[1])
	}
	if m := reIframeSrc.FindStringSubmatch(html); len(m) > 1 {
		data.Iframe = m[1]
	}

	if data.Fid == 0 && data.Iframe == "" {
		return nil, fmt.Errorf("%w: no fid or iframe found", invoker.ErrExtractFailed)
	}
	return data, nil
}

// extractIframeData 解析 iframe 页（wp_sign / ajaxm 地址）
func extractIframeData(html string) (*iframeData, error) {
	data := &iframeData{}

	if m := reWpSign.FindStringSubmatch(html); len(m) > 1 {
		data.WpSign = m[1]
	} else {
		return nil, fmt.Errorf("%w: wp_sign not found", invoker.ErrExtractFailed)
	}

	reAjaxm := regexp.MustCompile(`url\s*:\s*'(/ajaxm\.php\?file=(\d+))'`)
	if m := reAjaxm.FindStringSubmatch(html); len(m) > 2 {
		data.AjaxmURL = m[1]
		fmt.Sscanf(m[2], "%d", &data.Fid)
	}
	return data, nil
}

// ===== URL 工具 =====

func isLanzouURL(rawURL string) bool {
	for _, domain := range lanzouDomains {
		if strings.Contains(rawURL, domain) {
			return true
		}
	}
	return strings.Contains(rawURL, "lanzou") || strings.Contains(rawURL, "woozooo")
}

func normalizeURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	return strings.TrimSuffix(rawURL, "/")
}

func getBaseHost(rawURL string) string {
	rawURL = strings.TrimPrefix(rawURL, "https://")
	rawURL = strings.TrimPrefix(rawURL, "http://")
	if idx := strings.Index(rawURL, "/"); idx > 0 {
		return rawURL[:idx]
	}
	return rawURL
}
