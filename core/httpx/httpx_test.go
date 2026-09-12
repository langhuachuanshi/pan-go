package httpx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestGetWithQuery GET：query 拼接、默认 UA 注入、响应读取。
func TestGetWithQuery(t *testing.T) {
	var gotPath, gotUA, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotUA, gotQuery = r.URL.Path, r.Header.Get("User-Agent"), r.URL.RawQuery
		io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	e := New(Config{})
	resp, err := e.Do(context.Background(), &Request{
		URL:   srv.URL + "/file/sort",
		Query: urlValues(map[string]string{"_page": "1", "_size": "50"}),
	})
	if err != nil {
		t.Fatalf("Do 出错: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 %d", resp.StatusCode)
	}
	if gotPath != "/file/sort" || !strings.Contains(gotQuery, "_page=1") || !strings.Contains(gotQuery, "_size=50") {
		t.Fatalf("路径/查询不对: %s?%s", gotPath, gotQuery)
	}
	if gotUA != defaultUA {
		t.Fatalf("默认 UA 未注入: %q", gotUA)
	}
	if string(resp.Body) != `{"ok":true}` {
		t.Fatalf("响应体 %q", resp.Body)
	}
}

// TestPostJSON POST JSON：Content-Type 与请求体编码。
func TestPostJSON(t *testing.T) {
	var gotCT, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotCT, gotBody = r.Header.Get("Content-Type"), string(b)
		io.WriteString(w, `{"code":0}`)
	}))
	defer srv.Close()

	_, err := New(Config{}).Do(context.Background(), &Request{
		Method: http.MethodPost,
		URL:    srv.URL,
		Body:   JSONBody{V: map[string]any{"fid_list": []string{"a", "b"}}},
	})
	if err != nil {
		t.Fatalf("Do 出错: %v", err)
	}
	if !strings.Contains(gotCT, "application/json") {
		t.Fatalf("Content-Type %q", gotCT)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(gotBody), &parsed); err != nil {
		t.Fatalf("请求体不是合法 JSON: %q", gotBody)
	}
}

// TestPostForm POST 表单：Content-Type 与表单编码。
func TestPostForm(t *testing.T) {
	var gotCT string
	var gotTask, gotFolder string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		r.ParseForm()
		gotTask, gotFolder = r.PostForm.Get("task"), r.PostForm.Get("folder_id")
		io.WriteString(w, `{"zt":1}`)
	}))
	defer srv.Close()

	_, err := New(Config{}).Do(context.Background(), &Request{
		Method: http.MethodPost,
		URL:    srv.URL,
		Body:   FormBody{"task": "5", "folder_id": "-1"},
	})
	if err != nil {
		t.Fatalf("Do 出错: %v", err)
	}
	if !strings.Contains(gotCT, "application/x-www-form-urlencoded") {
		t.Fatalf("Content-Type %q", gotCT)
	}
	if gotTask != "5" || gotFolder != "-1" {
		t.Fatalf("表单解析不对: task=%q folder_id=%q", gotTask, gotFolder)
	}
}

// TestRetryOnTransportError 传输层错误按 Retries 重试：第一次断连，第二次成功。
func TestRetryOnTransportError(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			panic(http.ErrAbortHandler) // 模拟连接中断（传输层错误）
		}
		io.WriteString(w, `ok`)
	}))
	defer srv.Close()

	resp, err := New(Config{Retries: 1}).Do(context.Background(), &Request{URL: srv.URL})
	if err != nil {
		t.Fatalf("重试后应成功: %v", err)
	}
	if string(resp.Body) != "ok" {
		t.Fatalf("响应体 %q", resp.Body)
	}
	if attempts != 2 {
		t.Fatalf("尝试次数 %d，期望 2", attempts)
	}
}

// TestNoRetryExhausted 重试耗尽后返回最后一次错误。
func TestNoRetryExhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	defer srv.Close()

	_, err := New(Config{Retries: 2}).Do(context.Background(), &Request{URL: srv.URL})
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

// TestLogf 配置日志时按次记录。
func TestLogf(t *testing.T) {
	var logs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `ok`)
	}))
	defer srv.Close()

	e := New(Config{Logf: func(format string, args ...any) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}})
	if _, err := e.Do(context.Background(), &Request{URL: srv.URL + "/ping"}); err != nil {
		t.Fatalf("Do 出错: %v", err)
	}
	if len(logs) != 1 || !strings.Contains(logs[0], "/ping") {
		t.Fatalf("日志不对: %v", logs)
	}
}

// TestCustomUserAgent 自定义 UA 覆盖默认值。
func TestCustomUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
	}))
	defer srv.Close()

	_, _ = New(Config{UserAgent: "my-ua/1.0"}).Do(context.Background(), &Request{URL: srv.URL})
	if gotUA != "my-ua/1.0" {
		t.Fatalf("UA %q", gotUA)
	}
}

func urlValues(m map[string]string) url.Values {
	v := url.Values{}
	for k, val := range m {
		v.Set(k, val)
	}
	return v
}
