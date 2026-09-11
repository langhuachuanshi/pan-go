// 实测脚本 2：新建文件夹。复用 test-account 已登录的 token。
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/langhuachuanshi/alipan-go/alipan"
	"github.com/langhuachuanshi/alipan-go/alipan/file"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 复用已持久化的登录态（test-account.json）。
	c, err := alipan.New(ctx, alipan.WithTokenFile("test-account"))
	if err != nil {
		log.Fatalf("初始化客户端失败: %v", err)
	}
	driveID := c.DefaultDriveID()

	folderName := "alipan-go-测试文件夹"
	fmt.Printf("=== 测试 2：新建文件夹 %q ===\n", folderName)

	// 1. 创建文件夹。
	resp, err := c.Files().CreateFolder(ctx, &file.CreateFolderRequest{
		Name:         folderName,
		ParentFileID: "root",
		DriveID:      driveID,
	})
	if err != nil {
		log.Fatalf("创建文件夹失败: %v", err)
	}
	fmt.Printf("[OK] 创建成功，file_id=%s\n", resp.FileID)

	// 2. 验证：列出根目录，确认文件夹存在。
	files, err := c.Files().List(ctx, &file.ListRequest{
		ParentFileID: "root", DriveID: driveID, Limit: 50,
	})
	if err != nil {
		log.Fatalf("列文件验证失败: %v", err)
	}
	found := false
	for _, f := range files {
		if f.FileID == resp.FileID {
			found = true
			fmt.Printf("[OK] 验证：根目录找到该文件夹 name=%q type=%s\n", f.Name, f.Type)
			break
		}
	}
	if !found {
		log.Fatalf("验证失败：根目录未找到刚创建的文件夹")
	}

	// 把 file_id 写到 tmp 供后续测试（上传/删除）使用。
	fmt.Printf("\n[文件夹 file_id 供后续测试使用] %s\n", resp.FileID)
	fmt.Println("\n=== 测试 2 通过 ===")
}
