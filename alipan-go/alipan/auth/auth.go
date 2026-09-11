package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/langhuachuanshi/alipan-go/alipan/invoker"
	"github.com/langhuachuanshi/alipan-go/alipan/types"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// 本文件实现扫码登录与 token 刷新，对应该 aligo Auth._login / _login_by_qrcode / _refresh_token。

// ErrLoginFailed 扫码登录失败。
var ErrLoginFailed = invoker.NewAPIError(0, "LoginFailed", "qr code login failed")

// ErrRefreshFailed token 刷新失败。
var ErrRefreshFailed = invoker.NewAPIError(0, "RefreshFailed", "refresh token failed")

// debugLog 控制扫码登录的调试输出。测试期间可设为 true 排查问题。
var debugLog = false

// DebugLogin 开启/关闭扫码登录的调试日志。
func DebugLogin(on bool) { debugLog = on }

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[auth] "+format+"\n", args...)
}

// keysOf 返回 map 的 key 列表（用于调试）。
func keysOf(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// anyToStr 把任意值转成字符串，正确处理 json.Unmarshal 产生的 float64 数字。
// 关键：时间戳等大整数会被解析成 float64（如 1.78e12），必须格式化成整数字符串，
// 不能用 %v（会输出科学计数法），否则服务端解析失败返回 HTML 错误页。
func anyToStr(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		// 整数值的 float64 → 整数字符串。
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

// Login 执行登录决策。返回登录产物（含 token）。
//
// 决策顺序（对齐 aligo Auth.__init__）：
//  1. 若 cfg.Token 已提供，直接用。
//  2. 否则尝试从配置文件加载；若同时给了 RefreshToken，用它刷新覆盖。
//  3. 无配置但给了 RefreshToken，用它登录。
//  4. 否则扫码登录。
func Login(ctx context.Context, cfg *Config) (*Result, error) {
	if cfg.Token != nil {
		return &Result{Token: cfg.Token, DeviceID: deviceID(cfg.Token)}, nil
	}

	// 尝试加载本地配置。
	if loaded := loadToken(cfg.Name, cfg.ConfigDir); loaded != nil {
		if cfg.RefreshToken != "" {
			if err := RefreshToken(ctx, loaded, cfg.RefreshToken, cfg.Name, cfg.ConfigDir, true); err != nil {
				return nil, err
			}
		}
		return &Result{Token: loaded, DeviceID: deviceID(loaded)}, nil
	}

	// refresh_token 直登。
	if cfg.RefreshToken != "" {
		t := &types.Token{}
		if err := RefreshToken(ctx, t, cfg.RefreshToken, cfg.Name, cfg.ConfigDir, true); err != nil {
			return nil, err
		}
		return &Result{Token: t, DeviceID: deviceID(t)}, nil
	}

	// 扫码登录。
	t, err := loginByQrcode(ctx, cfg)
	if err != nil {
		return nil, err
	}
	// 持久化。
	saveToken(t, cfg.Name, cfg.ConfigDir)
	return &Result{Token: t, DeviceID: deviceID(t)}, nil
}

// RefreshToken 用 refreshToken 刷新 access_token，更新并持久化 token。
// loopCall=true 表示来自登录链路（防 502 递归）。
func RefreshToken(ctx context.Context, token *types.Token, refreshToken string, name, configDir string, loopCall bool) error {
	body := map[string]string{
		"refresh_token": refreshToken,
		"grant_type":    "refresh_token",
	}
	refreshURL := HostAPI + PathAccountToken

	for attempt := 0; ; attempt++ {
		data, status, err := postJSON(ctx, refreshURL, body, false)
		if err != nil {
			if attempt >= 1 {
				return fmt.Errorf("alipan: refresh token network error: %w", err)
			}
			if ctxErr := sleepCtx(ctx, time.Second); ctxErr != nil {
				return ctxErr
			}
			continue
		}
		switch status {
		case http.StatusOK:
			var t types.Token
			if err := json.Unmarshal(data, &t); err != nil {
				return fmt.Errorf("alipan: decode token response failed: %w", err)
			}
			devID := deviceID(token)
			if devID == "" {
				devID = newDeviceID()
			}
			t.XDeviceID = &devID
			// 把刷新前的 user 信息保留（刷新响应可能不含）。
			copyTokenFields(token, &t)
			*token = t
			saveToken(token, name, configDir)
			return nil
		case http.StatusBadGateway:
			if loopCall {
				return ErrRefreshFailed
			}
			if ctxErr := sleepCtx(ctx, 10*time.Second); ctxErr != nil {
				return ctxErr
			}
			loopCall = true
			continue
		default:
			return invoker.ParseAPIError(status, data)
		}
	}
}

// copyTokenFields 把 src 中 dst 没有的关键字段补过去（刷新响应可能不含 user 信息）。
func copyTokenFields(src, dst *types.Token) {
	if dst.XDeviceID == nil && src != nil {
		dst.XDeviceID = src.XDeviceID
	}
}

// loginByQrcode 扫码登录完整流程。
func loginByQrcode(ctx context.Context, cfg *Config) (*types.Token, error) {
	sess := newLoginSession()
	if err := sess.generateQrcode(ctx); err != nil {
		return nil, err
	}

	// 展示二维码：优先用自定义回调，否则按 LoginMethod 走默认。
	if cfg.ShowQR != nil {
		if err := cfg.ShowQR(sess.qr.CodeContent); err != nil {
			return nil, err
		}
	} else {
		switch cfg.LoginMethod {
		case LoginWeb:
			if err := sess.showQrcodeWeb(ctx, cfg.WebPort, sess.qr.CodeContent); err != nil {
				return nil, err
			}
			defer sess.stopWeb()
		default:
			showQrcodeTerminal(sess.qr.CodeContent)
		}
	}

	confirmed, err := sess.pollStatus(ctx, loginTimeout(cfg))
	if err != nil {
		return nil, err
	}
	refreshToken, err := parseBizExt(confirmed.BizExt)
	if err != nil {
		return nil, fmt.Errorf("alipan: parse bizExt failed: %w", err)
	}

	var t types.Token
	if err := RefreshToken(ctx, &t, refreshToken, cfg.Name, cfg.ConfigDir, true); err != nil {
		return nil, err
	}
	return &t, nil
}

func loginTimeout(cfg *Config) time.Duration {
	if cfg.LoginTimeout > 0 {
		return cfg.LoginTimeout
	}
	return 5 * time.Minute
}

// —— 扫码会话 ——

type loginSession struct {
	httpClient *http.Client
	qr         *qrcodeInfo
	webServer  *http.Server
}

type qrcodeInfo struct {
	CodeContent string
	Raw         map[string]any
}

type confirmedInfo struct {
	BizExt string
}

func newLoginSession() *loginSession {
	jar, _ := cookiejar.New(nil)
	return &loginSession{
		httpClient: &http.Client{Timeout: 30 * time.Second, Jar: jar},
	}
}

func (s *loginSession) generateQrcode(ctx context.Context) error {
	// ① OAuth authorize（拿 SESSIONID cookie）。
	params := url.Values{
		"login_type":    {"custom"},
		"response_type": {"code"},
		"redirect_uri":  {OAuthRedirect},
		"client_id":     {ClientID},
		"state":         {`{"origin":"file://"}`},
	}
	authorizeURL := HostAuth + PathOAuthAuthorize + "?" + params.Encode()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, authorizeURL, nil)
	setCommonHeaders(req)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("alipan: oauth authorize failed: %w", err)
	}
	resp.Body.Close()

	// ② 生成二维码。
	genURL := HostPassport + PathQrcodeGenerate + "?appName=" + AppName
	req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, genURL, nil)
	setCommonHeaders(req2)
	resp2, err := s.httpClient.Do(req2)
	if err != nil {
		return fmt.Errorf("alipan: qrcode generate failed: %w", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		return invoker.ParseAPIError(resp2.StatusCode, readAll(resp2.Body))
	}
	var gen struct {
		Content struct {
			HasError bool `json:"hasError"`
			Data     map[string]any `json:"data"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&gen); err != nil {
		return fmt.Errorf("alipan: decode qrcode generate failed: %w", err)
	}
	codeContent, _ := gen.Content.Data["codeContent"].(string)
	if gen.Content.HasError || codeContent == "" {
		return fmt.Errorf("alipan: qrcode generate returned empty codeContent")
	}
	// 关键：必须把 generate.do 返回的完整 data 对象原样保存，作为 query.do 的请求体。
	// data 里除 codeContent/title 外，还含 ck、t 等会话凭证，丢失会导致 query.do 一直失败。
	s.qr = &qrcodeInfo{
		CodeContent: codeContent,
		Raw:         gen.Content.Data,
	}
	if debugLog {
		logf("generate.do data 字段: %v", keysOf(gen.Content.Data))
	}
	return nil
}

func (s *loginSession) pollStatus(ctx context.Context, timeout time.Duration) (*confirmedInfo, error) {
	queryURL := HostPassport + PathQrcodeQuery + "?appName=" + AppName
	deadline := time.Now().Add(timeout)
	pollCount := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, ErrLoginFailed
		}
		// 用 form-encoded 回传完整 data 对象（对齐 aligo 的 data=data）。
		// 注意：json.Unmarshal 到 map[string]any 后，数字变成 float64，
		// 必须格式化成整数字符串（不能用 %v，否则时间戳变成科学计数法）。
		form := url.Values{}
		for k, v := range s.qr.Raw {
			form.Set(k, anyToStr(v))
		}
		encodedForm := form.Encode()
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, queryURL, bytes.NewReader([]byte(encodedForm)))
		setCommonHeaders(req)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		// passport 域接口需要 passport 的 Referer，否则可能被重定向到 HTML 登录页。
		req.Header.Set("Referer", "https://passport.aliyundrive.com/")
		req.Header.Set("Origin", "https://passport.aliyundrive.com")
		if debugLog && pollCount == 0 {
			logf("query.do 发送 form: %s", encodedForm)
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			if debugLog {
				logf("query.do 网络错误: %v", err)
			}
			if ctxErr := sleepCtx(ctx, 3*time.Second); ctxErr != nil {
				return nil, ctxErr
			}
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		pollCount++
		if debugLog && pollCount <= 3 {
			// 前 3 次打印原始响应，便于排查结构。
			bs := string(body)
			if len(bs) > 300 {
				bs = bs[:300] + "..."
			}
			logf("query.do #%d status=%d body=%s", pollCount, resp.StatusCode, bs)
		}
		var q struct {
			Content struct {
				HasError bool `json:"hasError"`
				Data     struct {
					QrCodeStatus string `json:"qrCodeStatus"`
					BizExt       string `json:"bizExt"`
				} `json:"data"`
			} `json:"content"`
		}
		if err := json.Unmarshal(body, &q); err != nil {
			if debugLog {
				logf("query.do 解析失败: %v", err)
			}
			if ctxErr := sleepCtx(ctx, 3*time.Second); ctxErr != nil {
				return nil, ctxErr
			}
			continue
		}
		switch q.Content.Data.QrCodeStatus {
		case "NEW":
			if debugLog && pollCount == 1 {
				logf("qrCodeStatus=NEW (等待扫码)")
			}
		case "SCANED":
			if debugLog {
				logf("qrCodeStatus=SCANED (已扫码，等待手机确认)")
			}
		case "CONFIRMED":
			if debugLog {
				logf("qrCodeStatus=CONFIRMED (登录已确认)")
			}
			return &confirmedInfo{BizExt: q.Content.Data.BizExt}, nil
		default:
			if debugLog {
				logf("qrCodeStatus=%s hasError=%v", q.Content.Data.QrCodeStatus, q.Content.HasError)
			}
			if q.Content.HasError {
				return nil, ErrLoginFailed
			}
		}
		if ctxErr := sleepCtx(ctx, 3*time.Second); ctxErr != nil {
			return nil, ctxErr
		}
	}
}

// parseBizExt 解析 bizExt：base64 → gb18030 → JSON → refreshToken。
func parseBizExt(bizExt string) (string, error) {
	if bizExt == "" {
		return "", fmt.Errorf("alipan: empty bizExt")
	}
	raw, err := base64.StdEncoding.DecodeString(bizExt)
	if err != nil {
		return "", fmt.Errorf("alipan: base64 decode bizExt failed: %w", err)
	}
	dec := simplifiedchinese.GB18030.NewDecoder()
	decoded, err := io.ReadAll(dec.Reader(bytes.NewReader(raw)))
	if err != nil {
		return "", fmt.Errorf("alipan: gb18030 decode bizExt failed: %w", err)
	}
	var biz struct {
		PdsLoginResult struct {
			RefreshToken string `json:"refreshToken"`
		} `json:"pds_login_result"`
	}
	if err := json.Unmarshal(decoded, &biz); err != nil {
		return "", fmt.Errorf("alipan: parse bizExt json failed: %w", err)
	}
	if biz.PdsLoginResult.RefreshToken == "" {
		return "", fmt.Errorf("alipan: bizExt has no refreshToken")
	}
	return biz.PdsLoginResult.RefreshToken, nil
}

// —— HTTP helpers ——

// postJSON 发 JSON POST。useAuth=false 不带鉴权（刷新接口）。
func postJSON(ctx context.Context, fullURL string, body any, useAuth bool) ([]byte, int, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(b))
	setCommonHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	hc := &http.Client{Timeout: 30 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return data, resp.StatusCode, err
}

func setCommonHeaders(req *http.Request) {
	req.Header.Set("Referer", "https://aliyundrive.com")
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("x-canary", XCanary)
}

func readAll(r io.Reader) []byte {
	b, _ := io.ReadAll(r)
	return b
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
