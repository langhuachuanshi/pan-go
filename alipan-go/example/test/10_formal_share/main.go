// 测试正式分享 /s/：用资源盘文件调 /adrive/v2/share_link/create。
// 之前 403 都是备份盘文件，资源盘还没测过这个接口。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tdata, _ := os.ReadFile(os.Getenv("USERPROFILE") + `\.aligo\test-account.json`)
	var tok struct {
		AccessToken string `json:"access_token"`
		XDeviceID   string `json:"x_device_id"`
	}
	json.Unmarshal(tdata, &tok)

	resourceDrive := "1802078720" // 资源盘
	// 资源盘里之前上传的文件。
	fileID := "6a31df6e50e71ee0aaa542ecb21daec41edf01d9"

	fmt.Printf("=== 正式分享测试（资源盘文件）===\n")
	fmt.Printf("drive=%s file=%s\n\n", resourceDrive, fileID)

	body, _ := json.Marshal(map[string]any{
		"drive_id":    resourceDrive,
		"file_id_list": []string{fileID},
		"share_pwd":   "1234",
		"expiration":  "",
	})

	hdr := http.Header{}
	hdr.Set("Authorization", tok.AccessToken)
	hdr.Set("User-Agent", "AliApp(AYSD/5.8.0) com.alicloud.databox/37029260 Channel/36176927979800@rimet_android_5.8.0 language/zh-CN /Android Mobile/Xiaomi Redmi")
	hdr.Set("Referer", "https://aliyundrive.com")
	hdr.Set("x-canary", "client=Android,app=adrive,version=v5.8.0")
	hdr.Set("x-device-id", tok.XDeviceID)
	hdr.Set("x-signature", "f4b7bed5d8524a04051bd2da876dd79afe922b8205226d65855d02b267422adb1e0d8a816b021eaf5c36d101892180f79df655c5712b348c2a540ca136e6b22001")
	hdr.Set("Content-Type", "application/json")

	req, _ := http.NewRequestWithContext(ctx, "POST",
		"https://api.aliyundrive.com/adrive/v2/share_link/create", bytesReader(body))
	req.Header = hdr
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("错误:", err)
		return
	}
	rbody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("status=%d\nbody=%s\n", resp.StatusCode, string(rbody))

	var r struct {
		ShareURL string `json:"share_url"`
		ShareID  string `json:"share_id"`
	}
	json.Unmarshal(rbody, &r)
	if r.ShareURL != "" {
		fmt.Printf("\n🎉 正式分享链接: %s\n", r.ShareURL)
	}
}

func bytesReader(b []byte) io.Reader { return &reader{b: b} }

type reader struct{ b []byte; i int }

func (r *reader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}
