// Package download 实现百度网盘文件下载。
//
// 下载两步：
//  1. GET /xpan/multimedia?method=filemetas&dlink=1&fsids=[fsid] → data.list[0].dlink（临时直链，8h 有效）
//  2. GET dlink + "&access_token=xxx"，UA 必须是 "pan.baidu.com"（CDN 防盗链校验）
//
// 关键坑（来自官方 SDK + 文档双重确认）：
//   - dlink 必须拼接 access_token 才能用（裸 GET 被 CDN 拒）
//   - 下载 UA 必须是 "pan.baidu.com"（不是 netdisk;... 也不是默认 UA）
//   - 无需 Cookie/Referer，鉴权全靠 query 里的 access_token
//   - dlink 会 302 跳转到真实 CDN，http.Client 默认跟随即可
package download

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/langhuachuanshi/panbaidu-go/baidu/invoker"
)

// CDN 防盗链要求的 UA（百度 SDK 同款）。
const cdnUserAgent = "pan.baidu.com"

// dlink 直链用的独立 client（不用 invoker，因为 UA 和 token 拼接方式不同）。
var dlinkHTTP = &http.Client{Timeout: 10 * time.Minute}

// tokenGetter 取当前 access_token（供 dlink 拼接）。
// 用接口而非直接依赖 auth.Manager，避免循环依赖。
type tokenGetter interface {
	RawToken(ctx context.Context) (string, error)
}

// Service 下载入口。
type Service struct {
	inv  invoker.Invoker
	tok  tokenGetter
}

// New 创建 download Service。tok 用于取 access_token 拼接 dlink。
func New(inv invoker.Invoker, tok tokenGetter) *Service {
	return &Service{inv: inv, tok: tok}
}

// DownloadRequest 下载请求。
type DownloadRequest struct {
	FSID       int64                         // 必填。要下载的文件 fs_id
	Writer     io.Writer                     // 必填。下载内容写入目标
	OnProgress func(downloaded, total int64) // 可选进度回调
}

// fileMetaResponse filemetas 响应。
type fileMetaResponse struct {
	Errno int `json:"errno"`
	List  []struct {
		Filename string `json:"filename"`
		Size     int64  `json:"size"`
		Dlink    string `json:"dlink"`
		IsDir    int    `json:"isdir"`
	} `json:"list"`
}

// GetDownloadURL 获取文件临时下载直链（dlink，有效期 8 小时）。
// 也可用于外部下载器（aria2）或 302 跳转。
//
// 注意：返回的 url 已是绝对地址，但实际下载时仍需拼接 access_token（Download 方法已自动处理）。
func (s *Service) GetDownloadURL(ctx context.Context, fsid int64) (string, error) {
	fsidsJSON, _ := json.Marshal([]int64{fsid})
	params := map[string]string{
		"method": "filemetas",
		"dlink":  "1",
		"fsids":  string(fsidsJSON),
	}
	var resp fileMetaResponse
	if err := invoker.GetAndDecode(ctx, s.inv, "/xpan/multimedia", params, &resp); err != nil {
		return "", err
	}
	if resp.Errno != 0 {
		return "", invoker.NewAPIError(resp.Errno, errnoMsg(resp.Errno))
	}
	if len(resp.List) == 0 || resp.List[0].Dlink == "" {
		return "", invoker.NewAPIError(0, "响应未返回 dlink")
	}
	return resp.List[0].Dlink, nil
}

// Download 下载文件内容到 Writer，流式（io.Copy，零额外内存）。
func (s *Service) Download(ctx context.Context, req *DownloadRequest) error {
	if req == nil || req.Writer == nil {
		return invoker.NewAPIError(0, "Writer is required")
	}
	progress := req.OnProgress

	dlink, err := s.GetDownloadURL(ctx, req.FSID)
	if err != nil {
		return err
	}

	// dlink 必须拼 access_token（裸 GET 被 CDN 拒）。
	tok, err := s.tok.RawToken(ctx)
	if err != nil {
		return err
	}
	fullURL := dlink + "&access_token=" + tok

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return err
	}
	// CDN 防盗链要求 UA 必须是 pan.baidu.com。
	httpReq.Header.Set("User-Agent", cdnUserAgent)

	resp, err := dlinkHTTP.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("下载失败: status=%d body=%s", resp.StatusCode, string(b))
	}

	total := resp.ContentLength
	if progress != nil {
		progress(0, max64(total, 0))
	}

	w := req.Writer
	if progress == nil {
		_, err = io.Copy(w, resp.Body)
		return err
	}
	// 带进度的流式拷贝。
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

// errnoMsg 常见 errno 描述。
func errnoMsg(errno int) string {
	switch errno {
	case -6:
		return "access_token 失效或权限不足"
	case -7:
		return "路径不存在或无权访问"
	case 31064:
		return "文件不存在"
	default:
		return "请求失败"
	}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
