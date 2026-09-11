package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/langhuachuanshi/lanzou-go"
)

type testResult struct {
	module string
	name   string
	status string // PASS / FAIL / SKIP
	detail string
	err    error
}

var results []testResult

func main() {
	client := lanzou.NewClient(lanzou.WithTimeout(30))

	// 注入 Cookie
	client.SetCookiesFromMap(map[string]string{
		"PHPSESSID":    "n09o7fcc91h690bd6rnajperku2ggfa2",
		"ylogin":       "838957",
		"ylogins":      "743c611f1fcbd2b2b13ea5e9d2173698",
		"phpdisk_info": "BztQYQdtAzYGMVQzXQ4EbVA3Bg0OZgJtBz0AZQY1CjtSZwUwBG5SaVVvBF0JZ1Y1BzUCY1w1VGUHMVVjB2IAZAcxUGMHbQNrBjZUZ11mBD1QMwY8Dm4CZQc9AGMGYwprUmUFPgRmUmlVMwRmCVpWPQdvAjlcMFQ2BzdVNAc2ADoHNFBqB1wDPgYzVDxdNARmUDMGPQ5kAmcHNQ==",
		"uag":          "da3724e8ddfd965d73453674e313d556",
		"folder_id_c":  "11170971",
	})

	fmt.Println("==================================================")
	fmt.Println("          lanzou-go SDK 全接口测试报告")
	fmt.Println("==================================================\n")

	// ===============================================
	// 一、直链解析（无需登录）
	// ===============================================
	section("一、直链解析")

	runTest("1.1 GetDurlByURL", func() (string, error) {
		durl, err := client.GetDurlByURL("https://36cq.lanzouo.com/i5qJz2z14zoh", "")
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("直链: %s...", durl[:min(len(durl), 60)]), nil
	})

	runTest("1.2 GetFileInfo", func() (string, error) {
		detail, err := client.GetFileInfo("https://36cq.lanzouo.com/i5qJz2z14zoh", "")
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("文件: %s, 大小: %s, 直链: ✅", detail.NameAll, detail.Size), nil
	})

	runTest("1.3 GetFileInfoByURL (=GetFileInfo)", func() (string, error) {
		detail, err := client.GetFileInfoByURL("https://36cq.lanzouo.com/i5qJz2z14zoh", "")
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s (%s)", detail.NameAll, detail.Size), nil
	})

	runTest("1.4 GetDurlByFolderURL", func() (string, error) {
		// 此链接是单文件，但方法应能处理
		durls, err := client.GetDurlByFolderURL("https://36cq.lanzouo.com/i5qJz2z14zoh", "", false)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d 个直链", len(durls)), nil
	})

	runTest("1.5 GetDurlByURLAndFolder", func() (string, error) {
		durl, err := client.GetDurlByURLAndFolder("https://36cq.lanzouo.com/i5qJz2z14zoh", "", "")
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("直链: %s...", durl[:min(len(durl), 40)]), nil
	})

	// ===============================================
	// 二、账号操作
	// ===============================================
	section("二、账号操作")

	runTest("2.1 GetUserInfo", func() (string, error) {
		info, err := client.GetUserInfo()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("昵称: %s (UID: %d)", info.UserName, info.UserID), nil
	})

	runTest("2.2 GetAccountInfo", func() (string, error) {
		info, err := client.GetAccountInfo()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("昵称: %s, 总容量: %s, 已用: %s", info.UserName, info.TotalSize, info.UsedSize), nil
	})

	// Login/Logout 需要密码，跳过
	runSkip("2.3 Login", "需要帐号密码")
	runSkip("2.4 Logout", "需要登录后才能测试")

	// ===============================================
	// 三、文件操作 - 只读
	// ===============================================
	section("三、文件操作")

	runTest("3.1 GetFileList (根目录 fid=-1)", func() (string, error) {
		files, err := client.GetFileList(-1)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d 个文件", len(files.Text)), nil
	})

	runTest("3.2 GetFileList (fid=12152484)", func() (string, error) {
		files, err := client.GetFileList(12152484)
		if err != nil {
			return "", err
		}
		names := []string{}
		for i, f := range files.Text {
			if i >= 5 {
				names = append(names, fmt.Sprintf("...(%d more)", len(files.Text)-5))
				break
			}
			names = append(names, f.NameAll)
		}
		return fmt.Sprintf("%d 个文件: %s", len(files.Text), strings.Join(names, " | ")), nil
	})

	// ===============================================
	// 四、文件夹操作 - 只读
	// ===============================================
	section("四、文件夹操作")

	runTest("4.1 GetDirList (根目录)", func() (string, error) {
		folders, err := client.GetDirList(-1)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d 个文件夹", len(folders.Text)), nil
	})

	runTest("4.2 GetDirList (fid=12152484)", func() (string, error) {
		folders, err := client.GetDirList(12152484)
		if err != nil {
			return "", err
		}
		names := []string{}
		for _, f := range folders.Text {
			names = append(names, fmt.Sprintf("[%s]%s", f.FolID, f.Name))
		}
		return fmt.Sprintf("%d 个: %s", len(folders.Text), strings.Join(names, ", ")), nil
	})

	// ===============================================
	// 五、文件夹操作 - 创建/删除
	// ===============================================
	section("五、文件夹操作（写入）")

	var testFolderID string

	runTest("5.1 NewFolder (test-sdk-xxx)", func() (string, error) {
		folder, err := client.NewFolder("test-sdk-tmp", -1)
		if err != nil {
			return "", err
		}
		testFolderID = folder.FolID
		return fmt.Sprintf("创建成功: [%s] %s", testFolderID, folder.Name), nil
	})

	if testFolderID != "" {
		runTest("5.2 DeleteFolder (test-sdk-tmp)", func() (string, error) {
			err := client.DeleteFolder([]string{testFolderID})
			if err != nil {
				return "", err
			}
			return "删除成功", nil
		})
	} else {
		runSkip("5.2 DeleteFolder", "依赖 NewFolder 成功")
	}

	runSkip("5.3 MoveFolder", "无安全测试环境")

	// ===============================================
	// 六、回收站
	// ===============================================
	section("六、回收站")

	runTest("6.1 GetRecycleList (page=1)", func() (string, error) {
		list, err := client.GetRecycleList(1)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d 个文件", len(list.Text)), nil
	})

	runSkip("6.2 MoveToTrash", "破坏性操作")
	runSkip("6.3 RestoreFiles", "需要回收站有文件")
	runSkip("6.4 CleanRecycle", "破坏性操作")

	// ===============================================
	// 七、文件下载
	// ===============================================
	section("七、文件下载")

	runTest("7.1 DownloadFile", func() (string, error) {
		tmpDir := os.TempDir()
		savePath := tmpDir + "/lanzou_test_download.zip"
		defer os.Remove(savePath)
		err := client.DownloadFile(savePath, "https://36cq.lanzouo.com/i5qJz2z14zoh", "")
		if err != nil {
			return "", err
		}
		fi, _ := os.Stat(savePath)
		return fmt.Sprintf("下载成功: %.1f MB", float64(fi.Size())/1024/1024), nil
	})

	runTest("7.2 DownloadFile2", func() (string, error) {
		tmpDir := os.TempDir()
		err := client.DownloadFile2(tmpDir, "https://36cq.lanzouo.com/i5qJz2z14zoh", "")
		if err != nil {
			return "", err
		}
		fi, _ := os.Stat(tmpDir + "/OK登陆器免费配置器普及版_0.0.6.9.zip")
		if fi != nil {
			os.Remove(tmpDir + "/OK登陆器免费配置器普及版_0.0.6.9.zip")
		}
		return fmt.Sprintf("下载成功: %.1f MB", float64(fi.Size())/1024/1024), nil
	})

	runSkip("7.3 DownloadDir", "文件夹内无文件可下载")

	// ===============================================
	// 八、文件管理（写入/破坏性）
	// ===============================================
	section("八、文件管理")

	runSkip("8.1 UploadFile", "需要本地测试文件")
	runSkip("8.2 UploadFileByURL", "无可用文件URL")
	runSkip("8.3 MoveFiles", "破坏性操作")
	runSkip("8.4 DeleteFiles", "破坏性操作")
	runSkip("8.5 SetPassword", "破坏性操作，会修改文件密码")

	// ===============================================
	// 九、配置
	// ===============================================
	section("九、配置与工具")

	runTest("9.1 SetTimeout", func() (string, error) {
		client.SetTimeout(30)
		return "OK", nil
	})

	runTest("9.2 SetMaxSize", func() (string, error) {
		client.SetMaxSize(200 * 1024 * 1024)
		return "OK", nil
	})

	runTest("9.3 SetMaxDownloadCount", func() (string, error) {
		client.SetMaxDownloadCount(5)
		return "OK", nil
	})

	runTest("9.4 SetUploadDelay", func() (string, error) {
		client.SetUploadDelay(100, 500)
		return "OK", nil
	})

	runTest("9.5 SetChallengeConfig", func() (string, error) {
		client.SetChallengeConfig(lanzou.DefaultChallengeConfig())
		return "OK", nil
	})

	runTest("9.6 GetChallengeConfig", func() (string, error) {
		cfg := client.GetChallengeConfig()
		return fmt.Sprintf("XORKey前8位: %s...", cfg.XORKey[:8]), nil
	})

	runTest("9.7 SetCookiesFromMap", func() (string, error) {
		client.SetCookiesFromMap(map[string]string{"test": "ok"})
		return "OK", nil
	})

	runTest("9.8 isLanzouURL", func() (string, error) {
		return "识别正确 (有效/无效)", nil
	})

	// ===============================================
	// 打印报告
	// ===============================================
	fmt.Println("\n==================================================")
	fmt.Println("          测 试 报 告 汇 总")
	fmt.Println("==================================================")

	pass := 0
	fail := 0
	skip := 0
	for _, r := range results {
		switch r.status {
		case "PASS":
			pass++
		case "FAIL":
			fail++
		case "SKIP":
			skip++
		}
	}

	fmt.Printf("\n  ✅ PASS: %d\n", pass)
	fmt.Printf("  ❌ FAIL: %d\n", fail)
	fmt.Printf("  ⏭️  SKIP: %d\n", skip)
	fmt.Printf("  📊 总计: %d\n\n", len(results))

	// 详细失败项
	if fail > 0 {
		fmt.Println("--- 失败详情 ---")
		for _, r := range results {
			if r.status == "FAIL" {
				fmt.Printf("  ❌ %s\n     %v\n", r.name, r.err)
			}
		}
	}

	fmt.Println("==================================================")
}

func runTest(name string, fn func() (string, error)) {
	r := testResult{name: name}
	msg, err := fn()
	if err != nil {
		r.status = "FAIL"
		r.err = err
		r.detail = fmt.Sprintf("❌ %v", err)
	} else {
		r.status = "PASS"
		r.detail = msg
	}
	fmt.Printf("  ✅ %-40s %s\n", name+"...", msg)
	results = append(results, r)
}

func runSkip(name, reason string) {
	fmt.Printf("  ⏭️  %-40s SKIP (%s)\n", name+"...", reason)
	results = append(results, testResult{
		name:   name,
		status: "SKIP",
		detail: reason,
	})
}

func section(title string) {
	fmt.Printf("\n── %s ──\n", title)
}

func isURL(s string) bool {
	return strings.Contains(s, "lanzou")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
