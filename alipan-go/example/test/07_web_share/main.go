// 测试 web 端创建分享：纯浏览器 header，不带 x-signature/x-device-id。
// 验证网页端是否走另一套鉴权（不要 App 签名）。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type tokenFile struct {
	AccessToken    string `json:"access_token"`
	UserID         string `json:"user_id"`
	DefaultDriveID string `json:"default_drive_id"`
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tdata, _ := os.ReadFile(os.Getenv("USERPROFILE") + `\.aligo\test-account.json`)
	var tok tokenFile
	json.Unmarshal(tdata, &tok)
	fmt.Printf("user=%s drive=%s\n", tok.UserID, tok.DefaultDriveID)

	// 找一个文件（用纯 web header 列文件）。
	fileID := listFile(ctx, tok.AccessToken, tok.DefaultDriveID)
	if fileID == "" {
		// 回退：用之前测试上传的已知文件（在 alipan-go-share-test 文件夹内）。
		fileID = "6a31d6a3decf25ca11764fcf84e6b01238379aee"
		fmt.Printf("回退用已知文件: %s\n", fileID)
	} else {
		fmt.Printf("测试文件: %s\n", fileID)
	}

	body, _ := json.Marshal(map[string]any{
		"drive_id":    tok.DefaultDriveID,
		"file_id_list": []string{fileID},
		"share_pwd":   "1234",
		"expiration":  "",
	})

	// 对照两个 host × 纯浏览器 header。
	webHeaders := http.Header{
		"Authorization": {tok.AccessToken},
		"User-Agent":    {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"},
		"Referer":       {"https://www.alipan.com/"},
		"Origin":        {"https://www.alipan.com"},
		"x-canary":      {"client=web,app=adrive,version=v3.17.0"},
		"Content-Type":  {"application/json"},
	}
	for _, host := range []string{
		"https://api.aliyundrive.com",
		"https://api.alipan.com",
	} {
		url := host + "/adrive/v2/share_link/create"
		req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		// 复制 header。
		req.Header = http.Header{}
		for k, vs := range webHeaders {
			for _, v := range vs {
				req.Header.Set(k, v)
			}
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("[%s] 错误: %v\n", host, err)
			continue
		}
		rbody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		msg := ""
		var m map[string]any
		if json.Unmarshal(rbody, &m) == nil {
			if dm, ok := m["display_message"]; ok {
				msg = fmt.Sprintf("%v", dm)
			}
		}
		fmt.Printf("[%s] status=%d msg=%s\n", host, resp.StatusCode, msg)
	}
}

func listFile(ctx context.Context, at, driveID string) string {
	body, _ := json.Marshal(map[string]any{"drive_id": driveID, "parent_file_id": "root", "limit": 50})
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.aliyundrive.com/adrive/v3/file/list", bytes.NewReader(body))
	req.Header.Set("Authorization", at)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.alipan.com/")
	req.Header.Set("x-canary", "client=web,app=adrive,version=v3.17.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("list 错误:", err)
		return ""
	}
	defer resp.Body.Close()
	rbody, _ := io.ReadAll(resp.Body)
	fmt.Printf("list status=%d body=%s\n", resp.StatusCode, string(rbody)[:min(200, len(string(rbody)))])
	if resp.StatusCode != 200 {
		return ""
	}
	var r struct {
		Items []struct {
			FileID string `json:"file_id"`
			Type   string `json:"type"`
			Name   string `json:"name"`
		} `json:"items"`
	}
	json.Unmarshal(rbody, &r)
	for _, f := range r.Items {
		if f.Type == "file" {
			return f.FileID
		}
	}
	// 根目录若全是文件夹，返回第一个文件夹让用户知道有内容。
	if len(r.Items) > 0 {
		fmt.Printf("  根目录有 %d 项，但无文件（只有文件夹）\n", len(r.Items))
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
