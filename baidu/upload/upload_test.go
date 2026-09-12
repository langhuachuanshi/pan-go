package upload

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// fakeInvoker 模拟百度接口，按 path 返回预设响应，用于测试 Upload 的进度回调逻辑。
type fakeInvoker struct {
	partSize int64 // 每片大小（用于断言）
}

func (f *fakeInvoker) Get(ctx context.Context, path string, query map[string]string, out any) error {
	return nil
}

// PostForm core 菜单表单（上传流程实际走 PostFormQuery）
func (f *fakeInvoker) Post(ctx context.Context, path string, body any, query map[string]string, out any) error {
	return nil
}

func (f *fakeInvoker) PostForm(ctx context.Context, path string, form map[string]string, out any) error {
	return nil
}

// PostFormQuery 处理 precreate / create（body 与 query 分离）
func (f *fakeInvoker) PostFormQuery(ctx context.Context, path string, body, params map[string]string, out any) error {
	switch path {
	case "/api/precreate":
		// 返回 return_type=1（需要上传分片），block_list=[0,1,2]（全部分片待传）
		resp, _ := json.Marshal(map[string]any{
			"errno":       0,
			"uploadid":    "test-upload-id",
			"return_type": 1,
			"block_list":  []int{0, 1, 2},
		})
		return json.Unmarshal(resp, out)
	case "/api/create":
		resp, _ := json.Marshal(map[string]any{
			"errno": 0,
			"data":  map[string]any{"fs_id": 123, "path": "/test/big.bin"},
		})
		return json.Unmarshal(resp, out)
	}
	return nil
}

func (f *fakeInvoker) PostMultipart(ctx context.Context, baseURL, path string, params map[string]string, fieldName, fileName string, data []byte) ([]byte, int, error) {
	// superfile2 分片上传：返回 {errno:0, md5:"fakepartmd5"}
	resp, _ := json.Marshal(map[string]any{"errno": 0, "md5": "fakepartmd5"})
	return resp, 200, nil
}

func (f *fakeInvoker) GetRaw(ctx context.Context, fullURL string) ([]byte, int, error) {
	return nil, 0, nil
}

// PostFormRaw 处理 superfile2 分片上传（PCS 接口走 Raw）
func (f *fakeInvoker) PostFormRaw(ctx context.Context, fullURL string, body map[string]string) ([]byte, int, error) {
	// superfile2 返回 {errno:0, md5:"abc"}
	resp, _ := json.Marshal(map[string]any{"errno": 0, "md5": "fakepartmd5"})
	return resp, 200, nil
}

func (f *fakeInvoker) PostMultipartForm(ctx context.Context, fullURL string, fields map[string]string) ([]byte, int, error) {
	return nil, 0, nil
}

func (f *fakeInvoker) Multipart(ctx context.Context, path string, form map[string]string, field, filename string, file io.Reader, out any) error {
	return nil
}

func (f *fakeInvoker) DownloadHeaders() map[string]string { return nil }

func (f *fakeInvoker) HTTPClient() *http.Client { return http.DefaultClient }

// TestUploadProgressCallback 验证 OnProgress 回调被正确触发：
// - 次数 = 分片数
// - uploaded 单调递增
// - 最后一次 total == size
func TestUploadProgressCallback(t *testing.T) {
	// 构造 10MB 数据（partSize=4MB → 3 片：4M, 4M, 2M）
	size := int64(10 * 1024 * 1024)
	data := make([]byte, size)
	req := &UploadRequest{
		ReaderAt: bytesReaderAt(data),
		FileName: "big.bin",
		Size:     size,
		DestPath: "/test",
	}

	var calls []struct{ uploaded, total int64 }
	req.OnProgress = func(uploaded, total int64) {
		calls = append(calls, struct{ uploaded, total int64 }{uploaded, total})
	}

	s := New(&fakeInvoker{partSize: partSize})
	if _, err := s.Upload(context.Background(), req); err != nil {
		t.Fatalf("Upload 失败: %v", err)
	}

	// 3 片 → 3 次回调
	if len(calls) != 3 {
		t.Fatalf("期望 3 次进度回调，实际 %d 次", len(calls))
	}

	// total 应恒等于 size
	for i, c := range calls {
		if c.total != size {
			t.Errorf("第 %d 次 total=%d, 期望 %d", i+1, c.total, size)
		}
	}

	// uploaded 应单调递增：4M, 8M, 10M
	expect := []int64{4 * 1024 * 1024, 8 * 1024 * 1024, 10 * 1024 * 1024}
	for i, e := range expect {
		if calls[i].uploaded != e {
			t.Errorf("第 %d 次 uploaded=%d, 期望 %d", i+1, calls[i].uploaded, e)
		}
	}

	// 最后一次应为 100%
	if calls[len(calls)-1].uploaded != size {
		t.Errorf("最后一次 uploaded=%d, 应等于 size %d", calls[len(calls)-1].uploaded, size)
	}
}

// TestUploadNoProgressCallback 不传 OnProgress 时应正常工作（向后兼容）
func TestUploadNoProgressCallback(t *testing.T) {
	size := int64(8 * 1024 * 1024) // 2 片
	data := make([]byte, size)
	req := &UploadRequest{
		ReaderAt: bytesReaderAt(data),
		FileName: "noprog.bin",
		Size:     size,
		DestPath: "/test",
		// OnProgress 不设
	}
	s := New(&fakeInvoker{partSize: partSize})
	if _, err := s.Upload(context.Background(), req); err != nil {
		t.Fatalf("不传 OnProgress 时 Upload 失败: %v", err)
	}
}

// bytesReaderAt 测试用 ReaderAt
type bytesReaderAt []byte

func (b bytesReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(b)) {
		return 0, io.EOF
	}
	return copy(p, b[off:]), nil
}
