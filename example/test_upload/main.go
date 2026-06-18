// 实测上传 + 文件管理：建目录 → 上传文件 → 列表验证 → 重命名 → 移动 → 删除清理。
// 百度开放平台应用有专属目录（/apps/<应用名>/），先探测可写路径。
package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/langhuachuanshi/panbaidu-go/baidu"
	"github.com/langhuachuanshi/panbaidu-go/baidu/file"
	"github.com/langhuachuanshi/panbaidu-go/baidu/upload"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	c, err := baidu.New(ctx, &baidu.Config{BDUSS: os.Getenv("PANBAIDU_BDUSS"), STOKEN: os.Getenv("PANBAIDU_STOKEN")})
	if err != nil {
		log.Fatal(err)
	}

	// 网页端方案可在根目录建目录（不像开放平台需 /apps 专属目录）。
	testDir := "/panbaidu-go-test"

	// 1. 建测试目录。
	fmt.Println("=== 1. MakeDir ===")
	if _, err := c.Management().MakeDir(ctx, testDir); err != nil {
		// 根目录可能受限，尝试 /apps 下。
		fmt.Printf("根目录建目录失败（预期）: %v\n尝试 /apps 路径...\n", err)
		testDir = "/apps/panbaidu-go-test"
		if _, err := c.Management().MakeDir(ctx, testDir); err != nil {
			log.Fatalf("建目录失败: %v", err)
		}
	}
	fmt.Printf("✓ 建目录 %s\n", testDir)

	// 2. 上传文件。
	fmt.Println("\n=== 2. Upload ===")
	content := []byte(fmt.Sprintf("panbaidu-go upload test %s", time.Now().Format(time.RFC3339)))
	fileName := fmt.Sprintf("upload-test-%d.txt", time.Now().Unix())
	up, err := c.Upload().Upload(ctx, &upload.UploadRequest{
		ReaderAt: bytes.NewReader(content),
		FileName: fileName,
		Size:     int64(len(content)),
		DestPath: testDir,
	})
	if err != nil {
		log.Fatalf("上传失败: %v", err)
	}
	fmt.Printf("✓ 上传成功: %s (fs_id=%d, size=%d)\n", fileName, up.FSID, up.Size)

	// 3. 列表验证。
	fmt.Println("\n=== 3. List 验证 ===")
	files, err := c.Files().List(ctx, &file.ListRequest{Dir: testDir, Num: 50})
	if err != nil {
		log.Fatalf("列测试目录失败: %v", err)
	}
	found := false
	for _, f := range files {
		fmt.Printf("  %s (%d 字节)\n", f.Name(), f.Size)
		if f.Name() == fileName {
			found = true
		}
	}
	if !found {
		log.Fatalf("✗ 列表未找到上传的文件 %s", fileName)
	}
	fmt.Printf("✓ 列表验证通过\n")

	// 4. 重命名。
	fmt.Println("\n=== 4. Rename ===")
	oldPath := testDir + "/" + fileName
	newName := "renamed-" + fileName
	if err := c.Management().Rename(ctx, oldPath, testDir+"/"+newName); err != nil {
		log.Fatalf("重命名失败: %v", err)
	}
	fmt.Printf("✓ 重命名为 %s\n", newName)

	// 5. 移动到测试目录根（演示 move：建子目录再移入）。
	fmt.Println("\n=== 5. Move ===")
	subDir := testDir + "/sub"
	if _, err := c.Management().MakeDir(ctx, subDir); err != nil {
		log.Fatalf("建子目录失败: %v", err)
	}
	if err := c.Management().Move(ctx, []string{testDir + "/" + newName}, subDir); err != nil {
		log.Fatalf("移动失败: %v", err)
	}
	fmt.Printf("✓ 移动到 %s\n", subDir)

	// 6. 删除整个测试目录。
	fmt.Println("\n=== 6. Delete（清理）===")
	if err := c.Management().Delete(ctx, []string{testDir}); err != nil {
		log.Fatalf("删除失败: %v", err)
	}
	fmt.Printf("✓ 已删除 %s\n", testDir)

	fmt.Println("\n=== 实测通过 ===")
}
