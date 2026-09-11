// 辅助：列出根目录文件，找一个可分享的。
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c, _ := alipan.New(ctx, alipan.WithTokenFile("test-account"))
	files, err := c.Files().List(ctx, &file.ListRequest{ParentFileID: "root", DriveID: c.DefaultDriveID(), Limit: 20})
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		if f.IsFile() && f.Status == "available" {
			fmt.Printf("file_id=%s name=%q status=%s size=%d\n", f.FileID, f.Name, f.Status, f.Size)
		}
	}
}
