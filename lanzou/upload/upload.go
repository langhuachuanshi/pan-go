// Package upload 上传：本地文件 / 流式进度上传 / 网盘文件转存。
package upload

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/langhuachuanshi/pan-go/lanzou/invoker"
)

// UploadResult 上传结果
type UploadResult struct {
	Zt       int    `json:"zt"`
	Info     string `json:"info"`
	FileID   string `json:"file_id"`
	FileName string `json:"name_all"`
}

// uploadResp html5up.php 上传接口响应
type uploadResp struct {
	Zt   int    `json:"zt"`
	Info string `json:"info"`
	Text []struct {
		ID      string `json:"id"`
		NameAll string `json:"name_all"`
	} `json:"text"`
}

// Service 上传服务。
type Service struct{ inv invoker.Invoker }

// New 创建 upload Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// File 上传本地文件（一次性载入内存，兼容保留；进度场景用 Stream）。
// filePath: 本地文件路径, fid: 目标文件夹ID（根目录传 0）, desc: 文件描述（可选）。
func (s *Service) File(ctx context.Context, filePath string, fid int, desc ...string) (*UploadResult, error) {
	if !s.inv.LoggedIn() {
		return nil, invoker.ErrNotLoggedIn
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat file failed: %w", err)
	}
	if info.Size() > int64(s.inv.MaxSize()) {
		return nil, fmt.Errorf("%w: file size %d exceeds limit %d", invoker.ErrFileSizeLimit, info.Size(), s.inv.MaxSize())
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file failed: %w", err)
	}
	defer f.Close()

	if minMs, maxMs := s.inv.UploadDelay(); maxMs > minMs {
		delay := minMs + rand.Intn(maxMs-minMs)
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	var resp uploadResp
	err = s.inv.Multipart(ctx, invoker.PathUpload, s.buildFields(filePath, fid, desc),
		"upload_file", filepath.Base(filePath), f, &resp)
	if err != nil {
		return nil, fmt.Errorf("upload request failed: %w", err)
	}
	return parseUploadResult(&resp)
}

// Stream 流式上传本地文件，支持上传进度回调。
//
// 文件内容边读边发（io.Pipe + multipart 直写 request body），不一次性载入内存。
// onProgress 在网络传输时触发，反映真实上传进度（可为 nil）。
func (s *Service) Stream(ctx context.Context, filePath string, fid int, onProgress func(uploaded, total int64), desc ...string) (*UploadResult, error) {
	if !s.inv.LoggedIn() {
		return nil, invoker.ErrNotLoggedIn
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat file failed: %w", err)
	}
	if info.Size() > int64(s.inv.MaxSize()) {
		return nil, fmt.Errorf("%w: file size %d exceeds limit %d", invoker.ErrFileSizeLimit, info.Size(), s.inv.MaxSize())
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file failed: %w", err)
	}
	defer f.Close()

	var resp uploadResp
	err = s.inv.PostMultipartStream(ctx, invoker.PathUpload, s.buildFields(filePath, fid, desc),
		"upload_file", filepath.Base(filePath), f, info.Size(), onProgress,
		map[string]string{
			"Referer": "https://pc.woozooo.com/mydisk.php",
			"Origin":  "https://pc.woozooo.com",
		}, &resp)
	if err != nil {
		return nil, fmt.Errorf("upload request failed: %w", err)
	}
	return parseUploadResult(&resp)
}

// ByURL 上传网盘已有文件（非本地文件）。
// fileURL: 文件URL, fid: 目标文件夹ID。
func (s *Service) ByURL(ctx context.Context, fileURL string, fid int, desc ...string) (*UploadResult, error) {
	if !s.inv.LoggedIn() {
		return nil, invoker.ErrNotLoggedIn
	}

	descStr := ""
	if len(desc) > 0 {
		descStr = desc[0]
	}

	var resp UploadResult
	err := s.inv.PostForm(ctx, invoker.PathTaskAPI, map[string]string{
		"task":      "42",
		"folder_id": fmt.Sprintf("%d", fid),
		"url":       fileURL,
		"name":      filepath.Base(fileURL),
		"des":       descStr,
	}, &resp)
	if err != nil {
		return nil, fmt.Errorf("upload by url failed: %w", err)
	}
	if resp.Zt == 0 {
		return nil, fmt.Errorf("%w: %s", invoker.ErrUploadFailed, resp.Info)
	}
	return &resp, nil
}

// buildFields 构造上传字段（与蓝奏云 web 端 html5up.php 一致）。
// 注意：folder_id 字段名为 folder_id_bb_n，vie/ve 为固定值，无需 t_a/t_b/t_c。
func (s *Service) buildFields(filePath string, fid int, desc []string) map[string]string {
	fields := map[string]string{
		"task":           "1",
		"vie":            "2",
		"ve":             "2",
		"id":             "WU_FILE_0",
		"folder_id_bb_n": fmt.Sprintf("%d", fid),
		"name":           filepath.Base(filePath),
	}
	if len(desc) > 0 && desc[0] != "" {
		fields["des"] = desc[0]
	}
	return fields
}

// parseUploadResult 解析上传响应，各上传入口共用。
func parseUploadResult(resp *uploadResp) (*UploadResult, error) {
	if resp.Zt != 1 {
		return nil, fmt.Errorf("%w: zt=%d info=%s", invoker.ErrUploadFailed, resp.Zt, resp.Info)
	}
	result := &UploadResult{Zt: resp.Zt, Info: resp.Info}
	if len(resp.Text) > 0 {
		result.FileID = resp.Text[0].ID
		result.FileName = resp.Text[0].NameAll
	}
	return result, nil
}
