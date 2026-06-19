// 实测下载：当前 main 分支（BDUSS 方案）下载未实现，本程序验证占位错误正确返回。
// 完整下载实测请用 openapi 分支。
package main

import (
	"context"
	"errors"
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

	// 调用下载，预期返回 ErrNotImplemented。
	err = c.Download().Download(ctx, &download.DownloadRequest{FSID: 1})
	if errors.Is(err, download.ErrNotImplemented) {
		fmt.Printf("✓ 占位生效：%v\n", err)
		fmt.Println("  下载功能尚未实现，请使用 openapi 分支。")
		return
	}
	if err != nil {
		log.Fatalf("下载返回了非预期的错误: %v", err)
	}
	log.Fatal("下载意外成功（占位应返回错误）")
}

