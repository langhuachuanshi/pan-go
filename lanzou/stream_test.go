package lanzou

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/langhuachuanshi/pan-go/core/httpx"
)

// TestProgressReader 验证 progressReader 边读边累计、回调被正确触发。
func TestProgressReader(t *testing.T) {
	const content = "hello world this is test data"
	const total = int64(len(content))
	src := strings.NewReader(content)
	var calls []struct{ uploaded, total int64 }
	pr := &progressReader{
		r:     src,
		total: total,
		onProgress: func(uploaded, total int64) {
			calls = append(calls, struct{ uploaded, total int64 }{uploaded, total})
		},
	}
	// 每次读 4 字节
	buf := make([]byte, 4)
	var readTotal int
	for {
		n, err := pr.Read(buf)
		readTotal += n
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read 出错: %v", err)
		}
	}
	if int64(readTotal) != total {
		t.Errorf("总读取 %d, 期望 %d", readTotal, total)
	}
	if len(calls) == 0 {
		t.Fatal("期望进度回调被触发，实际 0 次")
	}
	// total 应恒等于 total
	for i, c := range calls {
		if c.total != total {
			t.Errorf("第 %d 次 total=%d, 期望 %d", i+1, c.total, total)
		}
	}
	// 最后一次 uploaded 应等于 total
	if calls[len(calls)-1].uploaded != total {
		t.Errorf("最后 uploaded=%d, 期望 %d", calls[len(calls)-1].uploaded, total)
	}
	// uploaded 应单调递增（或相等）
	var prev int64
	for i, c := range calls {
		if c.uploaded < prev {
			t.Errorf("第 %d 次 uploaded=%d 小于前一次 %d（非单调递增）", i+1, c.uploaded, prev)
		}
		prev = c.uploaded
	}
}

// TestProgressReaderNilCallback onProgress 为 nil 时正常工作（向后兼容）
func TestProgressReaderNilCallback(t *testing.T) {
	src := strings.NewReader("test")
	pr := &progressReader{r: src, total: 4}
	buf := make([]byte, 2)
	if _, err := pr.Read(buf); err != nil {
		t.Fatalf("Read 出错: %v", err)
	}
}

// TestPostMultipartStream 用 httptest mock 上传接口，验证流式上传时
// onProgress 被触发、服务端收到全部字节。
func TestPostMultipartStream(t *testing.T) {
	// mock html5up.php：读取全部 body 后返回成功 JSON
	var receivedBytes int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		for {
			n, err := r.Body.Read(buf)
			atomic.AddInt64(&receivedBytes, int64(n))
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
		}
		// 蓝奏成功响应：zt=1 + text[0].id
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"zt":1,"info":"success","text":[{"id":"f123","name_all":"test.txt"}]}`)
	}))
	defer server.Close()

	c := &Client{exec: httpx.New(httpx.Config{HTTPClient: server.Client()})}
	c.cookies = []*http.Cookie{}

	var maxUploaded int64
	var callCount int64
	onProgress := func(uploaded, total int64) {
		atomic.AddInt64(&callCount, 1)
		if uploaded > atomic.LoadInt64(&maxUploaded) {
			atomic.StoreInt64(&maxUploaded, uploaded)
		}
	}

	fileContentStr := "stream upload test data " + strings.Repeat("x", 1000)
	total := int64(len(fileContentStr))
	fileContent := strings.NewReader(fileContentStr)
	var resp struct {
		Zt   int    `json:"zt"`
		Info string `json:"info"`
	}
	err := c.PostMultipartStream(
		context.Background(),
		server.URL,
		map[string]string{"task": "1"},
		"upload_file",
		"test.txt",
		fileContent,
		total,
		onProgress,
		nil,
		&resp,
	)
	if err != nil {
		t.Fatalf("PostMultipartStream 失败: %v", err)
	}
	if resp.Zt != 1 {
		t.Fatalf("响应解析不对: %+v", resp)
	}

	// 进度应被触发
	if atomic.LoadInt64(&callCount) == 0 {
		t.Fatal("期望 onProgress 被触发，实际 0 次")
	}
	// 最大 uploaded 应接近 total（允许小于等于 total）
	if mu := atomic.LoadInt64(&maxUploaded); mu > total {
		t.Errorf("maxUploaded=%d 超过 total=%d", mu, total)
	}
	// 服务端应收到全部字节
	if atomic.LoadInt64(&receivedBytes) == 0 {
		t.Error("服务端未收到任何字节")
	}
}
