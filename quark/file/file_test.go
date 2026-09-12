package file

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/langhuachuanshi/pan-go/quark/invoker"
)

// fakeInvoker 以 canned 页响应 List 请求，记录每页的 _page 参数。
type fakeInvoker struct {
	pages    []string // 第 n 次调用返回的 JSON
	gotPages []string // 收到的 _page 值序列
}

func (f *fakeInvoker) Get(ctx context.Context, path string, params map[string]string, headers map[string]string) ([]byte, int, error) {
	f.gotPages = append(f.gotPages, params["_page"])
	if len(f.gotPages) > len(f.pages) {
		return nil, 0, errors.New("no more canned pages")
	}
	b := f.pages[len(f.gotPages)-1]
	return []byte(b), 200, nil
}

func (f *fakeInvoker) Post(ctx context.Context, path string, body any, params map[string]string, headers map[string]string) ([]byte, int, error) {
	return nil, 0, errors.New("not used")
}

func (f *fakeInvoker) DownloadHeaders() map[string]string { return nil }

var _ invoker.Invoker = (*fakeInvoker)(nil)

// page 生成 /file/sort 单页响应 JSON；total 按参数原样写入。
func page(total int, start, count int) string {
	items := make([]string, count)
	for i := 0; i < count; i++ {
		n := start + i
		items[i] = fmt.Sprintf(`{"fid":"f%d","file_name":"item-%d"}`, n, n)
	}
	return fmt.Sprintf(`{"code":0,"status":200,"data":{"list":[%s],"total":%d}}`,
		strings.Join(items, ","), total)
}

// TestListTotalZeroTruncation 复现 total=0 截断 bug：夸克部分场景不回传 total，
// 大目录（306 项）必须在短页/空页兜底下翻完整页，而不是停在第一页。
func TestListTotalZeroTruncation(t *testing.T) {
	const total = 306
	const size = 50
	var pages []string
	for start := 0; start < total; start += size {
		n := size
		if start+n > total {
			n = total - start
		}
		pages = append(pages, page(0, start, n))
	}
	pages = append(pages, page(0, total, 0)) // 空页终止

	f := &fakeInvoker{pages: pages}
	got, err := New(f).List(context.Background(), &ListRequest{PDirFID: "0", Size: size})
	if err != nil {
		t.Fatalf("List 出错: %v", err)
	}
	if len(got) != total {
		t.Fatalf("total=0 时截断：取到 %d 项，期望 %d 项", len(got), total)
	}
}

// TestListStopsByTotal total>0 时取够即停，不多发请求。
func TestListStopsByTotal(t *testing.T) {
	f := &fakeInvoker{pages: []string{
		page(80, 0, 50),
		page(80, 50, 30),
		page(80, 80, 0), // 不应被请求到
	}}
	got, err := New(f).List(context.Background(), &ListRequest{PDirFID: "0", Size: 50})
	if err != nil {
		t.Fatalf("List 出错: %v", err)
	}
	if len(got) != 80 {
		t.Fatalf("取到 %d 项，期望 80", len(got))
	}
	if strings.Join(f.gotPages, ",") != "1,2" {
		t.Fatalf("页序 = %v，期望 [1 2]", f.gotPages)
	}
}

// TestListExactMultipleNoHang total=0 且条数恰为 size 整数倍时，靠空页终止且不丢数据。
func TestListExactMultipleNoHang(t *testing.T) {
	f := &fakeInvoker{pages: []string{
		page(0, 0, 50),
		page(0, 50, 50),
		page(0, 100, 0),
	}}
	got, err := New(f).List(context.Background(), &ListRequest{PDirFID: "0", Size: 50})
	if err != nil {
		t.Fatalf("List 出错: %v", err)
	}
	if len(got) != 100 {
		t.Fatalf("取到 %d 项，期望 100", len(got))
	}
}
