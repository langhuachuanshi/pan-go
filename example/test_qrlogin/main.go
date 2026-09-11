// 实测扫码登录：Create 出二维码 → 打印内容链接（自行贴到二维码生成器出图）→
// Wait 轮询换发 cookie → 用新 cookie 验证列根目录。
//
// 不落盘（不覆盖 ~/.quark/cookie.json），只打印验证结果。
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/langhuachuanshi/quark-go/quark"
	"github.com/langhuachuanshi/quark-go/quark/file"
	"github.com/langhuachuanshi/quark-go/quark/qrcode"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	login, err := qrcode.Create(ctx)
	if err != nil {
		log.Fatalf("创建二维码失败: %v", err)
	}
	fmt.Println("请用夸克App扫描此内容对应的二维码（贴到任意二维码生成器出图）：")
	fmt.Println("  " + login.QRURL)
	fmt.Println("（有效期约 2 分钟，等待扫码中...）")

	cookie, err := login.Wait(ctx, 120*time.Second)
	if err != nil {
		log.Fatalf("扫码登录失败: %v", err)
	}
	fmt.Printf("\n登录成功，换发 cookie %d 字符\n", len(cookie))

	// 立即验证
	c, err := quark.New(ctx, quark.WithCookie(cookie))
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}
	files, err := c.Files().List(ctx, &file.ListRequest{PDirFID: "0", Size: 5})
	if err != nil {
		log.Fatalf("验证失败: %v", err)
	}
	fmt.Printf("✅ 验证通过：新 cookie 列根目录 %d 项\n", len(files))
}
