// 分享功能实测：创建 → 查询 → 列表 → 取消（完整闭环）。
//
// 使用方式：
//   set PANBAIDU_BDUSS=你的BDUSS
//   set PANBAIDU_STOKEN=你的STOKEN
//   go run example/test_share/main.go
//
// 会创建一个测试分享（根目录下第一个文件），验证后自动取消清理。
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/langhuachuanshi/baidupan-go/baidu"
	"github.com/langhuachuanshi/baidupan-go/baidu/file"
	"github.com/langhuachuanshi/baidupan-go/baidu/share"
)

func main() {
	bduss := os.Getenv("PANBAIDU_BDUSS")
	stoken := os.Getenv("PANBAIDU_STOKEN")
	if bduss == "" || stoken == "" {
		log.Fatal("请设置 PANBAIDU_BDUSS 和 PANBAIDU_STOKEN 环境变量")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	c, err := baidu.New(ctx, &baidu.Config{BDUSS: bduss, STOKEN: stoken})
	if err != nil {
		log.Fatal("创建客户端失败:", err)
	}

	// 先列出根目录，找一个文件来分享
	fmt.Println("=== 0. 获取根目录文件列表 ===")
	files, err := c.Files().List(ctx, &file.ListRequest{Dir: "/", Num: 10})
	if err != nil {
		log.Fatal("获取文件列表失败:", err)
	}
	if len(files) == 0 {
		log.Fatal("根目录没有文件，无法测试分享")
	}
	// 取第一个非目录文件
	var testPath string
	for _, f := range files {
		if !f.IsFolder() {
			testPath = f.Path
			fmt.Printf("测试文件: %s (%d bytes)\n", f.Name(), f.Size)
			break
		}
	}
	if testPath == "" {
		log.Fatal("根目录没有普通文件，无法测试分享")
	}

	// === 1. 创建分享 ===
	fmt.Println("\n=== 1. 创建分享 ===")
	result, err := c.Share().ShareSet(ctx, []string{testPath}, &share.ShareOption{
		Period: 1, // 1天有效期
	})
	if err != nil {
		log.Fatal("❌ 创建分享失败:", err)
	}
	fmt.Printf("✅ 创建成功!\n")
	fmt.Printf("   链接: %s\n", result.Link)
	fmt.Printf("   密码: %s\n", result.Pwd)
	fmt.Printf("   ShareID: %d\n", result.ShareID)

	// === 2. 查询分享详情 ===
	fmt.Println("\n=== 2. 查询分享详情 ===")
	info, err := c.Share().ShareSURLInfo(ctx, result.ShareID)
	if err != nil {
		fmt.Printf("⚠️  查询详情失败: %v\n", err)
	} else {
		fmt.Printf("✅ 查询成功!\n")
		fmt.Printf("   短链: %s\n", info.ShortURL)
		fmt.Printf("   密码: %s\n", info.Pwd)
	}

	// === 3. 列出所有分享 ===
	fmt.Println("\n=== 3. 分享列表 ===")
	list, err := c.Share().ShareList(ctx, 1)
	if err != nil {
		fmt.Printf("⚠️  获取分享列表失败: %v\n", err)
	} else {
		fmt.Printf("✅ 共 %d 条分享:\n", len(list))
		for _, rec := range list {
			status := "正常"
			if rec.Status != 0 {
				status = "已取消"
			}
			fmt.Printf("   [%s] ShareID=%d %s 浏览=%d\n", status, rec.ShareID, rec.Path, rec.ViewCount)
		}
	}

	// === 4. 取消分享 ===
	fmt.Println("\n=== 4. 取消分享（清理） ===")
	err = c.Share().ShareCancel(ctx, []int64{result.ShareID})
	if err != nil {
		fmt.Printf("⚠️  取消分享失败: %v\n", err)
	} else {
		fmt.Println("✅ 已取消（清理完毕）")
	}

	fmt.Println("\n🎉 测试完成！")
}
