package file

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// 本文件实现上传进度的持久化，支持跨进程断点续传。
//
// 原理（已实测验证）：
//   - 阿里云盘底层 OSS 保留已 PUT 成功的分片（只要 upload_id 有效）
//   - upload_id 跨请求持久有效
//   - 中断后，用记录的 upload_id 重新 get_upload_url 拿新 URL，只传未完成分片
//
// 记录文件位置：~/.aligo/.upload-<sha1前16位>.json
// 每传完一片立即更新；complete 成功后删除。

// uploadProgress 上传进度记录（持久化）。
type uploadProgress struct {
	FilePath      string `json:"file_path"`
	FileSize      int64  `json:"file_size"`
	SHA1          string `json:"sha1"`           // 整文件 SHA1（大写），用作记录 key
	DriveID       string `json:"drive_id"`
	FileID        string `json:"file_id"`        // 阿里云盘文件 ID
	UploadID      string `json:"upload_id"`      // OSS 分片上传会话 ID
	ParentFileID  string `json:"parent_file_id"`
	Name          string `json:"name"`
	ChunkSize     int64  `json:"chunk_size"`
	DoneParts     []int  `json:"done_parts"`     // 已成功上传的分片号
}

// progressStoreDir 返回进度记录目录（~/.aligo）。
func progressStoreDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".aligo")
	return dir, os.MkdirAll(dir, 0o700)
}

// progressPath 返回某个文件的上传进度记录路径（按 sha1 区分）。
func progressPath(sha1 string) (string, error) {
	dir, err := progressStoreDir()
	if err != nil {
		return "", err
	}
	// sha1 取前 16 位作短名，避免文件名过长。
	short := sha1
	if len(short) > 16 {
		short = short[:16]
	}
	return filepath.Join(dir, ".upload-"+short+".json"), nil
}

// loadProgress 读取上传进度。文件不存在返回 nil。
func loadProgress(sha1 string) *uploadProgress {
	p, err := progressPath(sha1)
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var prog uploadProgress
	if json.Unmarshal(data, &prog) != nil {
		return nil
	}
	return &prog
}

// saveProgress 保存上传进度。
func saveProgress(prog *uploadProgress) {
	if prog == nil || prog.SHA1 == "" {
		return
	}
	p, err := progressPath(prog.SHA1)
	if err != nil {
		return
	}
	data, err := json.MarshalIndent(prog, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(p, data, 0o600)
}

// removeProgress 删除上传进度（complete 成功后调用）。
func removeProgress(sha1 string) {
	p, err := progressPath(sha1)
	if err != nil {
		return
	}
	_ = os.Remove(p)
}

// containsPart 判断分片号是否已在已传列表。
func containsPart(parts []int, n int) bool {
	for _, p := range parts {
		if p == n {
			return true
		}
	}
	return false
}
