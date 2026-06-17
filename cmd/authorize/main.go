// 授权辅助工具：打印授权链接，用户授权后输入 code，换取并持久化 token。
//
// 密钥通过环境变量传入（不写进代码）：
//   PANBAIDU_APP_KEY=xxx
//   PANBAIDU_SECRET_KEY=xxx
//
// 用法：
//   go run ./cmd/authorize
//   → 打印链接，浏览器打开，授权后页面显示 code（oob 模式）
//   → 把 code 粘贴回终端
//   → 自动换 token 并保存到 ~/.panbaidu/token.json
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/langhuachuanshi/panbaidu-go/baidu/auth"
)

func main() {
	appKey := os.Getenv("PANBAIDU_APP_KEY")
	secretKey := os.Getenv("PANBAIDU_SECRET_KEY")
	if appKey == "" || secretKey == "" {
		log.Fatal("请设置环境变量 PANBAIDU_APP_KEY 和 PANBAIDU_SECRET_KEY")
	}

	mgr := auth.New(&auth.Config{
		AppKey:     appKey,
		SecretKey:  secretKey,
		RedirectURI: "oob",
	})

	fmt.Println("请在浏览器打开以下链接，登录百度账号并同意授权：")
	fmt.Println()
	fmt.Println(mgr.AuthorizeURL())
	fmt.Println()
	fmt.Print("授权后页面会显示一个 code，请粘贴到这里并回车：")

	reader := bufio.NewReader(os.Stdin)
	code, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("读取 code 失败: %v", err)
	}
	// 去除换行和首尾空白。
	code = trim(code)
	if code == "" {
		log.Fatal("code 为空")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	token, err := mgr.ExchangeToken(ctx, code)
	if err != nil {
		log.Fatalf("换 token 失败: %v", err)
	}
	fmt.Printf("\n✓ 授权成功！\n")
	fmt.Printf("  access_token: %s...（已保存）\n", token.AccessToken[:min(20, len(token.AccessToken))])
	fmt.Printf("  有效期: %d 秒（约 %d 天）\n", token.ExpiresIn, token.ExpiresIn/86400)
	fmt.Printf("  已持久化到 ~/.panbaidu/token.json\n")
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\r' || s[0] == '\n') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\r' || s[len(s)-1] == '\n') {
		s = s[:len(s)-1]
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
