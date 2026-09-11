// 实测重复创建同名文件夹的行为（百度/蓝奏云历史上踩过的坑）：
//  1. 裸 MakeDir 连续两次建同名目录 → 服务端是拒绝、复用还是建出重复目录？
//  2. 若产生重复目录，List 按名匹配会拿到哪个 fid（库的 MakeDir 取第一个匹配）？
//  3. getOrCreate 编排（先 List 查同名复用）是否稳定避免重复。
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	c, err := quark.New(ctx)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	// 根目录下建测试父目录（getOrCreate）
	parentFID, err := ensureDir(ctx, c, "0", "mkdir重复测试-990")
	if err != nil {
		log.Fatalf("建父目录失败: %v", err)
	}
	fmt.Printf("父目录 mkdir重复测试-990 fid=%s\n\n", parentFID)

	// —— 场景1：裸 MakeDir 连续两次建同名 ——
	fmt.Println("=== 1. 裸 MakeDir 两次建同名 'dup' ===")
	fid1, err := c.Files().MakeDir(ctx, parentFID, "dup")
	fmt.Printf("第1次: fid=%s err=%v\n", fid1, err)
	fid2, err := c.Files().MakeDir(ctx, parentFID, "dup")
	fmt.Printf("第2次: fid=%s err=%v\n", fid2, err)
	sameFid := fid1 == fid2 && fid1 != ""

	// —— 数一下父目录里到底有几个 'dup' ——
	files, err := c.Files().List(ctx, &file.ListRequest{PDirFID: parentFID, Size: 100})
	if err != nil {
		log.Fatalf("列目录失败: %v", err)
	}
	var dups []string
	for _, f := range files {
		if f.Category == 0 && f.FileName == "dup" {
			dups = append(dups, f.FID)
		}
	}
	fmt.Printf("父目录里名为 'dup' 的目录数=%d fid=%v 两次返回同fid=%v\n", len(dups), dups, sameFid)

	// —— 场景2：再补一次裸 MakeDir（第3次）——
	fmt.Println("\n=== 2. 第3次裸 MakeDir ===")
	fid3, err := c.Files().MakeDir(ctx, parentFID, "dup")
	fmt.Printf("第3次: fid=%s err=%v\n", fid3, err)

	files, _ = c.Files().List(ctx, &file.ListRequest{PDirFID: parentFID, Size: 100})
	dups = dups[:0]
	for _, f := range files {
		if f.Category == 0 && f.FileName == "dup" {
			dups = append(dups, f.FID)
		}
	}
	fmt.Printf("最终 'dup' 目录数=%d\n", len(dups))

	// —— 场景3：getOrCreate 三连（应零新增）——
	fmt.Println("\n=== 3. getOrCreate 三连 ===")
	for i := 1; i <= 3; i++ {
		fid, err := ensureDir(ctx, c, parentFID, "dup")
		fmt.Printf("第%d次: fid=%s err=%v\n", i, fid, err)
	}

	// —— 清理 ——
	fmt.Println("\n=== 4. 清理 ===")
	all := []string{parentFID}
	files, _ = c.Files().List(ctx, &file.ListRequest{PDirFID: parentFID, Size: 100})
	for _, f := range files {
		if f.Category == 0 {
			all = append(all, f.FID)
		}
	}
	if err := c.Files().Delete(ctx, all); err != nil {
		fmt.Printf("清理失败（请手动删 mkdir重复测试-990）: %v\n", err)
	} else {
		fmt.Println("已清理")
	}
}

func ensureDir(ctx context.Context, c *quark.Client, parentFID, name string) (string, error) {
	files, err := c.Files().List(ctx, &file.ListRequest{PDirFID: parentFID, Size: 200})
	if err != nil {
		return "", fmt.Errorf("列目录: %w", err)
	}
	for _, f := range files {
		if f.Category == 0 && f.FileName == name {
			return f.FID, nil
		}
	}
	return c.Files().MakeDir(ctx, parentFID, name)
}
