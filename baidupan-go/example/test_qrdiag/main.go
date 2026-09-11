// 扫码登录诊断版：本地页面放大展示官方二维码（225px 原图太小扫不出），
// 打印每次轮询的状态变化（status/v），窗口 5 分钟。
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/langhuachuanshi/baidupan-go/baidu/qrcode"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	login, err := qrcode.Create(ctx)
	if err != nil {
		log.Fatalf("创建二维码失败: %v", err)
	}
	fmt.Println("官方二维码图片: " + login.ImgURL)
	fmt.Println("wappass 内容链接: " + login.QRURL)

	// 本地页面放大展示（原图 225x225，相机难对焦）
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><title>百度登录</title>
<body style="margin:0;background:#fff;display:flex;justify-content:center;align-items:center;min-height:100vh">
<img src="%s" style="width:480px;height:480px;image-rendering:pixelated;border:16px solid #fff"></body>`, login.ImgURL)
	})
	go func() { _ = http.ListenAndServe("127.0.0.1:18923", nil) }()
	fmt.Println("扫码页面: http://127.0.0.1:18923/")

	fmt.Println("开始轮询（最长 5 分钟，每 5 秒一次，状态变化才打印）...")
	last := ""
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		status, v, err := login.Probe(ctx)
		if err != nil {
			log.Fatalf("轮询出错: %v", err)
		}
		cur := fmt.Sprintf("status=%d v=%s", status, v)
		if cur != last {
			fmt.Printf("[%s] %s\n", time.Now().Format("15:04:05"), cur)
			last = cur
		}
		if status == 0 && v != "" {
			cred, err := login.Exchange(ctx)
			if err != nil {
				log.Fatalf("换发失败: %v", err)
			}
			fmt.Printf("✅ 登录成功 BDUSS=%d字符 STOKEN=%d字符\n", len(cred.BDUSS), len(cred.STOKEN))
			return
		}
		time.Sleep(5 * time.Second)
	}
	fmt.Println("超时未收到确认事件")
}
