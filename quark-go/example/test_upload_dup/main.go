// 实测同一目录下重复上传的行为（断点重跑场景）：
//  1. 同名同内容传两次 → 第二次是秒传复用、报错、还是建出重复文件？
//  2. 同名不同内容 → 覆盖、报错、还是并存？
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/langhuachuanshi/quark-go/quark"
	"github.com/langhuachuanshi/quark-go/quark/file"
	"github.com/langhuachuanshi/quark-go/quark/upload"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	c, err := quark.New(ctx)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	dirFID, err := ensureDir(ctx, c, "0", "上传重复测试-990")
	if err != nil {
		log.Fatalf("建目录失败: %v", err)
	}
	fmt.Printf("目录 上传重复测试-990 fid=%s\n\n", dirFID)

	// 准备两个内容不同的临时文件（1MB）
	mk := func(name string, seed int64) string {
		p, _ := os.CreateTemp("", "quark-dup-*")
		buf := make([]byte, 1<<20)
		rand.New(rand.NewSource(seed)).Read(buf)
		p.Write(buf)
		p.Close()
		_ = os.Rename(p.Name(), p.Name()+name) // 不影响，文件名以上传参数为准
		return p.Name() + name
	}
	fileA := mk("a.bin", 1)

	up := func(path, name string) (string, error) {
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer f.Close()
		st, _ := f.Stat()
		res, err := c.Upload().Upload(ctx, &upload.UploadRequest{
			ReaderAt: f, FileName: name, Size: st.Size(), PDirFID: dirFID,
		})
		if err != nil {
			return "", err
		}
		return res.FID, nil
	}

	fmt.Println("=== 1. 同名同内容连传两次 ===")
	fid1, err := up(fileA, "same.bin")
	fmt.Printf("第1次: fid=%s err=%v\n", fid1, err)
	fid2, err := up(fileA, "same.bin")
	fmt.Printf("第2次: fid=%s err=%v\n", fid2, err)
	fmt.Printf("同 fid=%v\n\n", fid1 == fid2 && fid1 != "")

	fmt.Println("=== 2. 同名不同内容 ===")
	fileB := mk("b.bin", 2)
	fid3, err := up(fileB, "same.bin")
	fmt.Printf("异内容同名: fid=%s err=%v\n", fid3, err)

	// 数目录里的 same.bin
	files, _ := c.Files().List(ctx, &file.ListRequest{PDirFID: dirFID, Size: 100})
	count := 0
	for _, f := range files {
		if f.FileName == "same.bin" {
			count++
		}
	}
	fmt.Printf("目录里 same.bin 文件数=%d\n\n", count)

	fmt.Println("=== 3. 清理 ===")
	if err := c.Files().Delete(ctx, []string{dirFID}); err != nil {
		fmt.Printf("清理失败（请手动删 上传重复测试-990）: %v\n", err)
	} else {
		fmt.Println("已清理")
	}
	os.Remove(fileA)
	os.Remove(fileB)
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
