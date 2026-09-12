package errors

import (
	"errors"
	"fmt"
	"testing"
)

// TestNewAndError 基本构造与 Error 文案。
func TestNewAndError(t *testing.T) {
	e := New("quark", 31003, 200, "cookie 失效", KindAuth)
	want := "quark: code=31003 status=200 kind=auth: cookie 失效"
	if e.Error() != want {
		t.Errorf("Error() = %q, 期望 %q", e.Error(), want)
	}
	if e.Kind() != KindAuth {
		t.Errorf("Kind = %v", e.Kind())
	}
}

// TestSemanticJudgment 语义判定：errors.As 穿透 wrap，Kind 对号入座。
func TestSemanticJudgment(t *testing.T) {
	wrapped := fmt.Errorf("列目录失败: %w", New("baidu", -6, 200, "login", KindAuth))

	if !IsAuth(wrapped) {
		t.Error("IsAuth 应为 true")
	}
	if IsRateLimited(wrapped) || IsNotFound(wrapped) || IsDenied(wrapped) {
		t.Error("其他语义不应命中")
	}
	if IsAuth(New("quark", 41013, 200, "太频繁", KindRateLimited)) {
		t.Error("限频不应命中 IsAuth")
	}
	if IsRateLimited(New("quark", 41013, 200, "太频繁", KindRateLimited)) != true {
		t.Error("IsRateLimited 应为 true")
	}
}

// TestNonAPIError 非统一错误一律返回 false，不 panic。
func TestNonAPIError(t *testing.T) {
	if IsAuth(errors.New("随便一个错误")) {
		t.Error("普通错误不应命中任何语义")
	}
	if IsAuth(nil) {
		t.Error("nil 不应命中")
	}
}

// TestKindString Kind 文案。
func TestKindString(t *testing.T) {
	cases := map[Kind]string{
		KindOther: "other", KindAuth: "auth", KindRateLimited: "rate_limited",
		KindNotFound: "not_found", KindDenied: "denied",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, 期望 %q", k, got, want)
		}
	}
}
