// 实测：验证 CheckLogin 能正确识别有效 BDUSS（修复 /disk/home → /disk/main 改版跳转误判）。
//
// 运行：
//
//	# 方式1：环境变量
//	PANBAIDU_BDUSS=xxx PANBAIDU_STOKEN=yyy go run ./example/test_checklogin
//	# 方式2：直接读 ~/.090cq/baidu_cookie.txt（workbench 落盘格式）
//	go run ./example/test_checklogin
//
// 预期：对有效 BDUSS 返回 ok=true（修复前会被 /disk/main 重定向误判为失效）。
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/langhuachuanshi/baidupan-go/baidu"
)

func main() {
	bduss := os.Getenv("PANBAIDU_BDUSS")
	stoken := os.Getenv("PANBAIDU_STOKEN")

	// 兜底：从 workbench 的 cookie 文件读
	if bduss == "" {
		cookieStr := readCookieFile()
		bduss, stoken = parseCookie(cookieStr)
	}

	if bduss == "" {
		log.Fatal("未提供 BDUSS（用 PANBAIDU_BDUSS 环境变量，或确保 ~/.090cq/baidu_cookie.txt 存在）")
	}
	fmt.Printf("BDUSS len=%d STOKEN len=%d\n", len(bduss), len(stoken))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := baidu.New(ctx, &baidu.Config{BDUSS: bduss, STOKEN: stoken})
	if err != nil {
		log.Fatalf("SDK 初始化失败: %v", err)
	}

	ok, err := c.User().CheckLogin(ctx, c.HTTPClient())
	fmt.Printf("\nCheckLogin 结果: ok=%v err=%v\n", ok, err)
	if err != nil {
		fmt.Printf("err 详情: %s\n", err.Error())
	}

	if ok && err == nil {
		fmt.Println("\n=== 实测通过：有效 BDUSS 被正确识别 ===")
	} else {
		fmt.Println("\n=== 实测失败：有效 BDUSS 仍被误判为失效 ===")
		os.Exit(1)
	}
}

// readCookieFile 读 workbench 落盘的百度 cookie 文件。
func readCookieFile() string {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(home + string(os.PathSeparator) + ".090cq" + string(os.PathSeparator) + "baidu_cookie.txt")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// parseCookie 从 "k1=v1; k2=v2" 解析 BDUSS / STOKEN。
func parseCookie(s string) (bduss, stoken string) {
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		if i := strings.Index(part, "="); i > 0 {
			switch part[:i] {
			case "BDUSS":
				bduss = part[i+1:]
			case "STOKEN":
				stoken = part[i+1:]
			}
		}
	}
	return
}
