// Package qrcode 实现百度网盘扫码登录：官方二维码 → 百度App扫码 → 轮询登录票据 →
// 换发 BDUSS/STOKEN（替代 F12 手动复制 cookie）。
//
// 流程参照 Go 客户端 pan-light 的实测实现（端点为百度 passport 通行证体系，多年稳定）：
//  1. GET  passport.baidu.com/v2/api/getqrcode?lp=pc&apiver=v3 → sign + imgurl（官方二维码图片）
//     二维码内容是 wappass.baidu.com 短链（带 sign、tpl=netdisk）
//  2. 轮询 GET passport.baidu.com/channel/unicast?channel_id=<sign>&tpl=netdisk&apiver=v3
//     → channel_v.status==0 且 v 为登录票据（status==1 已扫未确认，继续等）
//  3. GET  passport.baidu.com/v3/login/main/qrbdusslogin?bduss=<v>&qrcode=1&tpl=netdisk
//     → BDUSS/STOKEN 经 Set-Cookie 落入会话 cookiejar
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
	"regexp"
	"strconv"
	"strings"
	"time"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36"

// reLoginOK qrbdusslogin 成功标志（errInfo.no == "0"，容错空格）。
var reLoginOK = regexp.MustCompile(`"no"\s*:\s*"0"`)

// Login 一次扫码登录会话。
type Login struct {
	Sign   string // 二维码 sign（轮询 channel_id 用）
	QRURL  string // 二维码内容短链（不想用官方图片时自行渲染这个）
	ImgURL string // 官方二维码图片 URL（直接展示即可）

	gid    string
	ticket string
	client *http.Client // 带 cookiejar，第3步换 BDUSS/STOKEN 用
}

// Credentials 扫码换发的登录凭证。
type Credentials struct {
	BDUSS  string
	STOKEN string
	Raw    string // 完整 cookie 字符串（k=v; k=v）
}

// Create 发起一次扫码登录。
//
// 注意 tpl 语义（与 netcccyun/toolbox 现代实现对齐，2026-09-12 实测）：
// 二维码内容短链的 tpl 留空，轮询/换发用 tpl=pp——用错模板（如 tpl=netdisk）
// 会收不到扫码确认事件。
func Create(ctx context.Context) (*Login, error) {
	gid, err := newGID()
	if err != nil {
		return nil, err
	}
	// 关键：getqrcode 和后续轮询必须同一个 cookiejar 会话——getqrcode 下发的
	// BAIDUID 绑定扫码通道，换会话轮询 unicast 会秒回 errno:1（实测 2026-09-12：
	// 同会话时 unicast 正常长轮询 30s）。
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 45 * time.Second}

	q := url.Values{
		"lp":     {"pc"},
		"gid":    {gid},
		"apiver": {"v3"},
		"tt":     {ms()},
	}
	body, err := get(ctx, client, "https://passport.baidu.com/v2/api/getqrcode", q, "")
	if err != nil {
		return nil, fmt.Errorf("获取二维码失败: %w", err)
	}
	var resp struct {
		Errno  int    `json:"errno"`
		Sign   string `json:"sign"`
		ImgURL string `json:"imgurl"`
	}
	if err := json.Unmarshal([]byte(trimJSONP(string(body))), &resp); err != nil {
		return nil, fmt.Errorf("解析二维码响应失败: %w", err)
	}
	if resp.Errno != 0 || resp.Sign == "" {
		return nil, fmt.Errorf("获取二维码失败: errno=%d", resp.Errno)
	}
	imgURL := resp.ImgURL
	if !strings.HasPrefix(imgURL, "http") {
		imgURL = "https://" + imgURL
	}
	qrURL := "https://wappass.baidu.com/wp/?" + url.Values{
		"qrlogin": {""},
		"t":       {strconv.FormatInt(time.Now().Unix(), 10)},
		"error":   {"0"},
		"sign":    {resp.Sign},
		"cmd":     {"login"},
		"lp":      {"pc"},
		"tpl":     {""},
		"uaonly":  {""},
	}.Encode()

	return &Login{
		Sign:   resp.Sign,
		QRURL:  qrURL,
		ImgURL: imgURL,
		gid:    gid,
		client: client,
	}, nil
}

// Wait 等待用户扫码并换发登录凭证（阻塞轮询直到成功/超时）。
//
// interval 为轮询间隔（默认 2s；unicast 是长轮询，未扫码时约 30s 返回一次）。
func (l *Login) Wait(ctx context.Context, timeout time.Duration, interval ...time.Duration) (*Credentials, error) {
	gap := 2 * time.Second
	if len(interval) > 0 && interval[0] > 0 {
		gap = interval[0]
	}
	deadline := time.Now().Add(timeout)
	for {
		status, v, err := l.Probe(ctx)
		if err != nil {
			return nil, err
		}
		if status == 0 && v != "" {
			l.ticket = v
			return l.Exchange(ctx)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("扫码登录超时（%.0f 秒）", timeout.Seconds())
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(gap):
		}
	}
}

// Probe 查一次扫码状态（诊断/自管轮询用）。
//
// 返回 status 与 v：-1=尚未扫码（errno!=0）、1=已扫待App确认、
// 0=已确认（v 为登录票据，交给 Exchange 换发）。
// 网络抖动/长轮询超时按"尚未扫码"处理（-1），不返回错误。
func (l *Login) Probe(ctx context.Context) (status int, v string, err error) {
	q := url.Values{
		"channel_id": {l.Sign},
		"tpl":        {"pp"},
		"gid":        {l.gid},
		"apiver":     {"v3"},
		"tt":         {ms()},
	}
	body, err := get(ctx, l.client, "https://passport.baidu.com/channel/unicast", q, "https://passport.baidu.com/v2/?login")
	if err != nil {
		return -1, "", nil // 长轮询超时/网络抖动视为等待
	}
	var resp struct {
		Errno    int    `json:"errno"`
		ChannelV string `json:"channel_v"`
	}
	if err := json.Unmarshal([]byte(trimJSONP(string(body))), &resp); err != nil {
		return -1, "", nil
	}
	if resp.Errno != 0 || resp.ChannelV == "" {
		return -1, "", nil
	}
	var cv struct {
		Status int    `json:"status"`
		V      string `json:"v"`
	}
	if err := json.Unmarshal([]byte(resp.ChannelV), &cv); err != nil {
		return -1, "", nil
	}
	if cv.Status == 0 && cv.V != "" {
		l.ticket = cv.V // 命中即存，Probe→Exchange 手动流程可直接换发
	}
	return cv.Status, cv.V, nil
}

// Exchange 用登录票据换发 BDUSS/STOKEN（Set-Cookie 自动落入 client 的 cookiejar）。
//
// 响应是单引号 JSONP（callback({'errInfo':{'no':'0',...}})），成功判定前先归一化引号。
func (l *Login) Exchange(ctx context.Context) (*Credentials, error) {
	if l.ticket == "" {
		return nil, fmt.Errorf("尚未取得登录票据（Wait 成功或 Probe 返回 status=0 后再调用）")
	}
	// 注意作用域：qrbdusslogin 的 tpl 决定会话作用域——tpl=pp 换发的是通行证/贴吧
	// 作用域 BDUSS（pan 接口报 -6 用户未登录，2026-09-12 实测）；网盘要用
	// tpl=netdisk + u 指向 pan.baidu.com，并随后访问 pan 首页完成跨域 SSO 落 cookie。
	q := url.Values{
		"v":            {ms() + "00"},
		"bduss":        {l.ticket},
		"u":            {"https%3A%2F%2Fpan.baidu.com%2Fdisk%2Fhome"},
		"loginVersion": {"v4"},
		"qrcode":       {"1"},
		"tpl":          {"netdisk"},
		"apiver":       {"v3"},
		"tt":           {ms()},
		"callback":     {"callback"},
	}
	body, err := get(ctx, l.client, "https://passport.baidu.com/v3/login/main/qrbdusslogin", q, "https://passport.baidu.com/v2/?login")
	if err != nil {
		return nil, fmt.Errorf("换发登录凭证失败: %w", err)
	}
	// 响应 JSONP 混用双/单引号且带尾逗号，严格 json.Unmarshal 必失败；
	// 成功标志就是 errInfo.no == "0"（与 netcccyun/toolbox 判定一致），用正则取。
	trimmed := trimJSONP(string(body))
	if !reLoginOK.MatchString(trimmed) {
		return nil, fmt.Errorf("换发登录凭证失败: %s", trimTail(trimmed, 200))
	}

	// 走一次 pan 首页：qrbdusslogin 的跨域 SSO 在这里给 pan.baidu.com 补 STOKEN 等 cookie
	_, _ = get(ctx, l.client, "https://pan.baidu.com/disk/home", url.Values{}, "https://pan.baidu.com/")

	// 从 cookiejar 提取 BDUSS / STOKEN（可能在 .baidu.com 或 pan.baidu.com 域上）
	cred := &Credentials{}
	seen := map[string]bool{}
	for _, domain := range []string{"https://pan.baidu.com/", "https://passport.baidu.com/", "https://baidu.com/"} {
		uo, _ := url.Parse(domain)
		for _, ck := range l.client.Jar.Cookies(uo) {
			if seen[ck.Name] {
				continue
			}
			seen[ck.Name] = true
			cred.Raw = strings.TrimSpace(cred.Raw + "; " + ck.Name + "=" + ck.Value)
			switch ck.Name {
			case "BDUSS":
				cred.BDUSS = ck.Value
			case "STOKEN":
				cred.STOKEN = ck.Value
			}
		}
	}
	cred.Raw = strings.TrimPrefix(cred.Raw, "; ")
	if cred.BDUSS == "" {
		return nil, fmt.Errorf("换发登录凭证失败：cookie 中没有 BDUSS | %s", trimTail(string(body), 150))
	}
	return cred, nil
}

// ---------- 内部 ----------

func get(ctx context.Context, client *http.Client, base string, params url.Values, referer string) ([]byte, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if referer == "" {
		referer = "https://passport.baidu.com/v2/?login"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", referer)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, trimTail(string(body), 120))
	}
	return body, nil
}

// newGID 生成百度 passport 的 gid（GUID 大写十六进制 8-4-4-4-12）。
func newGID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	h := strings.ToUpper(hex.EncodeToString(b[:]))
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:], nil
}

// trimJSONP 剥掉 callback(...) 包装（部分接口返回 JSONP）。
func trimJSONP(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "{") {
		return s
	}
	if i := strings.Index(s, "("); i > 0 && strings.HasSuffix(s, ")") {
		return strings.TrimSpace(s[i+1 : len(s)-1])
	}
	return s
}

func trimTail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}

func ms() string {
	return strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)
}
