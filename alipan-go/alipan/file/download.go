package file

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/langhuachuanshi/alipan-go/alipan/invoker"
)

// DownloadRequest 下载请求。
type DownloadRequest struct {
	FileID      string // 通过 file_id 下载
	URL         string // 已知下载链接，直接下载
	LocalFolder string
	LocalName   string
	DriveID     string
	Overwrite   bool
	OnProgress  func(downloaded, total int64)
}

const (
	downloadChunkSize     = 1024 * 1024
	downloadTempSuffix    = ".ali"
	downloadHeaderTimeout = 30 * time.Second // 等待响应头的保底超时，body 读取不受限
)

// GetDownloadURL 获取下载链接。
func (s *Service) GetDownloadURL(ctx context.Context, fileID, driveID string) (string, error) {
	body := map[string]any{"file_id": fileID, "drive_id": driveID, "expire_sec": 14400}
	var resp struct {
		URL string `json:"url"`
	}
	if err := invoker.PostAndDecode(ctx, s.inv, pathFileGetDownloadURL, body, &resp, []int{200}); err != nil {
		return "", err
	}
	if resp.URL == "" {
		return "", invoker.NewAPIError(0, "EmptyDownloadURL", "download url is empty (file may be blocked)")
	}
	return resp.URL, nil
}

// Download 下载文件，支持断点续传。
//
// 注意：下载使用流式传输，整体耗时取决于文件大小和网络，不应套用固定整体超时。
// 为防止 OSS 直链偶发挂起导致永久阻塞：
//   - 调用方应传入可取消/带 deadline 的 context（如 context.WithTimeout/WithCancel）。
//   - 内部已对「等待响应头」设了保底超时（downloadHeaderTimeout），但 body 读取阶段
//     仍依赖 ctx 取消。
func (s *Service) Download(ctx context.Context, req *DownloadRequest) error {
	if req == nil || req.LocalFolder == "" {
		return invoker.NewAPIError(0, "InvalidArgument", "local_folder is required")
	}
	if req.FileID == "" && req.URL == "" {
		return invoker.NewAPIError(0, "InvalidArgument", "file_id or url is required")
	}
	url := req.URL
	name := req.LocalName
	if url == "" {
		var err error
		url, err = s.GetDownloadURL(ctx, req.FileID, req.DriveID)
		if err != nil {
			return err
		}
	}
	if name == "" && req.FileID != "" {
		if f, err := s.Get(ctx, req.FileID, req.DriveID); err == nil && f.Name != "" {
			name = f.Name
		}
	}
	if name == "" {
		name = inferNameFromURL(url)
	}
	name = sanitizeFileName(name)
	if err := os.MkdirAll(req.LocalFolder, 0o755); err != nil {
		return err
	}
	finalPath := filepath.Join(req.LocalFolder, name)
	tmpPath := finalPath + downloadTempSuffix
	if !req.Overwrite {
		if _, err := os.Stat(finalPath); err == nil {
			return nil
		}
	}
	offset := int64(0)
	if fi, err := os.Stat(tmpPath); err == nil {
		offset = fi.Size()
	}
	return s.downloadRange(ctx, url, tmpPath, finalPath, offset, req.OnProgress)
}

func (s *Service) downloadRange(ctx context.Context, url, tmpPath, finalPath string, offset int64, onProgress func(int64, int64)) error {
	// 不用整体 Timeout（会截断大文件 body 读取）。改用 ResponseHeaderTimeout 兜底：
	// 只约束「等到服务器开始返回响应头」的时间；拿到头后 body 仍流式读取，
	// 由调用方传入的 ctx 负责取消。
	hc := &http.Client{Transport: &http.Transport{
		ResponseHeaderTimeout: downloadHeaderTimeout,
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	setCommonHeaders(req)
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	total := resp.ContentLength
	if offset > 0 && resp.StatusCode == http.StatusPartialContent {
		total += offset
	}
	supportAppend := resp.StatusCode == http.StatusPartialContent
	var out *os.File
	if supportAppend && offset > 0 {
		out, err = os.OpenFile(tmpPath, os.O_APPEND|os.O_WRONLY, 0o644)
	} else {
		offset = 0
		out, err = os.Create(tmpPath)
	}
	if err != nil {
		return err
	}
	defer out.Close()
	if onProgress != nil {
		onProgress(offset, total)
	}
	buf := make([]byte, downloadChunkSize)
	var downloaded int64 = offset
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			downloaded += int64(n)
			if onProgress != nil {
				onProgress(downloaded, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	return os.Rename(tmpPath, finalPath)
}

// setCommonHeaders 设置 UA/Referer/x-canary（上传 PUT 到 OSS、下载直链都需要）。
func setCommonHeaders(req *http.Request) {
	req.Header.Set("Referer", "https://aliyundrive.com")
	req.Header.Set("User-Agent", "AliApp(AYSD/5.8.0) com.alicloud.databox/37029260 Channel/36176927979800@rimet_android_5.8.0 language/zh-CN /Android Mobile/Xiaomi Redmi")
	req.Header.Set("x-canary", "client=Android,app=adrive,version=v5.8.0")
}

func sanitizeFileName(name string) string {
	repl := strings.NewReplacer(
		`\`, "_", `/`, "_", `:`, "_", `*`, "_",
		`?`, "_", `"`, "_", `<`, "_", `>`, "_", `|`, "_",
	)
	return repl.Replace(name)
}

func inferNameFromURL(rawURL string) string {
	if i := strings.IndexByte(rawURL, '?'); i >= 0 {
		rawURL = rawURL[:i]
	}
	name := rawURL[strings.LastIndexByte(rawURL, '/')+1:]
	if name == "" {
		name = fmt.Sprintf("alipan_download_%d", time.Now().Unix())
	}
	return name
}
