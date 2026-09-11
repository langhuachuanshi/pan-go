package lanzou

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestZtIsOne 登录响应 zt 兼容数字/字符串两种形态（前端用 == 宽松比较）。
func TestZtIsOne(t *testing.T) {
	cases := []struct {
		in   interface{}
		want bool
	}{
		{float64(1), true},
		{"1", true},
		{float64(0), false},
		{"0", false},
		{float64(2), false},
		{"", false},
	}
	for _, c := range cases {
		if got := ztIsOne(c.in); got != c.want {
			t.Errorf("ztIsOne(%v) = %v, 期望 %v", c.in, got, c.want)
		}
	}
}

// TestFollowLoginRedirect 验证中转鉴权跳转链被正确跟随并吸收 Set-Cookie。
func TestFollowLoginRedirect(t *testing.T) {
	// 中转链：/sso → 302 /callback（带 Set-Cookie）→ 200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sso":
			http.Redirect(w, r, "/callback", http.StatusFound)
		case "/callback":
			http.SetCookie(w, &http.Cookie{Name: "ylogin", Value: "13800000000"})
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := NewClient(WithHTTPClient(srv.Client()))
	if err := c.followLoginRedirect(srv.URL + "/sso"); err != nil {
		t.Fatalf("followLoginRedirect 出错: %v", err)
	}
	if c.getCookieValue("ylogin") != "13800000000" {
		t.Errorf("未吸收中转链 Set-Cookie，cookies=%v", c.cookies)
	}
}
