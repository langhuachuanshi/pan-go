package lanzou

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

// UploadFile 上传本地文件到蓝奏云
// filePath: 本地文件路径, fid: 目标文件夹ID（根目录传 0）, desc: 文件描述（可选）
func (c *Client) UploadFile(filePath string, fid int, desc ...string) (*UploadResult, error) {
	if !c.isLoggedIn() {
		return nil, ErrNotLoggedIn
	}

	// 检查文件大小
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat file failed: %w", err)
	}
	if info.Size() > int64(c.maxsize) {
		return nil, fmt.Errorf("%w: file size %d exceeds limit %d", ErrFileSizeLimit, info.Size(), c.maxsize)
	}

	// 打开文件
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file failed: %w", err)
	}
	defer f.Close()

	// 上传延迟
	if c.uploadDelay[1] > c.uploadDelay[0] {
		delay := c.uploadDelay[0] + rand.Intn(c.uploadDelay[1]-c.uploadDelay[0])
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	// 构造上传字段（与蓝奏云 web 端 html5up.php 一致）
	// 注意：folder_id 字段名为 folder_id_bb_n，vie/ve 为固定值，无需 t_a/t_b/t_c
	descStr := ""
	if len(desc) > 0 {
		descStr = desc[0]
	}
	fields := map[string]string{
		"task":           "1",
		"vie":            "2",
		"ve":             "2",
		"id":             "WU_FILE_0",
		"folder_id_bb_n": fmt.Sprintf("%d", fid),
		"name":           filepath.Base(filePath),
	}
	if descStr != "" {
		fields["des"] = descStr
	}

	// 上传：必须走 pc.woozooo.com/html5up.php（up.woozooo.com/up.php 只返回 HTML 页面）
	headers := map[string]string{
		"Referer": baseURLPC + "/mydisk.php",
		"Origin":  baseURLPC,
	}
	body, _, err := c.postMultipart(
		baseURLPC+pathUpload,
		fields,
		"upload_file",
		filepath.Base(filePath),
		f,
		headers,
	)
	if err != nil {
		return nil, fmt.Errorf("upload request failed: %w", err)
	}

	return parseUploadResult(body)
}

// parseUploadResult 解析上传响应，UploadFile 和 UploadFileWithProgress 共用。
func parseUploadResult(body []byte) (*UploadResult, error) {
	var resp uploadResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("%w: invalid upload response (not JSON, first 200 bytes: %q)", ErrAPIError, truncate(string(body), 200))
	}
	if resp.Zt != 1 {
		infoStr := resp.Info
		return nil, fmt.Errorf("%w: zt=%d info=%s", ErrUploadFailed, resp.Zt, infoStr)
	}

	// text 是数组，第一个元素包含 file_id 和 name_all
	result := &UploadResult{
		Zt:   resp.Zt,
		Info: resp.Info,
	}
	if len(resp.Text) > 0 {
		result.FileID = resp.Text[0].ID
		result.FileName = resp.Text[0].NameAll
	}
	return result, nil
}

// uploadResp html5up.php 上传接口响应
type uploadResp struct {
	Zt   int `json:"zt"`
	Info string `json:"info"`
	Text []struct {
		ID      string `json:"id"`
		NameAll string `json:"name_all"`
	} `json:"text"`
}

// truncate 截断字符串到指定长度，用于错误信息
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// UploadFileByURL 上传网盘已有文件（非本地文件）
// fileURL: 文件URL, fid: 目标文件夹ID
func (c *Client) UploadFileByURL(fileURL string, fid int, desc ...string) (*UploadResult, error) {
	if !c.isLoggedIn() {
		return nil, ErrNotLoggedIn
	}

	descStr := ""
	if len(desc) > 0 {
		descStr = desc[0]
	}

	data := map[string]string{
		"task":      "42",
		"folder_id": fmt.Sprintf("%d", fid),
		"url":       fileURL,
		"name":      filepath.Base(fileURL),
		"des":       descStr,
	}
	body, _, err := c.post(baseURLPC+pathTaskAPI, data, nil)
	if err != nil {
		return nil, fmt.Errorf("upload by url failed: %w", err)
	}

	var resp UploadResult
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("%w: invalid upload response", ErrAPIError)
	}
	if resp.Zt == 0 {
		return nil, fmt.Errorf("%w: %s", ErrUploadFailed, resp.Info)
	}
	return &resp, nil
}
