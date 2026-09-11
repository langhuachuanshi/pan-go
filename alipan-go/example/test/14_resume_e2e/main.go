// 实测断点续传：25MB文件，分3片。
// 第一次运行（INTERRUPT=1）：传完第1片后强制退出，模拟中断。
// 第二次运行（无INTERRUPT）：应自动续传第2、3片，不重传第1片。
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/langhuachuanshi/alipan-go/alipan"
	"github.com/langhuachuanshi/alipan-go/alipan/file"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	c, err := alipan.New(ctx, alipan.WithTokenFile("test-account"))
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}
	rd := c.ResourceDriveID()

	// 25MB 文件（3片），内容含时间戳避免秒传。
	filePath := filepath.Join("tmp", "resume-sdk-test.bin")
	os.MkdirAll("tmp", 0o755)
	if _, err := os.Stat(filePath); err != nil {
		f, _ := os.Create(filePath)
		chunk := make([]byte, 10*1024*1024)
		// 用时间戳做种子，每次内容不同，确保不秒传、走分片路径。
		seed := byte(time.Now().UnixNano() % 256)
		for i := range chunk {
			chunk[i] = byte(i) ^ seed
		}
		f.Write(chunk)
		f.Write(chunk)
		f.Write(chunk[:5*1024*1024])
		f.Close()
	}
	fi, _ := os.Stat(filePath)
	fmt.Printf("文件: %s (%d字节 %.1fMB)\n", filePath, fi.Size(), float64(fi.Size())/1024/1024)

	interrupt := os.Getenv("INTERRUPT") == "1"
	if interrupt {
		fmt.Println("=== 第1次运行（INTERRUPT=1）：传第1片后中断 ===")
	}

	f, err := c.Files().Upload(ctx, &file.UploadRequest{
		FilePath:     filePath,
		ParentFileID: "root",
		DriveID:      rd,
		Name:         "resume-sdk-test.bin",
		OnProgress: func(sent, total int64) {
			pct := float64(sent) * 100 / float64(total)
			fmt.Printf("\n  进度回调: %d/%d (%.1f%%)", sent, total, pct)
			// 第1片传完（10MB）后中断。
			if interrupt && sent >= 10*1024*1024 {
				fmt.Println("\n  >>> [模拟中断] 触发 os.Exit <<<")
				os.Exit(2)
			}
		},
	})
	if err != nil {
		fmt.Printf("\n上传失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\n🎉 上传完成: file_id=%s size=%d\n", f.FileID, f.Size)
}
