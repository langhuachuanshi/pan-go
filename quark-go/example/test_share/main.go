// 实测创建分享：列文件找一个 → 创建永久公开分享 → 创建私密带提取码分享。
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/langhuachuanshi/quark-go/quark"
	"github.com/langhuachuanshi/quark-go/quark/file"
	"github.com/langhuachuanshi/quark-go/quark/share"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	c, err := quark.New(ctx)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	// 列文件找一个文件（递归找）。
	fid := findFile(ctx, c, "0")
	if fid == "" {
		log.Fatal("没找到可分享的文件")
	}
	fmt.Printf("测试文件 fid=%s\n\n", fid)

	// 1. 永久公开分享。
	fmt.Println("=== 1. 永久公开分享 ===")
	resp1, err := c.Share().Create(ctx, &share.CreateRequest{
		FIDs:    []string{fid},
		Title:   "quark-go-test-public",
		Forever: true,
	})
	if err != nil {
		log.Fatalf("创建公开分享失败: %v", err)
	}
	fmt.Printf("分享链接: %s\n", resp1.ShareURL)
	fmt.Printf("share_id: %s\n\n", resp1.ShareID)

	// 2. 私密带提取码分享（提取码必须4位字母数字混合，如 ab12）。
	fmt.Println("=== 2. 私密带提取码分享 ===")
	resp2, err := c.Share().Create(ctx, &share.CreateRequest{
		FIDs:         []string{fid},
		Title:        "quark-go-test-private",
		Forever:      true,
		WithPasscode: true,
		Passcode:     "ab12",
	})
	if err != nil {
		log.Fatalf("创建私密分享失败: %v", err)
	}
	fmt.Printf("分享链接: %s\n", resp2.ShareURL)
	fmt.Printf("提取码: %s\n", resp2.Passcode)
	fmt.Printf("share_id: %s\n", resp2.ShareID)

	fmt.Println("\n=== 实测通过 ===")
}

// findFile 递归找一个文件 fid。
func findFile(ctx context.Context, c *quark.Client, pdir string) string {
	files, _ := c.Files().List(ctx, &file.ListRequest{PDirFID: pdir, Size: 50})
	for _, f := range files {
		if f.IsFile() {
			return f.FID
		}
	}
	for _, f := range files {
		if f.IsFolder() {
			if fid := findFile(ctx, c, f.FID); fid != "" {
				return fid
			}
		}
	}
	return ""
}
