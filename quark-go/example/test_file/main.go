// 实测文件管理：建目录 → 重命名 → 移动 → 列表验证 → 删除清理。
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	c, err := quark.New(ctx)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	const root = "0"
	dirName := fmt.Sprintf("quark-go-file-test-%d", time.Now().Unix())
	renameTo := dirName + "-renamed"
	subName := dirName + "-sub"

	// 1. MakeDir：在根目录建文件夹，拿 fid。
	fmt.Println("=== 1. MakeDir ===")
	dirFID, err := c.Files().MakeDir(ctx, root, dirName)
	if err != nil {
		log.Fatalf("建目录失败: %v", err)
	}
	fmt.Printf("✓ 建目录 %q 成功，fid=%s\n", dirName, dirFID)

	// 2. Rename：改名。
	fmt.Println("\n=== 2. Rename ===")
	if err := c.Files().Rename(ctx, dirFID, renameTo); err != nil {
		log.Fatalf("重命名失败: %v", err)
	}
	fmt.Printf("✓ 重命名为 %q\n", renameTo)

	// 3. MakeDir 子目录，准备移动测试。
	fmt.Println("\n=== 3. MakeDir 子目录 ===")
	subFID, err := c.Files().MakeDir(ctx, root, subName)
	if err != nil {
		log.Fatalf("建子目录失败: %v", err)
	}
	fmt.Printf("✓ 建子目录 %q，fid=%s\n", subName, subFID)

	// 4. Move：把子目录移进改名后的目录。
	fmt.Println("\n=== 4. Move ===")
	if err := c.Files().Move(ctx, []string{subFID}, dirFID); err != nil {
		log.Fatalf("移动失败: %v", err)
	}
	fmt.Printf("✓ 已把 %q 移入 %q\n", subName, renameTo)

	// 5. List：在改名目录下列，验证子目录已移入。
	fmt.Println("\n=== 5. List 验证 ===")
	// 移动有延迟，重试几次。
	var found bool
	for attempt := 1; attempt <= 3 && !found; attempt++ {
		time.Sleep(time.Second)
		files, err := c.Files().List(ctx, &file.ListRequest{PDirFID: dirFID, Size: 50})
		if err != nil {
			log.Fatalf("列目录失败: %v", err)
		}
		for _, f := range files {
			if f.FileName == subName {
				found = true
				fmt.Printf("✓ 在 %q 下找到 %q\n", renameTo, subName)
				break
			}
		}
		if !found {
			fmt.Printf("（第 %d 次未见，重试...）\n", attempt)
		}
	}
	if !found {
		log.Fatalf("✗ 移动后未在目标目录找到子目录")
	}

	// 6. Delete：删除改名目录（会连带删除已移入的子目录）。
	fmt.Println("\n=== 6. Delete ===")
	if err := c.Files().Delete(ctx, []string{dirFID}); err != nil {
		log.Fatalf("删除失败: %v", err)
	}
	fmt.Printf("✓ 已删除 %q（及内部子目录）\n", renameTo)

	fmt.Println("\n=== 实测通过 ===")
}
