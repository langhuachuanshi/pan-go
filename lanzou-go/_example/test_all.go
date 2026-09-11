package main

import (
	"errors"
	"fmt"

	"github.com/langhuachuanshi/lanzou-go"
)

func main() {
	client := lanzou.NewClient(lanzou.WithTimeout(30))

	// 注入浏览器 Cookie
	cookies := map[string]string{
		"PHPSESSID":    "n09o7fcc91h690bd6rnajperku2ggfa2",
		"ylogin":       "838957",
		"ylogins":      "743c611f1fcbd2b2b13ea5e9d2173698",
		"phpdisk_info": "BztQYQdtAzYGMVQzXQ4EbVA3Bg0OZgJtBz0AZQY1CjtSZwUwBG5SaVVvBF0JZ1Y1BzUCY1w1VGUHMVVjB2IAZAcxUGMHbQNrBjZUZ11mBD1QMwY8Dm4CZQc9AGMGYwprUmUFPgRmUmlVMwRmCVpWPQdvAjlcMFQ2BzdVNAc2ADoHNFBqB1wDPgYzVDxdNARmUDMGPQ5kAmcHNQ==",
		"uag":          "da3724e8ddfd965d73453674e313d556",
		"folder_id_c":  "11170971",
	}
	client.SetCookiesFromMap(cookies)

	fmt.Println("========================================")
	fmt.Println("  lanzou-go SDK 功能测试")
	fmt.Println("========================================\n")

	// 1. 用户信息
	test("1. GetUserInfo", func() error {
		info, err := client.GetUserInfo()
		if err != nil {
			return err
		}
		fmt.Printf("  ✅ 用户昵称: %s\n", info.UserName)
		return nil
	})

	// 2. 帐号信息
	test("2. GetAccountInfo", func() error {
		info, err := client.GetAccountInfo()
		if err != nil {
			return err
		}
		fmt.Printf("  ✅ 昵称: %s, 总容量: %s, 已用: %s\n", info.UserName, info.TotalSize, info.UsedSize)
		return nil
	})

	// 3. 文件夹列表（根目录）
	test("3. GetDirList (根目录)", func() error {
		folders, err := client.GetDirList(-1)
		if err != nil {
			return err
		}
		fmt.Printf("  ✅ 共 %d 个文件夹\n", len(folders.Text))
		for i, f := range folders.Text {
			if i >= 10 {
				fmt.Printf("  ... 还有 %d 个\n", len(folders.Text)-10)
				break
			}
			fmt.Printf("  [%s] %s\n", f.FolID, f.Name)
		}
		return nil
	})

	// 4. 文件列表（根目录）
	test("4. GetFileList (根目录)", func() error {
		files, err := client.GetFileList(-1)
		if err != nil {
			return err
		}
		fmt.Printf("  ✅ 共 %d 个文件\n", len(files.Text))
		for i, f := range files.Text {
			if i >= 10 {
				fmt.Printf("  ... 还有 %d 个\n", len(files.Text)-10)
				break
			}
			fmt.Printf("  [%s] %s (%s)\n", f.ID, f.NameAll, f.Size)
		}
		return nil
	})

	// 5. 子文件夹（folder_id=12152484）
	test("5. GetDirList (12152484)", func() error {
		folders, err := client.GetDirList(12152484)
		if err != nil {
			return err
		}
		fmt.Printf("  ✅ 共 %d 个子文件夹\n", len(folders.Text))
		for _, f := range folders.Text {
			fmt.Printf("  [%s] %s\n", f.FolID, f.Name)
		}
		return nil
	})

	// 6. 回收站
	test("6. GetRecycleList", func() error {
		list, err := client.GetRecycleList(1)
		if err != nil {
			return err
		}
		fmt.Printf("  ✅ 回收站共 %d 个文件\n", len(list.Text))
		return nil
	})

	// 7. 直链解析
	test("7. GetDurlByURL", func() error {
		durl, err := client.GetDurlByURL("https://36cq.lanzouo.com/i5qJz2z14zoh", "")
		if err != nil {
			return err
		}
		fmt.Printf("  ✅ 直链: %s...\n", durl[:min(len(durl), 80)])
		return nil
	})

	fmt.Println("\n========================================")
	fmt.Println("  测试完成")
	fmt.Println("========================================")
}

func test(name string, fn func() error) {
	fmt.Printf("▶ %s\n", name)
	if err := fn(); err != nil {
		if errors.Is(err, lanzou.ErrNotLoggedIn) {
			fmt.Printf("  ❌ 未登录: %v\n", err)
		} else {
			fmt.Printf("  ❌ 失败: %v\n", err)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
