package lanzou

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// DownloadFile 下载文件到指定路径
// savePath: 保存路径（完整文件路径）, shareURL: 分享链接, pwd: 密码（可选）
func (c *Client) DownloadFile(savePath, shareURL string, pwd ...string) error {
	password := ""
	if len(pwd) > 0 {
		password = pwd[0]
	}

	// 获取直链
	durl, err := c.GetDurlByURL(shareURL, password)
	if err != nil {
		return fmt.Errorf("get download url failed: %w", err)
	}

	// 下载文件
	return c.downloadByURL(savePath, durl, shareURL)
}

// DownloadFile2 简化版下载（自动命名）
// saveDir: 保存目录, shareURL: 分享链接, pwd: 密码
func (c *Client) DownloadFile2(saveDir, shareURL, pwd string) error {
	// 获取文件信息
	detail, err := c.GetFileInfo(shareURL, pwd)
	if err != nil {
		return fmt.Errorf("get file info failed: %w", err)
	}

	// 确定文件名
	fileName := detail.NameAll
	if fileName == "" {
		fileName = filepath.Base(shareURL)
	}

	savePath := filepath.Join(saveDir, fileName)
	return c.DownloadFile(savePath, shareURL, pwd)
}

// DownloadDir 下载整个文件夹
// saveDir: 保存目录, fid: 文件夹ID
func (c *Client) DownloadDir(saveDir string, fid int) error {
	if !c.isLoggedIn() {
		return ErrNotLoggedIn
	}

	// 创建保存目录
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return fmt.Errorf("create dir failed: %w", err)
	}

	// 获取文件列表
	files, err := c.GetFileList(fid)
	if err != nil {
		return fmt.Errorf("get file list failed: %w", err)
	}

	// 并发下载
	sem := make(chan struct{}, c.maxDLCount)
	var wg sync.WaitGroup
	var errs []error
	var mu sync.Mutex

	for _, file := range files.Text {
		wg.Add(1)
		sem <- struct{}{}
		go func(f *FileInfo) {
			defer wg.Done()
			defer func() { <-sem }()

			savePath := filepath.Join(saveDir, f.NameAll)
			// 通过文件ID构造分享链接（需要从文件信息中获取分享URL）
			durl, err := c.resolveDownloadURLForFile(f)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("resolve %s failed: %w", f.NameAll, err))
				mu.Unlock()
				return
			}
			if err := c.downloadByURL(savePath, durl, baseURLPC+"/"); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("download %s failed: %w", f.NameAll, err))
				mu.Unlock()
			}
		}(file)
	}

	wg.Wait()

	// 获取子文件夹列表并递归下载
	folders, _ := c.GetDirList(fid)
	for _, folder := range folders.Text {
		subDir := filepath.Join(saveDir, folder.Name)
		subFid, _ := strconv.Atoi(folder.FolID)
		if err := c.DownloadDir(subDir, subFid); err != nil {
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

// resolveDownloadURLForFile 根据 FileInfo 解析下载URL
func (c *Client) resolveDownloadURLForFile(f *FileInfo) (string, error) {
	// 使用文件的 FID 和 is_newd 构造分享链接并获取直链
	if f.FID == "" {
		return "", fmt.Errorf("file has no fid")
	}
	// 通过 task=22 获取文件直链
	c.initUID()
	data := map[string]string{
		"task":    "22",
		"file_id": f.ID,
	}
	body, _, err := c.post(c.apiURL(pathAjaxm), data, nil)
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
	return c.GetDurlByURL(shareURL, resp.Info.Pwd)
}

// downloadByURL 通过直链下载文件
func (c *Client) downloadByURL(savePath, durl, referer string) error {
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
	req.Header.Set("User-Agent", defaultUA)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: http status %d", ErrDownloadFailed, resp.StatusCode)
	}

	// 写入文件
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write file failed: %w", err)
	}

	return nil
}

// DownloadByURL 直接通过直链下载（公开方法）
func (c *Client) DownloadByURL(savePath, durl string) error {
	return c.downloadByURL(savePath, durl, "")
}
