// 辅助：测试只读分享操作（ListMyShare）是否可用，判断 403 是风控还是代码问题。
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/langhuachuanshi/alipan-go/alipan"
	"github.com/langhuachuanshi/alipan-go/alipan/share"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c, _ := alipan.New(ctx, alipan.WithTokenFile("test-account"))

	links, err := c.Share().ListMyShare(ctx, &share.ListMyShareRequest{Limit: 20})
	if err != nil {
		log.Fatalf("ListMyShare 失败: %v", err)
	}
	fmt.Printf("[OK] ListMyShare 成功，共 %d 个分享\n", len(links))
	for i, lk := range links {
		if i >= 5 {
			break
		}
		fmt.Printf("  %d. %q %s expired=%v\n", i+1, lk.ShareName, lk.ShareURL, lk.Expired)
	}
}
