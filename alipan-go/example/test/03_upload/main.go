// 实测脚本 3：上传文件。复用 test-account token。
// 流程：建测试文件夹 → 写本地小文件 → 上传 → 验证。
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/langhuachuanshi/alipan-go/alipan"
	"github.com/langhuachuanshi/alipan-go/alipan/file"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	c, err := alipan.New(ctx, alipan.WithTokenFile("test-account"))
	if err != nil {
		log.Fatalf("初始化客户端失败: %v", err)
	}
	driveID := c.DefaultDriveID()

	fmt.Println("=== 测试 3：上传文件 ===")

	// 1. 建一个测试文件夹用于上传。
	folder, err := c.Files().CreateFolder(ctx, &file.CreateFolderRequest{
		Name: "alipan-go-upload-test", ParentFileID: "root", DriveID: driveID,
	})
	if err != nil {
		log.Fatalf("建文件夹失败: %v", err)
	}
	fmt.Printf("[1] 建文件夹成功 file_id=%s\n", folder.FileID)

	// 2. 写本地测试文件（含可识别内容）。
	tmpDir := "tmp"
	_ = os.MkdirAll(tmpDir, 0o755)
	localPath := filepath.Join(tmpDir, "upload-test.txt")
	content := fmt.Sprintf("alipan-go 上传测试 %s\n这是一段测试内容。", time.Now().Format(time.RFC3339))
	if err := os.WriteFile(localPath, []byte(content), 0o644); err != nil {
		log.Fatalf("写本地文件失败: %v", err)
	}
	fi, _ := os.Stat(localPath)
	fmt.Printf("[2] 本地文件已生成 %s (%d 字节)\n", localPath, fi.Size())

	// 3. 上传。
	f, err := c.Files().Upload(ctx, &file.UploadRequest{
		FilePath:     localPath,
		ParentFileID: folder.FileID,
		DriveID:      driveID,
		OnProgress:   func(sent, total int64) { fmt.Printf("\r   上传进度: %d/%d", sent, total) },
	})
	if err != nil {
		log.Fatalf("\n上传失败: %v", err)
	}
	fmt.Printf("\n[3] 上传成功 file_id=%s name=%s size=%d\n", f.FileID, f.Name, f.Size)

	// 4. 验证：列出文件夹内容。
	files, err := c.Files().List(ctx, &file.ListRequest{ParentFileID: folder.FileID, DriveID: driveID})
	if err != nil {
		log.Fatalf("列文件验证失败: %v", err)
	}
	found := false
	for _, fl := range files {
		if fl.FileID == f.FileID {
			found = true
			fmt.Printf("[4] 验证：找到上传文件 name=%q type=%s size=%d\n", fl.Name, fl.Type, fl.Size)
			break
		}
	}
	if !found {
		log.Fatalf("验证失败：文件夹内未找到上传的文件")
	}

	fmt.Printf("\n[上传文件 file_id 供删除测试使用] %s\n", f.FileID)
	fmt.Printf("[上传所在文件夹 file_id 供删除测试使用] %s\n", folder.FileID)
	fmt.Println("\n=== 测试 3 通过 ===")
}
