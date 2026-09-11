package auth

import (
	"context"
	"fmt"
	"net"
	"net/http"

	qrcode "github.com/skip2/go-qrcode"
)

// 本文件实现二维码展示：终端打印 + 网页。对应该 aligo _show_console / _show_qrcode_in_web。

// showQrcodeTerminal 在终端打印二维码 + 提示。
func showQrcodeTerminal(content string) {
	qr, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		fmt.Printf("生成二维码失败，请改用网页方式: %v\n二维码内容: %s\n", err, content)
		return
	}
	fmt.Println("请使用阿里云盘 App 扫描以下二维码登录：")
	fmt.Println(qr.ToString(false))
	fmt.Println("(扫描成功后请在手机上确认登录)")
}

// showQrcodeWeb 起本地 HTTP 服务，浏览器访问 http://<本机IP>:<port> 扫码。
func (s *loginSession) showQrcodeWeb(ctx context.Context, port int, content string) error {
	png, err := qrcode.Encode(content, qrcode.Medium, 256)
	if err != nil {
		return fmt.Errorf("alipan: encode qrcode png failed: %w", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta charset="utf-8"><title>扫码登录阿里云盘</title></head>
<body style="display:flex;align-items:center;justify-content:center;height:100vh;margin:0;font-family:sans-serif;">
<div style="text-align:center"><h2>请使用阿里云盘 App 扫码登录</h2><img src="/login.png"/></div>
</body></html>`)
	})
	mux.HandleFunc("/login.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	})

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("alipan: listen %s failed: %w", addr, err)
	}
	srv := &http.Server{Handler: mux}
	s.webServer = srv
	fmt.Printf("网页扫码登录服务已启动，请用浏览器访问: http://localhost:%d\n", port)

	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	go func() {
		_ = srv.Serve(ln)
	}()
	return nil
}

// stopWeb 关闭网页登录服务。
func (s *loginSession) stopWeb() {
	if s.webServer != nil {
		_ = s.webServer.Close()
		s.webServer = nil
	}
}
