// 实测脚本 1：扫码登录。
// 把二维码生成 PNG 保存到桌面，用手机阿里云盘 App 扫描该图片。
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/langhuachuanshi/alipan-go/alipan"
	"github.com/langhuachuanshi/alipan-go/alipan/auth"

	qrcode "github.com/skip2/go-qrcode"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	auth.DebugLogin(true)

	fmt.Println("=== 测试 1：扫码登录（PNG 文件方式）===")
	c, err := alipan.New(ctx,
		alipan.WithName("test-account"),
		alipan.WithLoginTimeout(8*time.Minute),
		alipan.WithShowQR(func(content string) error {
			// 保存到项目 tmp 目录。
			tmpDir := projectTmpDir()
			pngPath := filepath.Join(tmpDir, "alipan-login-qrcode.png")
			if err := qrcode.WriteFile(content, qrcode.Medium, 384, pngPath); err != nil {
				return fmt.Errorf("写二维码PNG失败: %w", err)
			}
			fmt.Printf("二维码已保存到: %s\n", pngPath)
			fmt.Println("请用手机阿里云盘 App 扫描该图片，然后在手机上确认登录。")
			return nil
		}),
	)
	if err != nil {
		log.Fatalf("登录失败: %v", err)
	}

	user, err := c.Users().Get(ctx)
	if err != nil {
		log.Fatalf("获取用户信息失败: %v", err)
	}
	fmt.Printf("\n[OK] 登录成功！\n")
	fmt.Printf("  用户名: %s\n  昵称: %s\n  手机: %s\n  默认drive: %s\n",
		user.UserName, user.NickName, user.Phone, c.DefaultDriveID())

	// 验证 token 持久化。
	home, _ := os.UserHomeDir()
	tp := filepath.Join(home, ".aligo", "test-account.json")
	if _, err := os.Stat(tp); err == nil {
		fmt.Printf("[OK] token 已持久化到 %s\n", tp)
	} else {
		fmt.Printf("[WARN] token 文件未找到: %v\n", err)
	}
	fmt.Println("\n=== 测试 1 通过 ===")
}

// projectTmpDir 返回项目下的 tmp 目录（不存在则创建）。
// 以测试程序编译产物所在位置为基准向上找 go.mod 定位项目根。
func projectTmpDir() string {
	// 从当前工作目录（运行时通常是项目根）取 tmp。
	dir := "tmp"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "."
	}
	abs, _ := filepath.Abs(dir)
	return abs
}
