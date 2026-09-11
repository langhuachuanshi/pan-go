// 实测上传：本机造一个测试文件 → 上传到夸克根目录 → 列文件验证已出现。
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/langhuachuanshi/quark-go/quark"
	"github.com/langhuachuanshi/quark-go/quark/file"
	"github.com/langhuachuanshi/quark-go/quark/upload"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	c, err := quark.New(ctx)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	// 1. 造一个本地测试文件（带唯一内容，避免和已有文件撞名/秒传干扰首次验证）。
	fileName := fmt.Sprintf("quark-go-upload-test-%d.txt", time.Now().Unix())
	content := fmt.Sprintf("quark-go 上传实测 %s\n这是一段测试内容。", time.Now().Format(time.RFC3339))
	tmpPath := filepath.Join(os.TempDir(), fileName)
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		log.Fatalf("写临时文件失败: %v", err)
	}
	defer os.Remove(tmpPath)

	f, err := os.Open(tmpPath)
	if err != nil {
		log.Fatalf("打开临时文件失败: %v", err)
	}
	defer f.Close()
	stat, _ := f.Stat()

	fmt.Printf("测试文件: %s (%d 字节)\n\n", fileName, stat.Size())

	// 2. 上传到根目录，带进度回调。
	fmt.Println("=== 开始上传 ===")
	result, err := c.Upload().Upload(ctx, &upload.UploadRequest{
		ReaderAt: f,
		FileName: fileName,
		Size:     stat.Size(),
		PDirFID:  "0",
		OnProgress: func(uploaded, total int64) {
			fmt.Printf("\r进度: %d / %d 字节", uploaded, total)
		},
	})
	if err != nil {
		log.Fatalf("\n上传失败: %v", err)
	}
	fmt.Printf("\n上传完成: fid=%s\n\n", result.FID)

	// 3. 验证：列根目录，确认新文件已出现（夸克列表 finish 后有秒级延迟，重试几次）。
	fmt.Println("=== 验证：根目录文件列表 ===")
	var found bool
	for attempt := 1; attempt <= 5 && !found; attempt++ {
		time.Sleep(2 * time.Second)
		files, err := c.Files().List(ctx, &file.ListRequest{PDirFID: "0", Size: 50})
		if err != nil {
			log.Fatalf("列文件失败: %v", err)
		}
		for _, fl := range files {
			if fl.FileName == fileName {
				found = true
				fmt.Printf("✓ 找到刚上传的文件: %s (size=%d, fid=%s)\n", fl.FileName, fl.Size, fl.FID)
				break
			}
		}
		if !found {
			fmt.Printf("（第 %d 次列表未见，重试中...）\n", attempt)
		}
	}
	if !found {
		log.Fatalf("✗ 上传返回 fid=%s 且 commit 成功，但列表 5 次重试均未出现", result.FID)
	}

	fmt.Println("\n=== 实测通过 ===")
}

