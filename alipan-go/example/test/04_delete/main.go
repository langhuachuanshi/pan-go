// 实测脚本 4：删除文件（移到回收站）。复用 test-account token。
// 流程：建文件夹→上传文件→删除文件→删除文件夹→验证两者进入回收站。
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
	files := c.Files()

	fmt.Println("=== 测试 4：删除文件（移到回收站）===")

	// 0. 准备：建文件夹 + 上传一个文件。
	folder, err := files.CreateFolder(ctx, &file.CreateFolderRequest{
		Name: "alipan-go-delete-test", ParentFileID: "root", DriveID: driveID,
	})
	if err != nil {
		log.Fatalf("建文件夹失败: %v", err)
	}
	localPath := filepath.Join("tmp", "delete-test.txt")
	_ = os.MkdirAll("tmp", 0o755)
	_ = os.WriteFile(localPath, []byte("待删除的测试内容 "+time.Now().Format(time.RFC3339)), 0o644)
	f, err := files.Upload(ctx, &file.UploadRequest{FilePath: localPath, ParentFileID: folder.FileID, DriveID: driveID})
	if err != nil {
		log.Fatalf("上传失败: %v", err)
	}
	fmt.Printf("[0] 准备完成：文件 %s 已上传到文件夹 %s\n", f.FileID, folder.FileID)

	// 1. 删除文件（移到回收站）。
	if err := files.Trash(ctx, f.FileID, driveID); err != nil {
		log.Fatalf("删除文件失败: %v", err)
	}
	fmt.Printf("[1] 删除文件成功 file_id=%s\n", f.FileID)

	// 2. 删除文件夹。
	if err := files.Trash(ctx, folder.FileID, driveID); err != nil {
		log.Fatalf("删除文件夹失败: %v", err)
	}
	fmt.Printf("[2] 删除文件夹成功 file_id=%s\n", folder.FileID)

	// 3. 验证：根目录已无该文件夹；回收站有这两个。
	rootFiles, _ := files.List(ctx, &file.ListRequest{ParentFileID: "root", DriveID: driveID, Limit: 100})
	for _, rf := range rootFiles {
		if rf.FileID == folder.FileID {
			log.Fatalf("验证失败：根目录仍存在被删除的文件夹")
		}
	}
	fmt.Printf("[3] 验证：根目录已无该文件夹\n")

	// 4. 验证回收站含这两个。
	recycled, err := files.ListRecycleBin(ctx, driveID)
	if err != nil {
		log.Fatalf("列回收站失败: %v", err)
	}
	fileInBin, folderInBin := false, false
	for _, r := range recycled {
		if r.FileID == f.FileID {
			fileInBin = true
		}
		if r.FileID == folder.FileID {
			folderInBin = true
		}
	}
	if !fileInBin || !folderInBin {
		log.Fatalf("验证失败：回收站未找到被删项 file=%v folder=%v", fileInBin, folderInBin)
	}
	fmt.Printf("[4] 验证：回收站已找到被删文件和文件夹\n")

	// 5. 顺手测试：从回收站恢复文件（验证 Restore）。
	if err := files.Restore(ctx, f.FileID, driveID); err != nil {
		log.Fatalf("恢复失败: %v", err)
	}
	fmt.Printf("[5] 验证：Restore 恢复文件成功\n")

	fmt.Println("\n=== 测试 4 通过 ===")
}
