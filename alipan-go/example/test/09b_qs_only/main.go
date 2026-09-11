// 精简测试：只测 QuickShare，用已知文件 ID（之前上传到资源盘的）。
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/langhuachuanshi/alipan-go/alipan"
	"github.com/langhuachuanshi/alipan-go/alipan/file"
	"github.com/langhuachuanshi/alipan-go/alipan/share"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	c, err := alipan.New(ctx, alipan.WithTokenFile("test-account"))
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	rd := c.ResourceDriveID()
	fmt.Printf("资源盘=%s\n", rd)

	// 列资源盘根目录找文件。
	rdFiles, _, _ := c.Files().ListPage(ctx, &file.ListRequest{ParentFileID: "root", DriveID: rd})
	fmt.Printf("资源盘根目录 %d 项\n", len(rdFiles))
	var testFile string
	for _, f := range rdFiles {
		fmt.Printf("  %s %s\n", f.Name, f.FileID)
		if f.IsFolder() {
			sub, _, _ := c.Files().ListPage(ctx, &file.ListRequest{ParentFileID: f.FileID, DriveID: rd})
			for _, sf := range sub {
				if sf.IsFile() {
					testFile = sf.FileID
					fmt.Printf("  -> 找到测试文件: %s (%s)\n", sf.Name, sf.FileID)
					break
				}
			}
		}
		if testFile != "" {
			break
		}
	}
	if testFile == "" {
		log.Fatal("没找到可分享的文件")
	}

	resp, err := c.Share().QuickShare(ctx, &share.QuickShareRequest{
		Files:   []string{testFile},
		DriveID: rd,
	})
	if err != nil {
		log.Fatalf("快传失败: %v", err)
	}
	fmt.Printf("\n🎉 分享链接: %s\n", resp.ShareURL)
}
