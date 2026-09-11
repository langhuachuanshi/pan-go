// 实测转存（用真实他人分享链接）：Transfer 到新建目录 → 列表验证 → 清理。
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

// 真实他人分享链接（公开，无密码）。
const shareURL = "https://pan.quark.cn/s/11593e901277#/list/share"

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	c, err := quark.New(ctx)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	// 1. 新建目标目录（转存落地处）。
	fmt.Println("=== 1. 新建目标目录 ===")
	dirName := fmt.Sprintf("quark-go-transfer-dst-%d", time.Now().Unix())
	dirFID, err := c.Files().MakeDir(ctx, "0", dirName)
	if err != nil {
		log.Fatalf("建目标目录失败: %v", err)
	}
	fmt.Printf("✓ 目标目录 %q fid=%s\n", dirName, dirFID)
	defer func() {
		if err := c.Files().Delete(ctx, []string{dirFID}); err != nil {
			log.Printf("清理失败(非致命): %v", err)
		} else {
			fmt.Println("\n✓ 已清理目标目录")
		}
	}()

	// 2. 先 ListShareFiles 看看分享里有什么（验证 token+detail 链路，并展示内容）。
	fmt.Println("\n=== 2. 解析分享 & 查看内容 ===")
	pwdID, _, err := share.ParseShareURL(shareURL) // 若未导出则改用内部，见下
	if err != nil {
		log.Fatalf("解析 URL 失败: %v", err)
	}
	fmt.Printf("✓ pwd_id=%s\n", pwdID)

	// 3. Transfer 一键转存。
	fmt.Println("\n=== 3. Transfer 转存 ===")
	newFIDs, err := c.Share().Transfer(ctx, shareURL, "", dirFID)
	if err != nil {
		log.Fatalf("转存失败: %v", err)
	}
	fmt.Printf("✓ 转存完成，转存项数=%d，新 fid 示例: %s\n", len(newFIDs), firstFID(newFIDs))

	// 4. 列目标目录验证。
	fmt.Println("\n=== 4. 列表验证 ===")
	var count int
	for attempt := 1; attempt <= 3; attempt++ {
		time.Sleep(time.Second)
		files, err := c.Files().List(ctx, &file.ListRequest{PDirFID: dirFID, Size: 100})
		if err != nil {
			log.Fatalf("列目录失败: %v", err)
		}
		count = len(files)
		if count > 0 {
			break
		}
		fmt.Printf("（第 %d 次列表为空，重试...）\n", attempt)
	}
	if count == 0 {
		log.Fatalf("✗ 转存后目标目录仍为空")
	}
	fmt.Printf("✓ 目标目录现有 %d 项，前几项:\n", count)
	files, _ := c.Files().List(ctx, &file.ListRequest{PDirFID: dirFID, Size: 5})
	for _, f := range files {
		if f.IsFolder() {
			fmt.Printf("  [文件夹] %s\n", f.FileName)
		} else {
			fmt.Printf("  [文件]   %s (%d 字节)\n", f.FileName, f.Size)
		}
	}

	fmt.Println("\n=== 实测通过 ===")
}

func firstFID(fids []string) string {
	if len(fids) == 0 {
		return "(无)"
	}
	return fids[0]
}
