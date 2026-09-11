// 实测百度扫码登录：Create 出二维码（官方图片 URL + 内容短链）→ Wait 轮询换发
// BDUSS/STOKEN → 用新凭证调 quota 接口验证。
//
// 不落盘、不打印凭证值，只打印验证结果。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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

	// 原图 225x225 太小相机难识别，本地页面放大展示
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><title>百度登录</title>
<body style="margin:0;background:#fff;display:flex;justify-content:center;align-items:center;min-height:100vh">
<img src="%s" style="width:480px;height:480px;image-rendering:pixelated;border:16px solid #fff"></body>`, login.ImgURL)
	})
	go func() { _ = http.ListenAndServe("127.0.0.1:18923", nil) }()
	fmt.Println("扫码页面: http://127.0.0.1:18923/")
	fmt.Println("用百度App扫码（放大页面），等待确认...")

	cred, err := login.Wait(ctx, 5*time.Minute)
	if err != nil {
		log.Fatalf("扫码登录失败: %v", err)
	}
	fmt.Printf("\n登录成功：BDUSS %d 字符、STOKEN %d 字符\n", len(cred.BDUSS), len(cred.STOKEN))

	// 验证：quota 接口（只需 BDUSS）
	q := url.Values{"checkfree": {"1"}, "checkexpire": {"1"},
		"channel": {"chunlei"}, "web": {"1"}, "app_id": {"250528"}, "clienttype": {"0"},
		"bdstoken": {""}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://pan.baidu.com/api/quota?"+q.Encode(), nil)
	req.Header.Set("Cookie", "BDUSS="+cred.BDUSS+"; STOKEN="+cred.STOKEN)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		log.Fatalf("验证请求失败: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Errno int `json:"errno"`
		Data  struct {
			Total int64 `json:"total"`
			Used  int64 `json:"used"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		log.Fatalf("解析 quota 失败: %s", string(body[:min(len(body), 150)]))
	}
	if out.Errno != 0 {
		log.Fatalf("验证失败 errno=%d（STOKEN 缺失或凭证无效）: %s", out.Errno, string(body[:min(len(body), 150)]))
	}
	fmt.Printf("✅ 验证通过：空间总量 %.1fTB 已用 %.1fGB\n",
		float64(out.Data.Total)/1e12, float64(out.Data.Used)/1e9)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
