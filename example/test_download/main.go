// 实测下载：上传已知内容 → 下载 → 字节校验一致 → 清理。
package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/langhuachuanshi/panbaidu-go/baidu"
	"github.com/langhuachuanshi/panbaidu-go/baidu/download"
	"github.com/langhuachuanshi/panbaidu-go/baidu/file"
	"github.com/langhuachuanshi/panbaidu-go/baidu/upload"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	c, err := baidu.New(ctx, &baidu.Config{AppKey: os.Getenv("PANBAIDU_APP_KEY"), SecretKey: os.Getenv("PANBAIDU_SECRET_KEY")})
	if err != nil {
		log.Fatal(err)
	}

	content := []byte(fmt.Sprintf("panbaidu-go download test %s — 下载字节校验", time.Now().Format(time.RFC3339)))
	fileName := fmt.Sprintf("dl-test-%d.txt", time.Now().Unix())
	testDir := "/panbaidu-go-dl-test"

	// 0. 建临时目录。
	if _, err := c.Management().MakeDir(ctx, testDir); err != nil {
		log.Fatalf("建目录失败: %v", err)
	}
	defer c.Management().Delete(ctx, []string{testDir})

	// 1. 上传已知内容。
	fmt.Println("=== 1. 上传已知内容 ===")
	up, err := c.Upload().Upload(ctx, &upload.UploadRequest{
		ReaderAt: bytes.NewReader(content),
		FileName: fileName,
		Size:     int64(len(content)),
		DestPath: testDir,
	})
	if err != nil {
		log.Fatalf("上传失败: %v", err)
	}
	fsid := up.FSID
	// upload 返回的 fs_id 可能为 0（create 响应字段不全），用列表兜底。
	if fsid == 0 {
		fmt.Println("  (upload 未返回 fs_id，用列表查)")
		files, err := c.Files().List(ctx, &file.ListRequest{Dir: testDir, Limit: 50})
		if err != nil {
			log.Fatalf("列表查 fsid 失败: %v", err)
		}
		for _, f := range files {
			if f.Name() == fileName {
				fsid = f.FSID
				break
			}
		}
		if fsid == 0 {
			log.Fatalf("未能取得文件 fs_id")
		}
	}
	fmt.Printf("✓ 上传: %s (%d 字节, fs_id=%d)\n", fileName, len(content), fsid)

	// 2. GetDownloadURL。
	fmt.Println("\n=== 2. GetDownloadURL ===")
	url, err := c.Download().GetDownloadURL(ctx, fsid)
	if err != nil {
		log.Fatalf("拿 dlink 失败: %v", err)
	}
	fmt.Printf("✓ dlink: %.70s...\n", url)

	// 3. Download 到内存，带进度。
	fmt.Println("\n=== 3. Download ===")
	var buf bytes.Buffer
	err = c.Download().Download(ctx, &download.DownloadRequest{
		FSID:    fsid,
		Writer:  &buf,
		OnProgress: func(d, t int64) { fmt.Printf("\r  进度: %d / %d", d, t) },
	})
	if err != nil {
		log.Fatalf("\n下载失败: %v", err)
	}
	fmt.Printf("\n✓ 下载完成: %d 字节\n", buf.Len())

	// 4. 字节校验。
	fmt.Println("\n=== 4. 内容校验 ===")
	if !bytes.Equal(buf.Bytes(), content) {
		log.Fatalf("✗ 内容不一致！上传 %d 字节，下载 %d 字节", len(content), buf.Len())
	}
	fmt.Printf("✓ 字节级一致（%d 字节）\n", buf.Len())

	fmt.Println("\n=== 实测通过 ===")
}
