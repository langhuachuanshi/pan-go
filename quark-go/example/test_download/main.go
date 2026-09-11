// 实测下载：上传一段已知内容 → 下载 → 校验内容一致 → 清理。
// 用内容比对而非只看大小，能验证下载字节级正确。
package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/langhuachuanshi/quark-go/quark"
	"github.com/langhuachuanshi/quark-go/quark/download"
	"github.com/langhuachuanshi/quark-go/quark/upload"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	c, err := quark.New(ctx)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	// 准备一段已知内容（带可识别标记）。
	content := []byte(fmt.Sprintf("quark-go download test content %s — 下载校验用", time.Now().Format(time.RFC3339)))
	fileName := fmt.Sprintf("quark-go-download-test-%d.txt", time.Now().Unix())

	// 1. 上传。
	fmt.Println("=== 1. 上传已知内容 ===")
	readerAt := bytes.NewReader(content)
	up, err := c.Upload().Upload(ctx, &upload.UploadRequest{
		ReaderAt: readerAt,
		FileName: fileName,
		Size:     int64(len(content)),
		PDirFID:  "0",
	})
	if err != nil {
		log.Fatalf("上传失败: %v", err)
	}
	fmt.Printf("✓ 上传完成: %s fid=%s (%d 字节)\n", fileName, up.FID, len(content))
	fid := up.FID
	defer func() {
		// 清理：删除测试文件。
		if err := c.Files().Delete(ctx, []string{fid}); err != nil {
			log.Printf("清理失败(非致命): %v", err)
		} else {
			fmt.Println("\n✓ 已清理测试文件")
		}
	}()

	// 2. GetDownloadURL：先单独验证拿链接。
	fmt.Println("\n=== 2. GetDownloadURL ===")
	url, err := c.Download().GetDownloadURL(ctx, fid)
	if err != nil {
		log.Fatalf("拿下载链接失败: %v", err)
	}
	fmt.Printf("✓ 拿到下载链接: %.80s...\n", url)

	// 3. Download 到内存 buffer，带进度。
	fmt.Println("\n=== 3. Download ===")
	var buf bytes.Buffer
	err = c.Download().Download(ctx, &download.DownloadRequest{
		FID:    fid,
		Writer: &buf,
		OnProgress: func(downloaded, total int64) {
			fmt.Printf("\r  下载进度: %d / %d 字节", downloaded, total)
		},
	})
	if err != nil {
		log.Fatalf("\n下载失败: %v", err)
	}
	fmt.Printf("\n✓ 下载完成: %d 字节\n", buf.Len())

	// 4. 校验：下载内容必须与上传内容字节一致。
	fmt.Println("\n=== 4. 内容校验 ===")
	if !bytes.Equal(buf.Bytes(), content) {
		log.Fatalf("✗ 内容不一致！上传 %d 字节，下载 %d 字节", len(content), buf.Len())
	}
	fmt.Printf("✓ 内容字节级一致（%d 字节）\n", buf.Len())

	fmt.Println("\n=== 实测通过 ===")
}
