package user

import "testing"

// TestIsLoginRedirect 验证登录页重定向判定（CheckLogin 的核心逻辑）。
//
// 修复背景：百度新版网盘把 /disk/home 302 跳到 /disk/main（站内改版跳转），
// 早期 CheckLogin 把所有 3xx 当失效，导致有效 BDUSS 被误判。
// 正确判定：只有重定向到 passport.baidu.com 或登录鉴权路径才算 BDUSS 失效。
func TestIsLoginRedirect(t *testing.T) {
	cases := []struct {
		name string
		loc  string
		want bool
	}{
		// 真·失效：重定向到 passport 登录页
		{"passport绝对地址", "https://passport.baidu.com/v3/login/api/auth/?return_type=5&tpl=netdisk", true},
		{"passporthttps", "https://passport.baidu.com/v3/login/api/auth/", true},

		// 有效：站内改版跳转（修复重点，之前会被误判）
		{"disk_main改版跳转", "/disk/main?from=homeFlow", false},
		{"disk_home站内", "/disk/home", false},
		{"disk_main绝对地址", "https://pan.baidu.com/disk/main", false},
		{"disk_home绝对地址", "https://pan.baidu.com/disk/home", false},

		// 边界
		{"空Location", "", false},
		{"相对登录路径", "/v3/login/api/auth/?tpl=netdisk", true},
		{"含login关键词", "/login?redirect=x", true},
		{"无关路径", "/disk/clouddl", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isLoginRedirect(c.loc); got != c.want {
				t.Errorf("isLoginRedirect(%q) = %v, want %v", c.loc, got, c.want)
			}
		})
	}
}
