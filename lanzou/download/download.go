// Package download 下载：文件 / 文件夹（递归并发）/ 直链。
package download

import (
	"encoding/json"
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
func (s *Service) File(savePath, shareURL string, pwd ...string) error {
	password := ""
	if len(pwd) > 0 {
		password = pwd[0]
	}

	// 获取直链
	durl, err := s.resolve.GetDurlByURL(shareURL, password)
	if err != nil {
		return fmt.Errorf("get download url failed: %w", err)
	}

	// 下载文件
	return s.byURL(savePath, durl, shareURL)
}

// FileAuto 简化版下载（按远端文件名自动命名，保存到目录）。
// saveDir: 保存目录, shareURL: 分享链接, pwd: 密码。
func (s *Service) FileAuto(saveDir, shareURL, pwd string) error {
	// 获取文件信息
	detail, err := s.resolve.GetFileInfo(shareURL, pwd)
	if err != nil {
		return fmt.Errorf("get file info failed: %w", err)
	}

	// 确定文件名
	fileName := detail.NameAll
	if fileName == "" {
		fileName = filepath.Base(shareURL)
	}

	savePath := filepath.Join(saveDir, fileName)
	return s.File(savePath, shareURL, pwd)
}

// Dir 下载整个文件夹（含子文件夹递归，并发受 MaxDownloadCount 限制）。
// saveDir: 保存目录, fid: 文件夹ID。
func (s *Service) Dir(saveDir string, fid int) error {
	if !s.inv.LoggedIn() {
		return invoker.ErrNotLoggedIn
	}

	// 创建保存目录
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return fmt.Errorf("create dir failed: %w", err)
	}

	// 获取文件列表
	files, err := s.files.List(fid)
	if err != nil {
		return fmt.Errorf("get file list failed: %w", err)
	}

	// 并发下载
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
			durl, err := s.resolveFileURL(fi)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("resolve %s failed: %w", fi.NameAll, err))
				mu.Unlock()
				return
			}
			if err := s.byURL(savePath, durl, pcBase+"/"); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("download %s failed: %w", fi.NameAll, err))
				mu.Unlock()
			}
		}(f)
	}

	wg.Wait()

	// 获取子文件夹列表并递归下载
	folders, _ := s.folders.List(fid)
	for _, fo := range folders.Text {
		subDir := filepath.Join(saveDir, fo.Name)
		subFid, _ := strconv.Atoi(fo.FolID)
		if err := s.Dir(subDir, subFid); err != nil {
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
func (s *Service) ByURL(savePath, durl string) error {
	return s.byURL(savePath, durl, "")
}

// resolveFileURL 根据 FileInfo 解析下载URL（task=22 → 构造分享链接 → 解析直链）。
func (s *Service) resolveFileURL(f *file.FileInfo) (string, error) {
	if f.FID == "" {
		return "", fmt.Errorf("file has no fid")
	}
	data := map[string]string{
		"task":    "22",
		"file_id": f.ID,
	}
	body, _, err := s.inv.Post(s.inv.AjaxmURL(), data, nil)
	if err != nil {
		return "", err
	}
	var resp struct {
		Zt   int `json:"zt"`
		Info struct {
			IsNewd string `json:"is_newd"`
			FID    string `json:"f_id"`
			Pwd    string `json:"pwd"`
		} `json:"info"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || resp.Zt != 1 {
		return "", fmt.Errorf("resolve file url failed")
	}
	// 构造分享链接
	shareURL := fmt.Sprintf("https://pan.lanzoul.com/%s", resp.Info.FID)
	return s.resolve.GetDurlByURL(shareURL, resp.Info.Pwd)
}

// byURL 通过直链下载文件。
func (s *Service) byURL(savePath, durl, referer string) error {
	// 确保目录存在
	dir := filepath.Dir(savePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir failed: %w", err)
	}

	// 创建文件
	f, err := os.Create(savePath)
	if err != nil {
		return fmt.Errorf("create file failed: %w", err)
	}
	defer f.Close()

	// 发起下载请求
	req, err := http.NewRequest("GET", durl, nil)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("User-Agent", s.inv.UserAgent())
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

	// 写入文件
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write file failed: %w", err)
	}

	return nil
}
