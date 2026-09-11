// Package invoker 定义 quark-go 各业务子包共享的 HTTP 调用接口与错误类型。
//
// 设计同 alipan-go：主包 Client 实现 Invoker 接口，各业务子包依赖接口而非主包，
// 避免循环依赖。夸克的特点：基于 cookie 鉴权，无 token、无签名。
package invoker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// APIError 夸克网盘业务错误统一封装。
// 注意夸克的错误体：{"code":0/非0,"status":非200,"message":"...","data":...}
// code==0 且 status==200 才是成功。
type APIError struct {
	Code    int    `json:"code"`    // 0=成功，非0=失败
	Status  int    `json:"-"`       // HTTP 状态码
	Message string `json:"message"` // 错误描述
}

func NewAPIError(code int, message string) *APIError {
	return &APIError{Code: code, Message: message}
}

func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("quark: code=%d status=%d message=%s", e.Code, e.Status, e.Message)
}

// IsAuthError 判断错误是否为登录态失效（未登录/cookie 过期），调用方应引导重新
// 扫码登录（quark/qrcode.Create）。实测（2026-09-12）：31001 = require login，
// 31003 = cookie 失效。
func IsAuthError(err error) bool {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae.Code == 31001 || ae.Code == 31003
	}
	return false
}

// Invoker 各业务子包依赖的调用接口。
//
// 夸克 API 约定：
//   - base URL: https://drive-pc.quark.cn/1/clouddrive/
//   - 多数接口是 POST，部分是 GET
//   - query 里要带固定的 pr（如 pr=uqm&fr=pc）和分页参数（_page/_size）
//   - cookie 通过底层 http.Client 的 cookiejar 注入
type Invoker interface {
	// Get 发 GET 请求，path 是相对 base 的路径（如 file/sort 或 /file/sort）。
	Get(ctx context.Context, path string, params map[string]string, headers map[string]string) ([]byte, int, error)
	// Post 发 POST JSON 请求。
	Post(ctx context.Context, path string, body any, params map[string]string, headers map[string]string) ([]byte, int, error)
	// DownloadHeaders 返回下载直链所需的鉴权头（Cookie/Referer/User-Agent）。
	// 夸克 /file/download 返回的临时直链 GET 时需要带登录态，否则 403 RequestDeniedByCallback。
	DownloadHeaders() map[string]string
}

// Decode 反序列化，空体不报错。
func Decode(data []byte, out any) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

// PostAndDecode POST + 反序列化。
func PostAndDecode(ctx context.Context, inv Invoker, path string, body, params, out any) error {
	data, _, err := inv.Post(ctx, path, body, toStrMap(params), nil)
	if err != nil {
		return err
	}
	if out != nil {
		return Decode(data, out)
	}
	return nil
}

// GetAndDecode GET + 反序列化。
func GetAndDecode(ctx context.Context, inv Invoker, path string, params, out any) error {
	data, _, err := inv.Get(ctx, path, toStrMap(params), nil)
	if err != nil {
		return err
	}
	if out != nil {
		return Decode(data, out)
	}
	return nil
}

// toStrMap 把 any 值的 map 转成 string（params 里的值多为 int，统一转字符串）。
func toStrMap(v any) map[string]string {
	if v == nil {
		return nil
	}
	m, ok := v.(map[string]string)
	if ok {
		return m
	}
	// 尝试 map[string]any。
	if am, ok := v.(map[string]any); ok {
		out := make(map[string]string, len(am))
		for k, val := range am {
			out[k] = fmt.Sprintf("%v", val)
		}
		return out
	}
	return nil
}
