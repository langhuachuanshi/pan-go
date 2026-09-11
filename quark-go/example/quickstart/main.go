// 示例：quark-go 快速入门。
// 运行前需准备夸克 cookie：浏览器登录 pan.quark.cn，F12 → Network → 任意请求 → 复制 Cookie。
// 把 cookie 写到 ~/.quark/cookie.json（格式 {"cookie":"__puus=...;..."}），或用 WithCookie 传入。
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/langhuachuanshi/quark-go/quark"
	"github.com/langhuachuanshi/quark-go/quark/file"
)

func main() {
	ctx := context.Background()
	c, err := quark.New(ctx, quark.WithCookieFile("")) // 留空用默认 ~/.quark/cookie.json
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	files, err := c.Files().List(ctx, &file.ListRequest{PDirFID: "0"})
	if err != nil {
		log.Fatalf("获取文件列表失败: %v", err)
	}
	fmt.Printf("根目录共 %d 项:\n", len(files))
	for _, f := range files {
		if f.IsFolder() {
			fmt.Printf("  [文件夹] %s\n", f.FileName)
		} else {
			fmt.Printf("  [文件]   %-30s %d 字节\n", f.FileName, f.Size)
		}
	}
}
