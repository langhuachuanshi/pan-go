package baidu_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	bd "github.com/langhuachuanshi/pan-go/baidu"
	"github.com/langhuachuanshi/pan-go/baidu/file"
)

// loadClient 从 ~/.090cq/baidu_cookie.txt 读 cookie 构造 client。
func loadClient(t *testing.T) *bd.Client {
	t.Helper()
	data, err := os.ReadFile(os.Getenv("USERPROFILE") + `\.090cq\baidu_cookie.txt`)
	if err != nil {
		t.Skip("无百度 cookie，跳过真实 API 测试")
	}
	cookie := strings.TrimSpace(string(data))
	var bduss, stoken string
	for _, p := range strings.Split(cookie, ";") {
		p = strings.TrimSpace(p)
		if i := strings.Index(p, "="); i > 0 {
			if p[:i] == "BDUSS" { bduss = p[i+1:] }
			if p[:i] == "STOKEN" { stoken = p[i+1:] }
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c, err := bd.New(ctx, &bd.Config{BDUSS: bduss, STOKEN: stoken})
	if err != nil { t.Fatalf("SDK init: %v", err) }
	return c
}

// TestMakeDirIfNotExist_AlreadyExists 验证：对已存在目录调 MakeDirIfNotExist
// 返回 nil（不创建），且不产生带后缀的重复目录。
//
// 对比 MakeDir（盲建）：对已存在目录调 MakeDir 会触发百度自动改名创建
// _20260719_xxx 垃圾目录。MakeDirIfNotExist 先 List 查存在，避免触发。
func TestMakeDirIfNotExist_AlreadyExists(t *testing.T) {
	c := loadClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	existing := "/090传奇/传奇素材"
	before := countSubDirs(t, c, "/090传奇")

	// 连续调 3 次 MakeDirIfNotExist，对已存在目录应都返回 nil 且不产生重复
	for i := 0; i < 3; i++ {
		f, err := c.Management().MakeDirIfNotExist(ctx, existing)
		if err != nil {
			t.Fatalf("第 %d 次 MakeDirIfNotExist 失败: %v", i+1, err)
		}
		if f != nil {
			t.Logf("第 %d 次返回非 nil（已存在应返回 nil）: %+v", i+1, f)
		}
	}

	after := countSubDirs(t, c, "/090传奇")
	if after != before {
		t.Errorf("产生重复目录：before=%d after=%d（应不变）", before, after)
	} else {
		t.Logf("✓ 连续 3 次无重复目录（before=%d after=%d）", before, after)
	}
}

// TestMakeDirIfNotExist_CreateNew 验证：对不存在目录调 MakeDirIfNotExist 会创建。
func TestMakeDirIfNotExist_CreateNew(t *testing.T) {
	c := loadClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	newDir := "/090传奇/传奇素材/_mkdirttest_new"
	// 清理（以防残留）
	c.Management().Delete(ctx, []string{newDir})

	f, err := c.Management().MakeDirIfNotExist(ctx, newDir)
	if err != nil {
		t.Fatalf("创建新目录失败: %v", err)
	}
	if f == nil {
		t.Fatal("创建新目录返回 nil（应返回新目录信息）")
	}
	if f.Path != newDir {
		t.Errorf("新目录 path=%s，期望 %s", f.Path, newDir)
	}
	t.Logf("✓ 创建成功: %s", f.Path)

	// 再调一次应返回 nil（已存在）
	f2, err := c.Management().MakeDirIfNotExist(ctx, newDir)
	if err != nil {
		t.Fatalf("第二次调用失败: %v", err)
	}
	if f2 != nil {
		t.Errorf("已存在应返回 nil，实际 %+v", f2)
	}

	// 清理
	c.Management().Delete(ctx, []string{newDir})
}

func countSubDirs(t *testing.T, c *bd.Client, dir string) int {
	t.Helper()
	items, err := c.Files().List(context.Background(), &file.ListRequest{Dir: dir, Num: 1000})
	if err != nil { t.Fatalf("List %s: %v", dir, err) }
	n := 0
	for _, f := range items {
		if f.IsFolder() { n++ }
	}
	return n
}
