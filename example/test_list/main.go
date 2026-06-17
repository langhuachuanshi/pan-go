// 实测：列出网盘根目录文件，验证 access_token 注入与 list 接口。
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/langhuachuanshi/panbaidu-go/baidu"
	"github.com/langhuachuanshi/panbaidu-go/baidu/file"
)

func appKey() string { return os.Getenv("PANBAIDU_APP_KEY") }
func secretKey() string { return os.Getenv("PANBAIDU_SECRET_KEY") }

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := baidu.New(ctx, &baidu.Config{
		AppKey:     appKey(),
		SecretKey:  secretKey(),
	})
	if err != nil {
		log.Fatal(err)
	}

	files, err := c.Files().List(ctx, &file.ListRequest{Dir: "/", Limit: 20})
	if err != nil {
		log.Fatalf("列文件失败: %v", err)
	}
	fmt.Printf("根目录共 %d 项:\n", len(files))
	for _, f := range files {
		if f.IsFolder() {
			fmt.Printf("  [文件夹] %s (fs_id=%d)\n", f.Name(), f.FSID)
		} else {
			fmt.Printf("  [文件]   %-30s %d 字节 (fs_id=%d)\n", f.Name(), f.Size, f.FSID)
		}
	}
	fmt.Println("\n=== 实测通过 ===")
}
