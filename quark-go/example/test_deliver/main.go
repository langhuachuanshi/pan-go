// 实测聚合发货:用 2 个自己的商品分享链接 → 反查 FID → 聚合成 1 个限时带码新分享。
//
// 运行前:把下面 shareURLs 换成你自己网盘里已有的分享链接(必须是本账号创建的)。
// 公开/私密、单文件夹/散文件均可。私密且 URL 没带 ?pwd= 的,在 passcodes 里填提取码。
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/langhuachuanshi/quark-go/quark"
	"github.com/langhuachuanshi/quark-go/quark/share"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	c, err := quark.New(ctx)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	// TODO: 换成你自己的商品分享链接(本账号创建的)。
	shareURLs := []string{
		"https://pan.quark.cn/s/xxxxxxxx",
		"https://pan.quark.cn/s/yyyyyyyy",
	}
	// 私密分享若 URL 里没带 ?pwd=,在这里填对应提取码(下标对齐);公开分享留空。
	passcodes := []string{"", ""}
	days := 1

	fmt.Printf("=== 聚合发货:%d 个商品 → 1 个新分享 ===\n", len(shareURLs))
	resp, err := c.Share().DeliverByShareURLs(ctx, &share.DeliverRequest{
		ShareURLs:   shareURLs,
		Passcodes:   passcodes,
		Title:       "quark-go 聚合发货测试",
		ExpiredDays: days,
	})
	if err != nil {
		log.Fatalf("聚合发货失败: %v", err)
	}

	fmt.Printf("新分享链接: %s\n", resp.ShareURL)
	fmt.Printf("提取码:    %s\n", resp.Passcode)
	fmt.Printf("有效期:    %d 天\n", days)
	fmt.Println("\n=== 实测通过 ===")
}
