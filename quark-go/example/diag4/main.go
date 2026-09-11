// 验证：查询我创建的分享是否真实有效，对比浏览器手动创建的。
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/langhuachuanshi/quark-go/quark"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	c, err := quark.New(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// 列出我的所有分享，看格式。
	fmt.Println("=== 我的所有分享 ===")
	// 夸克列分享接口：POST /share/mypage/detail 或 GET /share/mypage/list
	// 先试 raw。
	data, status, err := c.Get(ctx, "/share/mypage/detail", map[string]string{"_page": "1", "_size": "20"}, nil)
	fmt.Printf("mypage/detail status=%d err=%v\n", status, err)
	if data != nil {
		s := string(data)
		if len(s) > 800 {
			s = s[:800] + "..."
		}
		fmt.Printf("body: %s\n", s)
	}

	// 也测下另一个 path。
	fmt.Println("\n=== share/page ===")
	data2, status2, _ := c.Get(ctx, "/share/page", map[string]string{"_page": "1", "_size": "20"}, nil)
	fmt.Printf("status=%d\n", status2)
	if data2 != nil {
		s := string(data2)
		if len(s) > 800 {
			s = s[:800] + "..."
		}
		fmt.Printf("body: %s\n", s)
	}
}
