// 实测 SDK 一键快传：资源盘建文件夹→上传→QuickShare→官方分享链接。
// 这是用户的核心诉求：上传文件 + 一键分享。
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
	"github.com/langhuachuanshi/alipan-go/alipan/share"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	c, err := alipan.New(ctx, alipan.WithTokenFile("test-account"))
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}
	resourceDrive := c.ResourceDriveID()
	fmt.Printf("资源盘 drive_id=%s\n\n", resourceDrive)

	fmt.Println("=== SDK 一键快传分享测试 ===")

	// 1. 资源盘建文件夹。
	folder, err := c.Files().CreateFolder(ctx, &file.CreateFolderRequest{
		Name: "sdk-quickshare-demo", ParentFileID: "root", DriveID: resourceDrive,
	})
	if err != nil {
		log.Fatalf("建文件夹失败: %v", err)
	}
	fmt.Printf("[1] 建文件夹成功 file_id=%s\n", folder.FileID)

	// 2. 上传文件。
	tmpPath := filepath.Join("tmp", "sdk-share.txt")
	os.MkdirAll("tmp", 0o755)
	os.WriteFile(tmpPath, []byte("SDK快传测试 "+time.Now().Format(time.RFC3339)), 0o644)
	f, err := c.Files().Upload(ctx, &file.UploadRequest{
		FilePath: tmpPath, ParentFileID: folder.FileID, DriveID: resourceDrive,
	})
	if err != nil {
		log.Fatalf("上传失败: %v", err)
	}
	fmt.Printf("[2] 上传成功 file_id=%s name=%s\n", f.FileID, f.Name)

	// 3. 一键快传分享！
	resp, err := c.Share().QuickShare(ctx, &share.QuickShareRequest{
		Files: []string{f.FileID},
	})
	if err != nil {
		log.Fatalf("快传失败: %v", err)
	}
	fmt.Printf("\n[3] 🎉 快传分享成功！\n")
	fmt.Printf("    分享链接: %s\n", resp.ShareURL)
	fmt.Printf("    share_id: %s\n", resp.ShareID)
	fmt.Printf("    过期时间: %s\n", resp.Expiration)

	fmt.Println("\n=== 测试通过：上传 + 一键分享贯通 ===")
}
