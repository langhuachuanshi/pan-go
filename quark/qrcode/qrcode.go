// Package qrcode 实现夸克扫码登录：生成二维码 → 用户夸克App扫码 →
// 轮询 service_ticket → 换发登录 cookie（含 httpOnly 的 __pus，F12 复制时代
// 一直拿不到的就是它）。
//
// 端点与流程 2026-09-12 真机验证（与 QuarkPan 等开源实现一致）：
//  1. GET  uop.quark.cn/cas/ajax/getTokenForQrcodeLogin?client_id=532&v=1.2&request_id=<uuid>
//     → data.members.token（二维码 token，有效期约 2 分钟）
//  2. 二维码内容 = su.quark.cn 短链（调用方自行渲染成二维码图片）
//  3. 轮询 GET uop.quark.cn/cas/ajax/getServiceTicketByQrcodeToken?...&token=...
//     → status=2000000 且 data.members.service_ticket 即已确认
//  4. GET  pan.quark.cn/account/info?st=<ticket>&lw=scan → Set-Cookie 落地完整会话
//
// 换发的 cookie 只有 __pus 不带 __puus（网页登录则两者都有），drive 接口实测认。
package qrcode

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

const (
	apiBase  = "https://uop.quark.cn/cas/ajax"
	clientID = "532"
	apiVer   = "1.2"

	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36"
)

// Login 一次扫码登录会话。
type Login struct {
	Token     string // 二维码 token（轮询用）
	QRURL     string // 二维码内容短链（调用方渲染成二维码图片给用户扫）
	RequestID string // 创建时的 request_id，轮询要带

	ticket    string // 扫码确认后的 service_ticket（poll 内部写入）
	createdAt time.Time
	client    *http.Client // 带 cookiejar，第4步换 cookie 用
}

// Create 发起一次扫码登录，返回二维码会话。
func Create(ctx context.Context) (*Login, error) {
	requestID, err := newUUID()
	if err != nil {
		return nil, err
	}
	body, err := httpGet(ctx, apiBase+"/getTokenForQrcodeLogin",
		url.Values{"client_id": {clientID}, "v": {apiVer}, "request_id": {requestID}})
	if err != nil {
		return nil, fmt.Errorf("获取二维码 token 失败: %w", err)
	}
	var resp struct {
		Status int    `json:"status"`
		Msg    string `json:"message"`
		Data   struct {
			Members struct {
				Token string `json:"token"`
			} `json:"members"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析二维码 token 响应失败: %w", err)
	}
	if resp.Data.Members.Token == "" {
		return nil, fmt.Errorf("获取二维码 token 失败: status=%d %s", resp.Status, resp.Msg)
	}

	qrURL := "https://su.quark.cn/4_eMHBJ?" + url.Values{
		"token":        {resp.Data.Members.Token},
		"client_id":    {clientID},
		"ssb":          {"weblogin"},
		"uc_param_str": {""},
		"uc_biz_str":   {"S:custom|OPT:SAREA@0|OPT:IMMERSIVE@1|OPT:BACK_BTN_STYLE@0"},
	}.Encode()

	jar, _ := cookiejar.New(nil)
	return &Login{
		Token:     resp.Data.Members.Token,
		QRURL:     qrURL,
		RequestID: requestID,
		createdAt: time.Now(),
		client:    &http.Client{Jar: jar, Timeout: 30 * time.Second},
	}, nil
}

// Wait 等待用户扫码并换发登录 cookie（阻塞轮询直到成功/超时/二维码过期）。
//
// interval 为轮询间隔（默认 2s），timeout 为总等待上限。成功返回完整 cookie 字符串
// （可直接 quark.New(WithCookie(...)) 或 auth.SaveCookie 落盘）。
func (l *Login) Wait(ctx context.Context, timeout time.Duration, interval ...time.Duration) (string, error) {
	gap := 2 * time.Second
	if len(interval) > 0 && interval[0] > 0 {
		gap = interval[0]
	}
	deadline := time.Now().Add(timeout)
	for {
		switch err := l.poll(ctx); {
		case err == nil:
			return l.exchange(ctx)
		case isWaiting(err):
			// 还没扫码，继续等
		default:
			return "", err
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("扫码登录超时（%.0f 秒）", timeout.Seconds())
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(gap):
		}
	}
}

// errWaiting 轮询"还在等待扫码"的内部标记。
type errWaiting struct{ msg string }

func (e *errWaiting) Error() string { return e.msg }

func isWaiting(err error) bool {
	_, ok := err.(*errWaiting)
	return ok
}

// poll 查一次扫码状态：已确认返回 nil，等待中返回 errWaiting，其余为真实错误
// （含 50004002 二维码过期——有效期约 2 分钟，过期需重新 Create）。
func (l *Login) poll(ctx context.Context) error {
	body, err := httpGet(ctx, apiBase+"/getServiceTicketByQrcodeToken",
		url.Values{"client_id": {clientID}, "v": {apiVer}, "token": {l.Token}, "request_id": {l.RequestID}})
	if err != nil {
		return fmt.Errorf("查询扫码状态失败: %w", err)
	}
	var resp struct {
		Status int    `json:"status"`
		Msg    string `json:"message"`
		Data   struct {
			Members struct {
				ServiceTicket string `json:"service_ticket"`
			} `json:"members"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("解析扫码状态响应失败: %w", err)
	}
	if resp.Status == 2000000 && resp.Msg == "ok" && resp.Data.Members.ServiceTicket != "" {
		l.ticket = resp.Data.Members.ServiceTicket
		return nil
	}
	if resp.Status == 50004002 || strings.Contains(resp.Msg, "Not Found") {
		return fmt.Errorf("二维码已过期（有效期约 2 分钟），请重新 Create: status=%d %s", resp.Status, resp.Msg)
	}
	// 50004001 = 等待扫码中
	return &errWaiting{msg: fmt.Sprintf("等待扫码: status=%d %s", resp.Status, resp.Msg)}
}

// exchange 用 service_ticket 换发登录 cookie（带重定向跟随，Set-Cookie 自动入 jar）。
func (l *Login) exchange(ctx context.Context) (string, error) {
	u := "https://pan.quark.cn/account/info?st=" + url.QueryEscape(l.ticket) + "&lw=scan"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://pan.quark.cn/")
	resp, err := l.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("换取登录 cookie 失败: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	// 从 jar 收集 .quark.cn / pan.quark.cn 域的 cookie
	var parts []string
	for _, domain := range []string{"https://pan.quark.cn/", "https://quark.cn/"} {
		uo, _ := url.Parse(domain)
		for _, ck := range l.client.Jar.Cookies(uo) {
			if !strings.Contains(strings.Join(parts, "; "), ck.Name+"=") {
				parts = append(parts, ck.Name+"="+ck.Value)
			}
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("换取登录 cookie 失败：响应未携带 Set-Cookie（http=%d）", resp.StatusCode)
	}
	return strings.Join(parts, "; "), nil
}

// ---------- 内部 ----------

func httpGet(ctx context.Context, base string, params url.Values) ([]byte, error) {
	u := base + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", "https://pan.quark.cn/")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(body[:min(len(body), 120)]))
	}
	return body, nil
}

// newUUID crypto/rand 生成 v4 UUID。
func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:], nil
}
