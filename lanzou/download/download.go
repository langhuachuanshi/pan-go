// Package download 下载：文件 / 文件夹（递归并发）/ 直链。
package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/langhuachuanshi/pan-go/lanzou/file"
	"github.com/langhuachuanshi/pan-go/lanzou/folder"
	"github.com/langhuachuanshi/pan-go/lanzou/invoker"
	"github.com/langhuachuanshi/pan-go/lanzou/resolve"
)

const pcBase = "https://pc.woozooo.com"

// Service 下载服务（依赖 file / folder / resolve 服务）。
type Service struct {
	inv     invoker.Invoker
	files   *file.Service
	folders *folder.Service
	resolve *resolve.Service
}

// New 创建 download Service。
func New(inv invoker.Invoker) *Service {
	return &Service{
		inv:     inv,
		files:   file.New(inv),
		folders: folder.New(inv),
		resolve: resolve.New(inv),
	}
}

// File 下载文件到指定路径。
// savePath: 保存路径（完整文件路径）, shareURL: 分享链接, pwd: 密码（可选）。
func (s *Service) File(ctx context.Context, savePath, shareURL string, pwd ...string) error {
	password := ""
	if len(pwd) > 0 {
		password = pwd[0]
	}

	durl, err := s.resolve.GetDurlByURL(ctx, shareURL, password)
	if err != nil {
		return fmt.Errorf("get download url failed: %w", err)
	}
	return s.byURL(ctx, savePath, durl, shareURL)
}

// FileAuto 简化版下载（按远端文件名自动命名，保存到目录）。
// saveDir: 保存目录, shareURL: 分享链接, pwd: 密码。
func (s *Service) FileAuto(ctx context.Context, saveDir, shareURL, pwd string) error {
	detail, err := s.resolve.GetFileInfo(ctx, shareURL, pwd)
	if err != nil {
		return fmt.Errorf("get file info failed: %w", err)
	}

	fileName := detail.NameAll
	if fileName == "" {
		fileName = filepath.Base(shareURL)
	}
	return s.File(ctx, filepath.Join(saveDir, fileName), shareURL, pwd)
}

// Dir 下载整个文件夹（含子文件夹递归，并发受 MaxDownloadCount 限制）。
// saveDir: 保存目录, fid: 文件夹ID。
func (s *Service) Dir(ctx context.Context, saveDir string, fid int) error {
	if !s.inv.LoggedIn() {
		return invoker.ErrNotLoggedIn
	}

	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return fmt.Errorf("create dir failed: %w", err)
	}

	files, err := s.files.List(ctx, fid)
	if err != nil {
		return fmt.Errorf("get file list failed: %w", err)
	}

	sem := make(chan struct{}, s.inv.MaxDownloadCount())
	var wg sync.WaitGroup
	var errs []error
	var mu sync.Mutex

	for _, f := range files.Text {
		wg.Add(1)
		sem <- struct{}{}
		go func(fi *file.FileInfo) {
			defer wg.Done()
			defer func() { <-sem }()

			savePath := filepath.Join(saveDir, fi.NameAll)
			durl, err := s.resolveFileURL(ctx, fi)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("resolve %s failed: %w", fi.NameAll, err))
				mu.Unlock()
				return
			}
			if err := s.byURL(ctx, savePath, durl, pcBase+"/"); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("download %s failed: %w", fi.NameAll, err))
				mu.Unlock()
			}
		}(f)
	}

	wg.Wait()

	folders, _ := s.folders.List(ctx, fid)
	for _, fo := range folders.Text {
		subDir := filepath.Join(saveDir, fo.Name)
		subFid, _ := strconv.Atoi(fo.FolID)
		if err := s.Dir(ctx, subDir, subFid); err != nil {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("download dir completed with %d errors: %v", len(errs), errs[0])
	}
	return nil
}

// ByURL 直接通过直链下载。
func (s *Service) ByURL(ctx context.Context, savePath, durl string) error {
	return s.byURL(ctx, savePath, durl, "")
}

// resolveFileURL 根据 FileInfo 解析下载URL（task=22 → 构造分享链接 → 解析直链）。
func (s *Service) resolveFileURL(ctx context.Context, f *file.FileInfo) (string, error) {
	if f.FID == "" {
		return "", fmt.Errorf("file has no fid")
	}
	var resp struct {
		Zt   int `json:"zt"`
		Info struct {
			FID string `json:"f_id"`
			Pwd string `json:"pwd"`
		} `json:"info"`
	}
	err := s.inv.PostForm(ctx, invoker.PathAjaxm, map[string]string{
		"task":    "22",
		"file_id": f.ID,
	}, &resp)
	if err != nil || resp.Zt != 1 {
		return "", fmt.Errorf("resolve file url failed")
	}
	shareURL := fmt.Sprintf("https://pan.lanzoul.com/%s", resp.Info.FID)
	return s.resolve.GetDurlByURL(ctx, shareURL, resp.Info.Pwd)
}

// byURL 通过直链下载文件（流式写盘；直链 GET 用模块下载头 + 底层 client）。
func (s *Service) byURL(ctx context.Context, savePath, durl, referer string) error {
	dir := filepath.Dir(savePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir failed: %w", err)
	}

	f, err := os.Create(savePath)
	if err != nil {
		return fmt.Errorf("create file failed: %w", err)
	}
	defer f.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, durl, nil)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	for k, v := range s.inv.DownloadHeaders() {
		req.Header.Set(k, v)
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := s.inv.HTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: http status %d", invoker.ErrDownloadFailed, resp.StatusCode)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write file failed: %w", err)
	}
	return nil
}
