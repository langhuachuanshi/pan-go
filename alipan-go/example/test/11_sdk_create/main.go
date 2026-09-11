// 实测 SDK Create 正式分享（/s/ 链接）。
// 用资源盘文件，不传 drive_id，验证自动用资源盘。
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
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c, err := alipan.New(ctx, alipan.WithTokenFile("test-account"))
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}
	rd := c.ResourceDriveID()
	fmt.Printf("资源盘=%s\n", rd)

	// 找一个资源盘的文件。
	rdFiles, _, _ := c.Files().ListPage(ctx, &file.ListRequest{ParentFileID: "root", DriveID: rd})
	var testFile string
	for _, f := range rdFiles {
		if f.IsFolder() {
			sub, _, _ := c.Files().ListPage(ctx, &file.ListRequest{ParentFileID: f.FileID, DriveID: rd})
			for _, sf := range sub {
				if sf.IsFile() {
					testFile = sf.FileID
					fmt.Printf("测试文件: %s (%s)\n", sf.Name, sf.FileID)
					break
				}
			}
		}
		if testFile != "" {
			break
		}
	}
	if testFile == "" {
		log.Fatal("没找到资源盘文件")
	}

	fmt.Println("\n=== SDK Create 正式分享（不传drive_id，自动用资源盘）===")
	resp, err := c.Share().Create(ctx, &share.CreateShareLinkRequest{
		FileIDList: []string{testFile},
		SharePwd:   "1234",
		// DriveID 故意不传，验证自动用资源盘
	})
	if err != nil {
		log.Fatalf("创建分享失败: %v", err)
	}
	fmt.Printf("\n🎉 正式分享成功！\n")
	fmt.Printf("   分享链接: %s\n", resp.ShareURL)
	fmt.Printf("   提取码:   %s\n", resp.SharePwd)
	fmt.Printf("   share_id: %s\n", resp.ShareID)
	fmt.Printf("   过期:     %v\n", resp.Expired)
}
