package lanzou

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Login 登录蓝奏云帐号。
// 蓝奏已把账号系统从 pc.woozooo.com/account/loginajax 迁到 accounts.woozooo.com：
// 先过 acw_sc__v2 JS 反爬挑战拿会话 cookie，再 POST /accounts.php（task=uselogin）。
func (c *Client) Login(user, pwd string) error {
	// Step 1: 请求登录页并自动过 acw_sc__v2 挑战（复用直链解析同款挑战处理）
	loginPageURL := baseURLAccount + "/accounts.php?action=login&ref=pc.woozooo.com"
	if _, err := c.fetchPageWithChallenge(loginPageURL); err != nil {
		return fmt.Errorf("login page request failed: %w", err)
	}

	// Step 2: POST 登录（字段与新版登录页 uselogin() 一致）
	data := map[string]string{
		"task":     "uselogin",
		"username": user,
		"password": pwd,
		"ref":      "pc.woozooo.com",
	}
	body, _, err := c.post(baseURLAccount+pathAccountLogin, data, map[string]string{
		"Referer":          loginPageURL,
		"X-Requested-With": "XMLHttpRequest",
	})
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}

	// zt 前端用宽松比较（== '1'），可能为数字或字符串，用 interface{} 兼容两种
	var resp struct {
		Zt   interface{} `json:"zt"`
		Msgs string      `json:"msgs"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("%w: invalid login response: %s", ErrAPIError, string(body))
	}

	if !ztIsOne(resp.Zt) {
		if strings.Contains(resp.Msgs, "密码") {
			return ErrPasswordWrong
		}
		return fmt.Errorf("%w: login failed: %s", ErrAPIError, resp.Msgs)
	}

	// Step 3: 成功时 msgs 是中转鉴权跳转 URL，跟随它把最终登录态 cookie 落到 pc.woozooo.com 域
	if strings.HasPrefix(resp.Msgs, "http") {
		if err := c.followLoginRedirect(resp.Msgs); err != nil {
			return fmt.Errorf("login redirect failed: %w", err)
		}
	}

	c.logged = true
	c.initUID()
	return nil
}

// ztIsOne 判断登录响应 zt 是否表示成功（兼容数字 1 与字符串 "1"）。
func ztIsOne(v interface{}) bool {
	switch t := v.(type) {
	case float64:
		return t == 1
	case string:
		return t == "1"
	case json.Number:
		return t.String() == "1"
	}
	return false
}

// followLoginRedirect 跟随中转鉴权跳转链（最多 5 跳），用于把登录态 cookie 落在最终域。
func (c *Client) followLoginRedirect(next string) error {
	for i := 0; i < 5 && strings.HasPrefix(next, "http"); i++ {
		_, hdr, err := c.get(next, nil)
		if err != nil {
			return err
		}
		loc := hdr.Get("Location")
		if loc == "" {
			return nil
		}
		if !strings.HasPrefix(loc, "http") {
			u, err := url.Parse(next)
			if err != nil {
				return err
			}
			u = u.ResolveReference(&url.URL{Path: loc})
			loc = u.String()
		}
		next = loc
	}
	return nil
}

// Logout 登出
func (c *Client) Logout() error {
	_, _, err := c.get(baseURLPC+pathLogout, nil)
	if err != nil {
		return fmt.Errorf("logout request failed: %w", err)
	}
	c.logged = false
	c.cookies = nil
	return nil
}

// GetUserInfo 获取用户信息
func (c *Client) GetUserInfo() (*UserInfo, error) {
	if !c.isLoggedIn() {
		return nil, ErrNotLoggedIn
	}
	c.initUID()
	return &UserInfo{
		UserName: c.uid,
	}, nil
}

// GetAccountInfo 获取帐号详细信息
func (c *Client) GetAccountInfo() (*AccountInfo, error) {
	if !c.isLoggedIn() {
		return nil, ErrNotLoggedIn
	}
	c.initUID()

	// 通过个人中心页面获取帐号信息
	body, _, err := c.get(baseURLPC+"/mydisk.php?item=profile&action=mypower", nil)
	if err != nil {
		return nil, fmt.Errorf("get account info failed: %w", err)
	}
	html := string(body)

	// 从页面提取用户名
	reName := regexp.MustCompile(`(\d{11,})`)
	info := &AccountInfo{
		UserInfo: UserInfo{
			UserName: c.uid,
		},
	}
	if m := reName.FindStringSubmatch(html); len(m) > 1 {
		info.UserName = m[1]
	}

	// 提取容量信息
	reSize := regexp.MustCompile(`(\d+\.?\d*)\s*(GB|MB|KB|TB)`)
	matches := reSize.FindAllStringSubmatch(html, 2)
	if len(matches) >= 2 {
		info.TotalSize = matches[0][0]
		info.UsedSize = matches[1][0]
	} else if len(matches) >= 1 {
		info.TotalSize = matches[0][0]
	}

	return info, nil
}
