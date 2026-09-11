// dump 创建分享的真实响应 JSON，找出正确的字段名。
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/langhuachuanshi/quark-go/quark"
	"github.com/langhuachuanshi/quark-go/quark/file"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	c, err := quark.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fid := findFile(ctx, c, "0")
	if fid == "" {
		log.Fatal("没找到文件")
	}
	fmt.Printf("fid=%s\n", fid)

	// 直接用 invoker 拿原始响应。
	body := map[string]any{
		"fid_list":     []string{fid},
		"title":        "diag-test",
		"url_type":     1,
		"expired_type": 1,
	}
	data, _, err := c.Post(ctx, "/share", body, nil, nil)
	if err != nil {
		log.Fatalf("请求失败: %v", err)
	}
	fmt.Printf("\n完整响应:\n%s\n", string(data))
}

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
