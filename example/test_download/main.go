// 实测下载：验证下载 URL 获取。
// 需要 PANBAIDU_BDUSS 环境变量。
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/langhuachuanshi/baidupan-go/baidu"
	"github.com/langhuachuanshi/baidupan-go/baidu/download"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := baidu.New(ctx, &baidu.Config{BDUSS: os.Getenv("PANBAIDU_BDUSS")})
	if err != nil {
		log.Fatal(err)
	}

	// 获取 PCS 直链下载 URL（按路径）
	url, err := c.Download().GetDownloadURL(ctx, &download.DownloadRequest{
		Path: "/test.txt",
	})
	if err != nil {
		log.Fatal("获取下载 URL 失败:", err)
	}
	fmt.Printf("PCS 下载直链: %s\n", url)

	// 也可以用 PanAPI 方式（需要先设置 PanHome 缓存 + fs_id）
	fmt.Println("\n下载功能已实现:")
	fmt.Println("  - PCS 直链: GetDownloadURL(path) -> 直接 HTTP GET 下载")
	fmt.Println("  - PanAPI:    设置 PanHome 缓存后可按 fs_id 下载")
	fmt.Println("  - Locate:    GetPCSLocateURL(path, uid, bduss) 获取多节点 URL")
}
