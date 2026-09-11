// 实测端到端：登录检查 → 逐级建目录（getOrCreate 编排，镜像工具同款逻辑）
// → 上传单文件 → 文件分享（带提取码）→ 文件夹分享（决策前提，未实测过）→ 清理。
//
// 前置：~/.quark/cookie.json 存在（pan.quark.cn 登录后 F12 复制完整 cookie）。
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
	"github.com/langhuachuanshi/quark-go/quark/share"
	"github.com/langhuachuanshi/quark-go/quark/upload"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	c, err := quark.New(ctx) // 默认读 ~/.quark/cookie.json
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	// 1. 登录检查：列根目录
	fmt.Println("=== 1. 登录检查（列根目录） ===")
	rootFiles, err := c.Files().List(ctx, &file.ListRequest{PDirFID: "0", Size: 100})
	if err != nil {
		log.Fatalf("登录态无效或列目录失败: %v", err)
	}
	fmt.Printf("登录有效，根目录 %d 项\n", len(rootFiles))

	// 2. 逐级建目录（getOrCreate：先 List 查同名复用，没有才 MakeDir——绕开 MakeDir 重名拿错 fid 的坑）
	fmt.Println("\n=== 2. 逐级建目录 090传奇/传奇素材/测试分类/990测试 ===")
	cur := "0"
	for _, name := range []string{"090传奇", "传奇素材", "测试分类", "990测试"} {
		cur, err = ensureDir(ctx, c, cur, name)
		if err != nil {
			log.Fatalf("建目录 %s 失败: %v", name, err)
		}
		fmt.Printf("  目录 %q fid=%s\n", name, cur)
	}
	testDirFID := cur

	// 3. 上传单文件（本地临时 2MB 随机数据，真实分片链路）
	fmt.Println("\n=== 3. 上传单文件（2MB 临时文件） ===")
	tmp, err := os.CreateTemp("", "quark-e2e-*.bin")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	buf := make([]byte, 2<<20)
	rand.New(rand.NewSource(time.Now().UnixNano())).Read(buf)
	if _, err := tmp.Write(buf); err != nil {
		log.Fatal(err)
	}
	tmp.Close()

	f, err := os.Open(tmp.Name())
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	st, _ := f.Stat()

	start := time.Now()
	uploaded, err := c.Upload().Upload(ctx, &upload.UploadRequest{
		ReaderAt: f,
		FileName: "e2e-test.bin",
		Size:     st.Size(),
		PDirFID:  testDirFID,
		OnProgress: func(done, total int64) {
			if done == total {
				fmt.Printf("  上传完成 %.1fMB 用时 %s\n", float64(total)/1048576, time.Since(start).Round(time.Millisecond))
			}
		},
	})
	if err != nil {
		log.Fatalf("上传失败: %v", err)
	}
	fmt.Printf("  上传成功 fid=%s\n", uploaded.FID)

	// 4. 文件分享（永久 + 4位提取码）
	fmt.Println("\n=== 4. 文件分享（永久+提取码） ===")
	fs1, err := c.Share().Create(ctx, &share.CreateRequest{
		FIDs:         []string{uploaded.FID},
		Title:        "quark-go-e2e-file",
		Forever:      true,
		WithPasscode: true,
	})
	if err != nil {
		log.Fatalf("文件分享失败: %v", err)
	}
	fmt.Printf("  文件分享: %s 提取码=%s\n", fs1.ShareURL, fs1.Passcode)

	// 5. 文件夹分享（关键验证：夸克原生能力但库未实测过）
	fmt.Println("\n=== 5. 文件夹分享（永久+提取码） ===")
	fs2, err := c.Share().Create(ctx, &share.CreateRequest{
		FIDs:         []string{testDirFID},
		Title:        "quark-go-e2e-folder",
		Forever:      true,
		WithPasscode: true,
	})
	if err != nil {
		log.Fatalf("文件夹分享失败: %v", err)
	}
	fmt.Printf("  文件夹分享: %s 提取码=%s\n", fs2.ShareURL, fs2.Passcode)

	// 6. 清理：删除 990测试 目录（连带上传的测试文件）
	fmt.Println("\n=== 6. 清理测试目录 ===")
	if err := c.Files().Delete(ctx, []string{testDirFID}); err != nil {
		fmt.Printf("  清理失败（请手动删 090传奇/传奇素材/测试分类/990测试）: %v\n", err)
	} else {
		fmt.Println("  已删除")
	}

	fmt.Println("\n✅ e2e 全部通过：登录 / 建目录 / 上传 / 文件分享 / 文件夹分享")
}

// ensureDir getOrCreate：先列父目录找同名文件夹，找到复用 fid；没有才 MakeDir。
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
