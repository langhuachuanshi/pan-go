package resolve

// requestDownloadURL 延迟对冲的行为测试：mock Invoker（嵌接口，只覆盖解析链路用的两个方法），
// 验证平静网络零额外请求、慢端点对冲、终局错误即返、快速失败 kick 补发、外层 ctx 可取消。

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/langhuachuanshi/pan-go/lanzou/invoker"
)

// hedgeInvoker mock：按候选端点顺序编排行为（延时 + 结果），并记录实际调用。
type hedgeInvoker struct {
	invoker.Invoker // 嵌接口：未覆盖的方法调用即 panic，测试只走得到下面两个

	mu       sync.Mutex
	calls    []string // 每次实际 POST 的端点
	behavior map[string]hedgeBehavior
}

type hedgeBehavior struct {
	delay  time.Duration
	netErr bool   // true=传输层错误（不返回 JSON）；否则按 zt/mess 正常响应
	zt     int    // 1=成功；2=文件失效；0=其它业务失败
	mess   string // 含「密码」时按密码错处理
}

func newHedgeInvoker(behavior map[string]hedgeBehavior) *hedgeInvoker {
	return &hedgeInvoker{behavior: behavior}
}

func (m *hedgeInvoker) FetchPageWithChallengeCtx(ctx context.Context, pageURL string) (string, error) {
	return "<html></html>", nil
}

func (m *hedgeInvoker) PostFormHeaders(ctx context.Context, api string, form map[string]string,
	headers map[string]string, out any) error {
	m.mu.Lock()
	m.calls = append(m.calls, api)
	b := m.behavior[api]
	m.mu.Unlock()

	select {
	case <-time.After(b.delay):
	case <-ctx.Done():
		return ctx.Err()
	}
	if b.netErr {
		return errors.New("network error")
	}
	// 直接写 out（*ajaxmResp）——同包测试可用内部类型
	resp := out.(*ajaxmResp)
	resp.Zt, resp.Mess = b.zt, b.mess
	if b.zt == 1 {
		resp.Dom, resp.Url = "https://cdn.example.com", "?ABC"
	}
	return nil
}

func (m *hedgeInvoker) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func hedgeIframe(first, second string) *iframeData {
	return &iframeData{
		WpSign:   "sign",
		AjaxURLs: []string{first, second},
		Kdns:     1,
	}
}

func TestHedgeFastFirstNoExtraRequest(t *testing.T) {
	// 平静网络：首端点 50ms 成功 → 不应补发第二个请求
	inv := newHedgeInvoker(map[string]hedgeBehavior{
		"https://a.example": {delay: 50 * time.Millisecond, zt: 1},
		"https://b.example": {delay: 10 * time.Millisecond, zt: 1},
	})
	s := New(inv)
	durl, err := s.requestDownloadURL(context.Background(), "https://share.example",
		hedgeIframe("https://a.example", "https://b.example"), 1, "")
	if err != nil || durl == "" {
		t.Fatalf("want success, got durl=%q err=%v", durl, err)
	}
	if inv.callCount() != 1 {
		t.Fatalf("平静网络应只发 1 个请求，实际 %d 次", inv.callCount())
	}
}

func TestHedgeSlowFirstSecondWins(t *testing.T) {
	// 首端点慢（2s）→ 400ms 后对冲补发，次端点 50ms 成功 → 总耗时应远小于串行的 2s
	inv := newHedgeInvoker(map[string]hedgeBehavior{
		"https://a.example": {delay: 2 * time.Second, zt: 1},
		"https://b.example": {delay: 50 * time.Millisecond, zt: 1},
	})
	s := New(inv)
	start := time.Now()
	durl, err := s.requestDownloadURL(context.Background(), "https://share.example",
		hedgeIframe("https://a.example", "https://b.example"), 1, "")
	elapsed := time.Since(start)
	if err != nil || durl == "" {
		t.Fatalf("want success, got durl=%q err=%v", durl, err)
	}
	if elapsed > time.Second {
		t.Fatalf("对冲应 ~450ms 返回，实际 %v", elapsed)
	}
}

func TestHedgeTerminalErrorImmediate(t *testing.T) {
	// 首端点返回密码错（终局）→ 立即返回 ErrPasswordWrong，不补发、不等待
	inv := newHedgeInvoker(map[string]hedgeBehavior{
		"https://a.example": {delay: 30 * time.Millisecond, zt: 0, mess: "密码错误"},
		"https://b.example": {delay: 30 * time.Millisecond, zt: 1},
	})
	s := New(inv)
	_, err := s.requestDownloadURL(context.Background(), "https://share.example",
		hedgeIframe("https://a.example", "https://b.example"), 1, "")
	if !errors.Is(err, invoker.ErrPasswordWrong) {
		t.Fatalf("want ErrPasswordWrong, got %v", err)
	}
	if inv.callCount() != 1 {
		t.Fatalf("终局错误不应补发，实际调用 %d 次", inv.callCount())
	}
}

func TestHedgeFastFailKicksBackup(t *testing.T) {
	// 首端点 30ms 网络错误 → kick 立即补发（不等 400ms），次端点成功
	inv := newHedgeInvoker(map[string]hedgeBehavior{
		"https://a.example": {delay: 30 * time.Millisecond, netErr: true},
		"https://b.example": {delay: 30 * time.Millisecond, zt: 1},
	})
	s := New(inv)
	start := time.Now()
	durl, err := s.requestDownloadURL(context.Background(), "https://share.example",
		hedgeIframe("https://a.example", "https://b.example"), 1, "")
	elapsed := time.Since(start)
	if err != nil || durl == "" {
		t.Fatalf("want success, got durl=%q err=%v", durl, err)
	}
	if elapsed > 300*time.Millisecond {
		t.Fatalf("kick 应快速补发（~60ms），实际 %v", elapsed)
	}
}

func TestHedgeAllFailReturnsError(t *testing.T) {
	inv := newHedgeInvoker(map[string]hedgeBehavior{
		"https://a.example": {delay: 20 * time.Millisecond, netErr: true},
		"https://b.example": {delay: 20 * time.Millisecond, netErr: true},
	})
	s := New(inv)
	_, err := s.requestDownloadURL(context.Background(), "https://share.example",
		hedgeIframe("https://a.example", "https://b.example"), 1, "")
	if err == nil {
		t.Fatal("全败应返回错误")
	}
}

func TestHedgeOuterCtxCancels(t *testing.T) {
	// 外层 ctx 300ms 超时 + 首端点 hang 2s → 必须在 ~300ms 返回（ctx 贯通的回归测试）
	inv := newHedgeInvoker(map[string]hedgeBehavior{
		"https://a.example": {delay: 2 * time.Second, zt: 1},
		"https://b.example": {delay: 2 * time.Second, zt: 1},
	})
	s := New(inv)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := s.requestDownloadURL(ctx, "https://share.example",
		hedgeIframe("https://a.example", "https://b.example"), 1, "")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("ctx 超时应返回错误")
	}
	if elapsed > 600*time.Millisecond {
		t.Fatalf("ctx 应 ~300ms 生效，实际 %v（ctx 未贯通？）", elapsed)
	}
}
