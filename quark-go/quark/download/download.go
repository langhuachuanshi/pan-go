// Package download 实现夸克网盘文件下载。
//
// 夸克下载分两步：
//  1. POST /file/download（body {fids:[fid]}）→ data[0].download_url，临时直链（有时效）
//  2. GET 该直链 → 文件字节流
//
// download_url 是临时直链，GET 时需带 Cookie/Referer/UA（AList 实现里也是这么带的）。
// 单连接流式下载（io.Copy），不做并发分片——简单优先，后续需要再加。
package download

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/langhuachuanshi/quark-go/quark/invoker"
)

// Service 下载操作入口。
type Service struct {
	inv invoker.Invoker
}

// New 创建 download Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// DownloadRequest 下载请求（可选字段）。
type DownloadRequest struct {
	FID        string                          // 必填。要下载的文件 fid
	Writer     io.Writer                       // 必填。下载内容写入目标
	OnProgress func(downloaded, total int64)   // 可选进度回调（total 来自 Content-Length，未知则 0）
}

// GetDownloadURL 获取文件临时下载直链（有时效，一般几分钟到几小时）。
// 适用场景：302 跳转、外部下载器（aria2）、前端直接拉取。
//
// 夸克接口：POST /file/download，body {fids:[fid]}，响应 data[0].download_url。
func (s *Service) GetDownloadURL(ctx context.Context, fid string) (string, error) {
	if fid == "" {
		return "", invoker.NewAPIError(0, "fid is required")
	}
	body := map[string]any{"fids": []string{fid}}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"message"`
		Data []struct {
			DownloadURL string `json:"download_url"`
		} `json:"data"`
	}
	if err := invoker.PostAndDecode(ctx, s.inv, "/file/download", body, nil, &resp); err != nil {
		return "", err
	}
	if resp.Code != 0 {
		return "", invoker.NewAPIError(resp.Code, resp.Msg)
	}
	if len(resp.Data) == 0 || resp.Data[0].DownloadURL == "" {
		return "", invoker.NewAPIError(0, "响应未返回 download_url")
	}
	return resp.Data[0].DownloadURL, nil
}

// Download 下载文件内容到 Writer，流式（io.Copy，零额外内存）。
//
// 用法：f, _ := os.Create("本地路径"); c.Download().Download(ctx, req); f.Close()
func (s *Service) Download(ctx context.Context, req *DownloadRequest) error {
	if req == nil || req.FID == "" {
		return invoker.NewAPIError(0, "FID is required")
	}
	if req.Writer == nil {
		return invoker.NewAPIError(0, "Writer is required")
	}
	progress := req.OnProgress

	url, err := s.GetDownloadURL(ctx, req.FID)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	// 直链 GET 需带夸克鉴权头（Cookie/Referer/UA），否则 403 RequestDeniedByCallback。
	for k, v := range s.inv.DownloadHeaders() {
		httpReq.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("下载失败: status=%d body=%.200s", resp.StatusCode, string(b))
	}

	total := resp.ContentLength // 未知则为 -1
	if progress != nil {
		progress(0, max64(total, 0))
	}

	// 流式拷贝，带进度。
	w := req.Writer
	if progress == nil {
		_, err = io.Copy(w, resp.Body)
		return err
	}
	buf := make([]byte, 64*1024)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)
			progress(written, max64(total, 0))
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
