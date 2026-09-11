// 实测脚本 5：分享-管理我的分享。复用 test-account token。
// 流程：建文件夹→上传文件→创建分享→列出我的分享→取消分享。
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	c, err := alipan.New(ctx, alipan.WithTokenFile("test-account"))
	if err != nil {
		log.Fatalf("初始化客户端失败: %v", err)
	}
	driveID := c.DefaultDriveID()
	files := c.Files()

	fmt.Println("=== 测试 5：分享 - 管理我的分享 ===")

	// 0. 准备：建文件夹 + 上传一个文件。
	folder, err := files.CreateFolder(ctx, &file.CreateFolderRequest{
		Name: "alipan-go-share-test", ParentFileID: "root", DriveID: driveID,
	})
	if err != nil {
		log.Fatalf("建文件夹失败: %v", err)
	}
	localPath := filepath.Join("tmp", "share-test.txt")
	_ = os.MkdirAll("tmp", 0o755)
	_ = os.WriteFile(localPath, []byte("分享测试内容 "+time.Now().Format(time.RFC3339)), 0o644)
	f, err := files.Upload(ctx, &file.UploadRequest{FilePath: localPath, ParentFileID: folder.FileID, DriveID: driveID})
	if err != nil {
		log.Fatalf("上传失败: %v", err)
	}
	fmt.Printf("[0] 准备完成：文件 %s (%s)\n", f.FileID, f.Name)

	// 1. 创建分享链接（带提取码）。
	sharePwd := "1234"
	resp, err := c.Share().Create(ctx, &share.CreateShareLinkRequest{
		FileIDList: []string{f.FileID},
		SharePwd:   sharePwd,
	})
	if err != nil {
		log.Fatalf("创建分享失败: %v", err)
	}
	fmt.Printf("[1] 创建分享成功\n")
	fmt.Printf("    share_id=%s\n    share_url=%s\n    share_pwd=%s\n    expired=%v\n",
		resp.ShareID, resp.ShareURL, resp.SharePwd, resp.Expired)

	// 2. 列出我的分享，确认刚创建的在列表里。
	links, err := c.Share().ListMyShare(ctx, &share.ListMyShareRequest{Limit: 20})
	if err != nil {
		log.Fatalf("列出我的分享失败: %v", err)
	}
	found := false
	for _, lk := range links {
		if lk.ShareID == resp.ShareID {
			found = true
			fmt.Printf("[2] 验证：我的分享列表中找到该分享 name=%q url=%s\n", lk.ShareName, lk.ShareURL)
			break
		}
	}
	if !found {
		log.Fatalf("验证失败：我的分享列表中未找到刚创建的分享")
	}

	// 3. 取消分享。
	if err := c.Share().Cancel(ctx, resp.ShareID); err != nil {
		log.Fatalf("取消分享失败: %v", err)
	}
	fmt.Printf("[3] 取消分享成功 share_id=%s\n", resp.ShareID)

	// 4. 验证：再次列出，应已无该分享。
	links2, err := c.Share().ListMyShare(ctx, &share.ListMyShareRequest{Limit: 50})
	if err != nil {
		log.Fatalf("再次列出失败: %v", err)
	}
	for _, lk := range links2 {
		if lk.ShareID == resp.ShareID {
			log.Fatalf("验证失败：取消后分享仍存在")
		}
	}
	fmt.Printf("[4] 验证：取消后分享已不在列表\n")

	fmt.Println("\n=== 测试 5 通过 ===")
}
