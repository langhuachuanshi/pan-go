package main

import (
	"fmt"
	"log"
	"time"

	"github.com/langhuachuanshi/lanzou-go"
)

func main() {
	// 创建客户端
	client := lanzou.NewClient(
		lanzou.WithTimeout(30),
		lanzou.WithMaxSize(100*1024*1024), // 100MB
		lanzou.WithMaxDownloadCount(3),
	)

	// ===== 示例1：解析直链（无需登录） =====
	fmt.Println("=== 示例1：解析直链 ===")
	durl, err := client.GetDurlByURL("https://pan.lanzoul.com/ixKQw26c7wd", "")
	if err != nil {
		log.Printf("解析直链失败: %v\n", err)
	} else {
		fmt.Println("直链:", durl)
	}

	// ===== 示例2：登录后操作 =====
	fmt.Println("\n=== 示例2：登录操作 ===")
	err = client.Login("your_username", "your_password")
	if err != nil {
		log.Printf("登录失败: %v\n", err)
		return
	}
	defer client.Logout()

	// 获取用户信息
	userInfo, err := client.GetUserInfo()
	if err != nil {
		log.Printf("获取用户信息失败: %v\n", err)
	} else {
		fmt.Printf("用户: %s\n", userInfo.UserName)
	}

	// ===== 示例3：获取文件列表 =====
	fmt.Println("\n=== 示例3：文件列表 ===")
	files, err := client.GetFileList(-1) // 根目录
	if err != nil {
		log.Printf("获取文件列表失败: %v\n", err)
	} else {
		fmt.Printf("共 %d 个文件:\n", len(files.Text))
		for _, f := range files.Text {
			fmt.Printf("  [%s] %s (%s) - %s\n", f.ID, f.NameAll, f.Size, f.Time)
		}
	}

	// ===== 示例4：获取文件夹列表 =====
	fmt.Println("\n=== 示例4：文件夹列表 ===")
	folders, err := client.GetDirList(-1) // 根目录
	if err != nil {
		log.Printf("获取文件夹列表失败: %v\n", err)
	} else {
		fmt.Printf("共 %d 个文件夹:\n", len(folders.Text))
		for _, f := range folders.Text {
			fmt.Printf("  [%s] %s\n", f.FolID, f.Name)
		}
	}

	// ===== 示例5：创建文件夹 =====
	fmt.Println("\n=== 示例5：创建文件夹 ===")
	newFolder, err := client.NewFolder("test-folder", -1)
	if err != nil {
		log.Printf("创建文件夹失败: %v\n", err)
	} else {
		fmt.Printf("创建成功: [%s] %s\n", newFolder.FolID, newFolder.Name)
	}

	// ===== 示例6：上传文件 =====
	fmt.Println("\n=== 示例6：上传文件 ===")
	// 确保有测试文件
	uploadResult, err := client.UploadFile("test.txt", 0, "测试上传")
	if err != nil {
		log.Printf("上传失败: %v\n", err)
	} else {
		fmt.Printf("上传成功: [%s] %s\n", uploadResult.FileID, uploadResult.FileName)
	}

	// ===== 示例7：下载文件 =====
	fmt.Println("\n=== 示例7：下载文件 ===")
	err = client.DownloadFile("./downloads/test.txt", "https://pan.lanzoul.com/xxxxx", "")
	if err != nil {
		log.Printf("下载失败: %v\n", err)
	} else {
		fmt.Println("下载成功!")
	}

	// ===== 示例8：获取回收站 =====
	fmt.Println("\n=== 示例8：回收站 ===")
	recycleList, err := client.GetRecycleList(1)
	if err != nil {
		log.Printf("获取回收站失败: %v\n", err)
	} else {
		fmt.Printf("回收站共 %d 个文件:\n", len(recycleList.Text))
		for _, f := range recycleList.Text {
			fmt.Printf("  [%s] %s\n", f.FileID, f.FileName)
		}
	}

	// ===== 示例9：批量下载文件夹 =====
	fmt.Println("\n=== 示例9：批量下载文件夹 ===")
	err = client.DownloadDir("./downloads/myfolder", -1) // 根目录
	if err != nil {
		log.Printf("批量下载失败: %v\n", err)
	} else {
		fmt.Println("批量下载完成!")
	}

	fmt.Println("\n所有示例执行完毕!")
	time.Sleep(time.Second)
}
