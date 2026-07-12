// Package download 实现百度网盘文件下载（网页端 / BDUSS 方案）。
//
// 支持两种下载方式：
//  1. PCS 直链：GET /rest/2.0/pcs/file?method=download&path=...&app_id=250528
//     最简单，只需 BDUSS cookie，直接返回文件流。
//  2. PanAPI 下载：POST /api/download?sign=...&timestamp=...&fidlist=[...]
//     需要 PanHome 签名（从 /disk/home 提取 sign1/sign3），通过 fs_id 下载。
//
// 鉴权靠 BDUSS cookie。PCS 直链适合按路径下载，PanAPI 适合按 fs_id 下载。
// 参考：BaiduPCS-Go。
package download

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/langhuachuanshi/baidupan-go/baidu/invoker"
	"github.com/langhuachuanshi/baidupan-go/baidu/panhome"
	"github.com/langhuachuanshi/baidupan-go/baidu/sign"
)

const (
	pcsBase = "https://pcs.baidu.com"   // PCS 接口域名
	panBase = "https://pan.baidu.com"   // 网页端接口域名
	appID   = "250528"                  // 百度网盘 app_id
)

// Service 下载入口。
type Service struct {
	inv     invoker.Invoker
	phCache *panhome.Cache // PanHome 签名缓存（PanAPI 下载用）
}

// New 创建 download Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// SetPanHomeCache 设置 PanHome 签名缓存（使用 PanAPI 下载前必须调用）。
// httpClient 应携带 BDUSS cookie。
func (s *Service) SetPanHomeCache(httpClient *http.Client) {
	s.phCache = panhome.NewCache(httpClient)
}

// DownloadRequest 下载请求。
type DownloadRequest struct {
	Path      string       // 文件绝对路径（PCS 直链方式）
	FSID      int64        // 文件 fs_id（PanAPI 方式，优先于 Path）
	Writer    io.Writer    // 下载内容写入目标
	OnProgress func(downloaded, total int64) // 可选进度回调
}

// —— 数据类型 ——

// DlinkInfo 下载链接信息（来自 PanAPI 响应）。
type DlinkInfo struct {
	Dlink string `json:"dlink"`
	FsID  string `json:"fs_id"`
}

type panAPIDownloadResp struct {
	Errno     int         `json:"errno"`
	Errmsg    string      `json:"err_msg"`
	DlinkList []DlinkInfo `json:"dlink"`
}

// GetDownloadURL 获取文件下载直链。
//
// 优先使用 PanAPI（需要 fs_id + PanHome 签名），获取 dlink。
// 如果未设置 PanHome 缓存或使用 Path 方式，返回 PCS 直链 URL。
//
// 返回的 URL 可直接用 HTTP GET 下载（需携带与 SDK 相同的 cookie）。
func (s *Service) GetDownloadURL(ctx context.Context, req *DownloadRequest) (string, error) {
	// PanAPI 方式（有 fs_id 且有 PanHome 缓存）
	if req.FSID != 0 && s.phCache != nil {
		dlink, err := s.getPanAPIDlink(ctx, req.FSID)
		if err == nil {
			return dlink, nil
		}
		// PanAPI 失败，回退到 PCS 直链
	}
	// PCS 直链方式
	if req.Path == "" {
		return "", fmt.Errorf("download: Path 和 FSID 不能同时为空")
	}
	return s.getPCSDirectURL(req.Path), nil
}

// Download 下载文件内容到 Writer。
//
// 使用 PCS 直链下载，需要配置了 BDUSS 的 http.Client。
// httpClient 应携带与 SDK 相同的 cookie（BDUSS）。
func (s *Service) Download(ctx context.Context, httpClient *http.Client, req *DownloadRequest) error {
	if req.Writer == nil {
		return fmt.Errorf("download: Writer 不能为空")
	}

	downloadURL, err := s.GetDownloadURL(ctx, req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("download: 创建下载请求失败: %w", err)
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	httpReq.Header.Set("Referer", "https://pan.baidu.com/disk/main")

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("download: 下载请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: 下载失败 HTTP %d", resp.StatusCode)
	}

	if req.OnProgress != nil {
		// 带进度回调的复制
		buf := make([]byte, 32*1024) // 32KB buffer
		var written int64
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				if _, writeErr := req.Writer.Write(buf[:n]); writeErr != nil {
					return fmt.Errorf("download: 写入失败: %w", writeErr)
				}
				written += int64(n)
				req.OnProgress(written, resp.ContentLength)
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return fmt.Errorf("download: 读取失败: %w", readErr)
			}
		}
	} else {
		if _, err := io.Copy(req.Writer, resp.Body); err != nil {
			return fmt.Errorf("download: 复制数据失败: %w", err)
		}
	}

	return nil
}

// getPCSDirectURL 构建 PCS 直链下载 URL。
func (s *Service) getPCSDirectURL(path string) string {
	q := url.Values{}
	q.Set("app_id", appID)
	q.Set("method", "download")
	q.Set("path", path)
	return pcsBase + "/rest/2.0/pcs/file?" + q.Encode()
}

// GetStreamURL 获取流媒体下载 URL（支持 Range 断点续传、音视频拖动）。
//
// 对应 PCS stream 接口，专为音视频流式播放优化：
//   GET /rest/2.0/pcs/stream?app_id=250528&method=download&path=...
//
// 返回的 URL 支持 HTTP Range 请求头，可用于：
//   - 视频播放器直接播放（VLC/mpv/IINA）
//   - 浏览器 <video> 标签（需处理 CORS）
//   - curl -r 范围下载
//
// 调用方需携带与 SDK 相同的 cookie 发起 GET 请求。
func (s *Service) GetStreamURL(path string) string {
	q := url.Values{}
	q.Set("app_id", appID)
	q.Set("method", "download")
	q.Set("path", path)
	return pcsBase + "/rest/2.0/pcs/stream?" + q.Encode()
}

// DownloadStream 流式下载文件内容（支持 Range 断点续传）。
//
// 与 Download 不同，此方法走 /pcs/stream 接口，支持：
//   - 通过 RangeStart/RangeEnd 指定下载范围（视频拖动）
//   - HTTP 206 Partial Content 响应
//
// RangeStart/RangeEnd 为 0 时下载完整文件。
func (s *Service) DownloadStream(ctx context.Context, httpClient *http.Client, path string, writer io.Writer, rangeStart, rangeEnd int64, onProgress func(downloaded, total int64)) error {
	if writer == nil {
		return fmt.Errorf("download: Writer 不能为空")
	}

	streamURL := s.GetStreamURL(path)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return fmt.Errorf("download: 创建流下载请求失败: %w", err)
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	httpReq.Header.Set("Referer", "https://pan.baidu.com/disk/main")

	if rangeStart > 0 || rangeEnd > 0 {
		rangeVal := fmt.Sprintf("bytes=%d-", rangeStart)
		if rangeEnd > 0 {
			rangeVal = fmt.Sprintf("bytes=%d-%d", rangeStart, rangeEnd)
		}
		httpReq.Header.Set("Range", rangeVal)
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("download: 流下载请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 206=Partial Content, 200=Full Content
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("download: 流下载失败 HTTP %d", resp.StatusCode)
	}

	if onProgress != nil {
		buf := make([]byte, 32*1024)
		var written int64
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				if _, writeErr := writer.Write(buf[:n]); writeErr != nil {
					return fmt.Errorf("download: 写入失败: %w", writeErr)
				}
				written += int64(n)
				onProgress(written, resp.ContentLength)
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return fmt.Errorf("download: 读取失败: %w", readErr)
			}
		}
	} else {
		if _, err := io.Copy(writer, resp.Body); err != nil {
			return fmt.Errorf("download: 复制数据失败: %w", err)
		}
	}

	return nil
}

// getPanAPIDlink 通过 PanAPI 获取 dlink。
func (s *Service) getPanAPIDlink(ctx context.Context, fsid int64) (string, error) {
	signRes, err := s.phCache.CacheSignature()
	if err != nil {
		return "", fmt.Errorf("获取 PanHome 签名失败: %w", err)
	}

	fidList := "[" + strconv.FormatInt(fsid, 10) + "]"
	body := map[string]string{
		"sign":      signRes.Sign,
		"timestamp": signRes.Timestamp,
		"fidlist":   fidList,
	}

	// 构建 URL，带上 bdstoken（需要鉴权）
	// POST 到 /api/download，body 是 form 参数
	q := url.Values{}
	q.Set("bdstoken", "") // 由 Client 自动注入

	fullURL := panBase + "/api/download"
	data, _, err := s.inv.PostFormRaw(ctx, fullURL, body)
	if err != nil {
		return "", fmt.Errorf("PanAPI 下载请求失败: %w", err)
	}

	var resp panAPIDownloadResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("解析 PanAPI 响应失败: %w", err)
	}
	if resp.Errno != 0 {
		// 签名过期（errno 112/113），重置缓存
		if resp.Errno == 112 || resp.Errno == 113 {
			s.phCache.Reset()
		}
		return "", invoker.NewAPIError(resp.Errno, resp.Errmsg)
	}
	if len(resp.DlinkList) == 0 {
		return "", fmt.Errorf("download: 未获取到下载链接")
	}

	return resp.DlinkList[0].Dlink, nil
}

// GetPCSLocateURL 获取 PCS locate download URL（需要 uid）。
// 此方法用于获取下载地址列表（多节点），需要提供百度用户 uid。
func (s *Service) GetPCSLocateURL(ctx context.Context, path string, uid uint64, bduss string) (string, error) {
	sig := sign.NewLocateDownloadSign(uid, bduss)

	q := url.Values{}
	q.Set("ant", "1")
	q.Set("check_blue", "1")
	q.Set("es", "1")
	q.Set("esl", "1")
	q.Set("app_id", appID)
	q.Set("method", "locatedownload")
	q.Set("path", path)
	q.Set("ver", "4.0")
	q.Set("clienttype", "17")
	q.Set("channel", "0")
	q.Set("apn_id", "1_0")
	q.Set("freeisp", "0")
	q.Set("queryfree", "0")
	q.Set("use", "0")

	fullURL := pcsBase + "/rest/2.0/pcs/file?" + q.Encode() + "&" + sig.URLParam()

	// 使用 GetRaw 获取（不需要注入通用 query）
	data, _, err := s.inv.GetRaw(ctx, fullURL)
	if err != nil {
		return "", fmt.Errorf("locate download 失败: %w", err)
	}

	// 解析返回的 URL 列表
	var result struct {
		Errno int `json:"errno"`
		URLs  []struct {
			URL string `json:"url"`
		} `json:"urls"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("解析 locate download 响应失败: %w", err)
	}
	if result.Errno != 0 {
		return "", invoker.NewAPIError(result.Errno, "locate download 失败")
	}
	if len(result.URLs) == 0 {
		return "", fmt.Errorf("download: locate download 未返回链接")
	}
	return result.URLs[0].URL, nil
}

// mergeInt64List 将 int64 数组格式化为 JSON 数组字符串。
func mergeInt64List(ids ...int64) string {
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = strconv.FormatInt(id, 10)
	}
	return "[" + strings.Join(strs, ",") + "]"
}
