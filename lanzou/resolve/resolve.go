// Package resolve 直链解析（无需登录）：分享链接 → 真实下载直链 / 文件详情。
package resolve

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	WpSign     string
	Fid        int
	AjaxmURL   string   // 旧页面：相对端点 /ajaxm.php?file=N
	AjaxURLs   []string // 新页面：绝对端点候选（domain1/domain2）
	WebSignKey string   // websignkey/signs 动态值（新页面 var ajaxdata；空=旧页面硬编码 erCO）
	Kdns       int      // kd 参数：页面 var kdns（1=killdns 已加载，0=未加载）
}

// kdnsFallbackDomain 是 kdns=0 时页面固定使用的下载域名。
const kdnsFallbackDomain = "https://developer2oss.lanzouc.com:661"

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
	mainHTML, err := s.inv.FetchPageWithChallengeCtx(ctx, shareURL)
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
	iframeHTML, err := s.inv.FetchPageWithChallengeCtx(ctx, iframeURL)
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

	mainHTML, err := s.inv.FetchPageWithChallengeCtx(ctx, shareURL)
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
	iframeHTML, err := s.inv.FetchPageWithChallengeCtx(ctx, iframeURL)
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

// hedgeDelay 首个 ajax 端点发出后等多久没结果才补发其余候选。
// 单跳正常 RTT ~100-250ms，400ms 只在真慢时才触发；平静网络不多发一个请求。
const hedgeDelay = 400 * time.Millisecond

// requestDownloadURL POST ajax 端点（旧 /ajaxm.php / 新 /ajaxfile.php）获取真实下载链接。
// 2026-11 实测：新页面把端点写成绝对地址（domain1=apifile.woozooo.com、domain2=apifile.lanzouw.com），
// 旧的相对路径写法继续兼容。
//
// 候选端点用延迟对冲：只先发第一个，hedgeDelay 内没结果（或它快速失败）才补发其余，
// 先成功者胜。纯串行逐个重试时首个端点慢/挂会把整段解析拖到 5~10s（实测尾延迟大头）；
// 全并行则平静网络也多付一个请求的握手成本（实测冷启动 p50 反而 +80ms），故取中间。
// 终局结果（密码错/文件失效）与端点无关，一出现立即返回。
func (s *Service) requestDownloadURL(ctx context.Context, referer string, iframe *iframeData, fid int, pwd string) (string, error) {
	webSignKey := iframe.WebSignKey
	if webSignKey == "" {
		webSignKey = "erCO"
	}

	data := map[string]string{
		"action":     "downprocess",
		"websignkey": webSignKey,
		"signs":      webSignKey,
		"sign":       iframe.WpSign,
		"websign":    "",
		"kd":         strconv.Itoa(iframe.Kdns),
		"ves":        "1",
	}
	if pwd != "" {
		data["p"] = pwd
	}

	candidates := append([]string(nil), iframe.AjaxURLs...)
	if iframe.AjaxmURL != "" {
		if strings.HasPrefix(iframe.AjaxmURL, "http") {
			candidates = append(candidates, iframe.AjaxmURL)
		} else {
			candidates = append(candidates, "https://"+getBaseHost(referer)+iframe.AjaxmURL)
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("%w: no ajax endpoint in iframe page", invoker.ErrExtractFailed)
	}

	// raceCtx：胜者确定后取消在途请求；out 缓冲 = 候选总数，输家退出时不阻塞、无泄漏。
	raceCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type outcome struct {
		durl     string
		err      error
		terminal bool // 成功或终局错误（密码错/文件失效），出现即返回
	}
	out := make(chan outcome, len(candidates))
	worker := func(api string) {
		var resp ajaxmResp
		err := s.inv.PostFormHeaders(raceCtx, api, data, map[string]string{
			"Referer":          referer,
			"X-Requested-With": "XMLHttpRequest",
		}, &resp)
		if err != nil {
			out <- outcome{err: fmt.Errorf("ajaxm request failed: %w", err)}
			return
		}
		if resp.Zt == 1 {
			dom := resp.Dom
			if iframe.Kdns == 0 {
				dom = kdnsFallbackDomain
			}
			out <- outcome{durl: dom + "/file/" + resp.Url, terminal: true}
			return
		}
		if strings.Contains(resp.Mess, "密码") {
			out <- outcome{err: invoker.ErrPasswordWrong, terminal: true}
			return
		}
		if resp.Zt == 2 {
			out <- outcome{err: invoker.ErrFileExpired, terminal: true}
			return
		}
		out <- outcome{err: fmt.Errorf("%w: %s", invoker.ErrAPIError, resp.Mess)}
	}

	// 补发器：hedgeDelay 到点或收到 kick（已发请求全数失败）时发出剩余候选。
	// raceCtx 已结束时为未发出的候选补记账结果，收集器不会挂等不存在的请求。
	rest := candidates[1:]
	kick := make(chan struct{}, 1)
	go func() {
		select {
		case <-raceCtx.Done():
		case <-time.After(hedgeDelay):
		case <-kick:
		}
		if raceCtx.Err() != nil {
			for range rest {
				out <- outcome{err: fmt.Errorf("%w: canceled before launch", invoker.ErrAPIError)}
			}
			return
		}
		for _, api := range rest {
			go worker(api)
		}
	}()
	go worker(candidates[0])

	// 收集器：终局结果（成功/密码错/文件失效）即返；在途全败且还有候选 → kick 快速补发。
	var lastErr error
	pending := 1 // 在途请求数，仅用于判断「全败快补发」，不追求精确
	for settled := 0; settled < len(candidates); settled++ {
		oc := <-out
		pending--
		if oc.terminal {
			cancel()
			if oc.durl != "" {
				return oc.durl, nil
			}
			return "", oc.err
		}
		lastErr = oc.err
		if pending <= 0 && len(rest) > 0 {
			select {
			case kick <- struct{}{}:
			default:
			}
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%w: every ajax endpoint failed", invoker.ErrAPIError)
	}
	return "", lastErr
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

// extractIframeData 解析 iframe 页（wp_sign / ajax 端点与参数）。
// 2026-09-12 蓝奏改动（实测）：ajax 端点从 /ajaxm.php 变为 /ajaxfile.php?file=<id>，
// 且 websignkey/signs 的值改为页面变量 ajaxdata（不再硬编码 erCO）。两种端点都兼容。
func extractIframeData(html string) (*iframeData, error) {
	data := &iframeData{}

	if m := reWpSign.FindStringSubmatch(html); len(m) > 1 {
		data.WpSign = m[1]
	} else {
		return nil, fmt.Errorf("%w: wp_sign not found", invoker.ErrExtractFailed)
	}

	// ajax 端点：旧 /ajaxm.php?file=N 或新 /ajaxfile.php?file=N（相对路径写法）
	reAjax := regexp.MustCompile(`url\s*:\s*'(/ajax(?:m|file)\.php\?file=(\d+))'`)
	if m := reAjax.FindStringSubmatch(html); len(m) > 2 {
		data.AjaxmURL = m[1]
		fmt.Sscanf(m[2], "%d", &data.Fid)
	}

	// 2026-11 实测：新页面端点是绝对地址，domain1（apifile.woozooo.com）、domain2（apifile.lanzouw.com）
	reDomain := regexp.MustCompile(`var\s+domain[12]\s*=\s*'([^']+)'`)
	for _, m := range reDomain.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 && strings.HasPrefix(m[1], "http") {
			data.AjaxURLs = append(data.AjaxURLs, m[1])
		}
	}
	if m := regexp.MustCompile(`var\s+kdns\s*=\s*(\d+)`).FindStringSubmatch(html); len(m) > 1 {
		fmt.Sscanf(m[1], "%d", &data.Kdns)
	}

	// websignkey/signs 动态值：var ajaxdata = 'xxx'（旧页面无此变量则留空用旧硬编码）
	if m := regexp.MustCompile(`var\s+ajaxdata\s*=\s*'([^']*)'`).FindStringSubmatch(html); len(m) > 1 {
		data.WebSignKey = m[1]
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
