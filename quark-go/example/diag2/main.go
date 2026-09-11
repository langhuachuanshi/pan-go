// 诊断：研究夸克转存相关接口。
// 从 share URL 提取 share_id，测 get_share_stoken / detail 接口。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cookie := readCookie()

	// 用一个公开分享测试（无密码）。
	// 先测 get_share_stoken。
	shareID := "test_share_id" // 待用户提供真实分享链接后替换
	fmt.Println("=== 测 get_share_stoken（POST /1/clouddrive/share/sharepage/token）===")
	body, _ := json.Marshal(map[string]string{"share_id": shareID, "passcode": ""})
	do(ctx, cookie, "POST", "https://drive-pc.quark.cn/1/clouddrive/share/sharepage/token?pr=ucpro&fr=pc", body)
}

func do(ctx context.Context, cookie, method, url string, body []byte) {
	var r io.Reader
	if body != nil {
		r = strings.NewReader(string(body))
	}
	req, _ := http.NewRequestWithContext(ctx, method, url, r)
	req.Header.Set("Cookie", cookie)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://pan.quark.cn/")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("err:", err)
		return
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	fmt.Printf("status=%d body=%s\n", resp.StatusCode, string(data)[:min(400, len(string(data)))])
}

func readCookie() string {
	data, _ := os.ReadFile(os.Getenv("USERPROFILE") + `\.quark\cookie.json`)
	s := string(data)
	idx := strings.Index(s, `"cookie":"`)
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(`"cookie":"`):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
