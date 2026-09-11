// 诊断：dump 夸克文件列表请求的完整 URL/header/响应，并测不同 path。
package main

import (
	"context"
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

	// 读 cookie。
	data, _ := os.ReadFile(os.Getenv("USERPROFILE") + `\.quark\cookie.json`)
	s := string(data)
	idx := strings.Index(s, `"cookie":"`)
	cookie := ""
	if idx >= 0 {
		rest := s[idx+len(`"cookie":"`):]
		end := strings.Index(rest, `"`)
		if end >= 0 {
			cookie = rest[:end]
		}
	}
	fmt.Printf("cookie 长度=%d 含__puus=%v\n\n", len(cookie), strings.Contains(cookie, "__puus="))

	// 测试不同 path 组合。
	cases := []struct {
		name string
		url  string
	}{
		{"file/sort + pr/fr", "https://drive-pc.quark.cn/1/clouddrive/file/sort?pr=uqm&fr=pc&pdir_fid=0&_page=1&_size=50&_fetch_total=1&_sort=file_type:asc,updated_at:desc"},
		{"file/sort 无pr", "https://drive-pc.quark.cn/1/clouddrive/file/sort?pdir_fid=0&_page=1&_size=50"},
		{"file/list", "https://drive-pc.quark.cn/1/clouddrive/file/list?pr=uqm&fr=pc&pdir_fid=0&_page=1&_size=50"},
	}

	for _, c := range cases {
		req, _ := http.NewRequestWithContext(ctx, "GET", c.url, nil)
		req.Header.Set("Cookie", cookie)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Referer", "https://pan.quark.cn/")
		req.Header.Set("Origin", "https://pan.quark.cn")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("[%s] err: %v\n", c.name, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		bs := string(body)
		if len(bs) > 250 {
			bs = bs[:250] + "..."
		}
		fmt.Printf("[%s] status=%d body=%s\n\n", c.name, resp.StatusCode, bs)
	}
}
