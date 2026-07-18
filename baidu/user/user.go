// Package user 提供百度网盘用户信息和空间配额查询（网页端 / BDUSS 方案）。
//
// 接口：
//   - Quota：GET pan.baidu.com/api/quota（需 BDUSS）
//   - UserInfo：从 /disk/home 页面提取用户信息
//
// 参考：BaiduPCS-Go + 官方 baidu-drive-sdk-go。
package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/langhuachuanshi/baidupan-go/baidu/invoker"
)

// Service 用户信息入口。
type Service struct {
	inv invoker.Invoker
}

// New 创建 user Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// QuotaInfo 空间配额信息。
type QuotaInfo struct {
	Total  int64 // 总空间（字节）
	Used   int64 // 已用空间（字节）
	Free   int64 // 剩余空间（字节）
	Expire bool  // 是否有即将过期的文件
}

// UserInfo 用户基本信息。
type UserInfo struct {
	UK         int64  // 用户 UK
	BaiduName  string // 百度用户名
	AvatarURL  string // 头像 URL
	VipType    int    // VIP 类型（0=普通, 1=VIP, 2=SVIP）
}

// Quota 获取空间配额信息。
// 接口：GET https://pan.baidu.com/api/quota?checkfree=1&checkexpire=1
func (s *Service) Quota(ctx context.Context) (*QuotaInfo, error) {
	data, _, err := s.inv.Get(ctx, "/api/quota", map[string]string{
		"checkfree":   "1",
		"checkexpire": "1",
	})
	if err != nil {
		return nil, fmt.Errorf("获取配额失败: %w", err)
	}

	var resp struct {
		Errno  int   `json:"errno"`
		Total  int64 `json:"total"`
		Used   int64 `json:"used"`
		Free   int64 `json:"free"`
		Expire bool  `json:"expire"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("解析配额响应失败: %w", err)
	}
	if resp.Errno != 0 {
		return nil, invoker.NewAPIError(resp.Errno, "获取配额失败")
	}
	return &QuotaInfo{
		Total:  resp.Total,
		Used:   resp.Used,
		Free:   resp.Total - resp.Used,
		Expire: resp.Expire,
	}, nil
}

// userInfoRE 从 /disk/home HTML 提取用户信息。
var userInfoRE = regexp.MustCompile(`"username":"(.*?)"[\s\S]*"avatar_url":"(.*?)"[\s\S]*"uk":(\d+)[\s\S]*"vip_type":(\d+)`)

// User 从 /disk/home 页面提取用户基本信息。
// httpClient 应携带 BDUSS cookie。
func (s *Service) User(ctx context.Context, httpClient *http.Client) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://pan.baidu.com/disk/home", nil)
	if err != nil {
		return nil, fmt.Errorf("user: 创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("user: 请求 /disk/home 失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查重定向（BDUSS 失效）
	loc := resp.Header.Get("Location")
	if loc == "/" || (len(loc) > 0 && loc[:8] == "https://") {
		return nil, fmt.Errorf("user: BDUSS 已失效（重定向到 %s）", loc)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("user: 读取响应失败: %w", err)
	}

	match := userInfoRE.FindSubmatch(body)
	if len(match) <= 4 {
		return nil, fmt.Errorf("user: 未能从 /disk/home 提取用户信息")
	}

	uk := int64(0)
	fmt.Sscanf(string(match[3]), "%d", &uk)
	vipType := 0
	fmt.Sscanf(string(match[4]), "%d", &vipType)

	return &UserInfo{
		BaiduName: string(match[1]),
		AvatarURL: string(match[2]),
		UK:        uk,
		VipType:   vipType,
	}, nil
}

// ErrLoginExpired 表示 BDUSS 已失效（被百度重定向到登录页）。
// 调用方用 errors.Is(err, ErrLoginExpired) 判定，不要靠字符串匹配。
var ErrLoginExpired = errors.New("user: 登录已失效（BDUSS 已过期）")

// CheckLogin 探活：检查 BDUSS 是否仍有效。
//
// 策略（精准识别登录页重定向，不受站内改版跳转影响）：
//   - 克隆传入的 httpClient 并临时禁用重定向（CheckRedirect 返回 ErrUseLastResponse），
//     不修改调用方原始 client（若其已自定义 CheckRedirect 也不会被破坏）
//   - 请求 https://pan.baidu.com/disk/home：
//   - 2xx：BDUSS 有效，返回 (true, nil)
//   - 3xx 且 Location 指向 passport.baidu.com（登录鉴权接口）：BDUSS 失效，
//     返回 (false, fmt.Errorf("%w...", ErrLoginExpired, ...))
//   - 3xx 但 Location 是站内路径（如 /disk/main?from=homeFlow，新版网盘改版跳转）：
//     BDUSS 仍有效，返回 (true, nil)
//   - 其他状态码：网络/服务异常，返回 (false, err)
//
// 背景：百度新版网盘把 /disk/home 302 跳到 /disk/main（站内正常跳转），
// 早期实现把所有 3xx 当失效，导致有效 BDUSS 被误判。
//
// httpClient 应携带待检测的 BDUSS cookie（通常传 Client.HTTPClient()）。
func (s *Service) CheckLogin(ctx context.Context, httpClient *http.Client) (bool, error) {
	// 浅拷贝 client struct，仅覆盖 CheckRedirect；Transport/Jar 等底层共享，安全。
	noRedirect := *httpClient
	noRedirect.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://pan.baidu.com/disk/home", nil)
	if err != nil {
		return false, fmt.Errorf("user: 创建探活请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := noRedirect.Do(req)
	if err != nil {
		return false, fmt.Errorf("user: 探活请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 2xx：有效
	if resp.StatusCode < 300 {
		return true, nil
	}

	// 3xx：区分登录页重定向（失效）vs 站内跳转（有效）
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		loc := resp.Header.Get("Location")
		if isLoginRedirect(loc) {
			return false, fmt.Errorf("%w（重定向到 %s）", ErrLoginExpired, loc)
		}
		// 站内跳转（如 /disk/main）：BDUSS 有效
		return true, nil
	}

	return false, fmt.Errorf("user: 探活失败 HTTP %d", resp.StatusCode)
}

// isLoginRedirect 判断重定向目标是否是百度登录鉴权页（BDUSS 失效的标志）。
// 失效时百度跳到 passport.baidu.com 或 /v3/login/api/auth；站内改版跳转（如
// /disk/main、/disk/home）不算失效。
func isLoginRedirect(loc string) bool {
	if loc == "" {
		return false
	}
	// 绝对地址：passport.baidu.com
	if strings.HasPrefix(loc, "https://") || strings.HasPrefix(loc, "http://") {
		return strings.Contains(loc, "://passport.baidu.com") ||
			strings.Contains(loc, "passport.baidu.com")
	}
	// 相对地址：登录鉴权接口路径
	return strings.Contains(loc, "/login") || strings.Contains(loc, "/v3/login/api/auth")
}
