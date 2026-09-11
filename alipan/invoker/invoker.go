// Package invoker 定义 alipan-go 各业务子包共享的 HTTP 调用接口与错误类型。
//
// 设计目的：打破循环依赖。主包 Client 实现这些接口，各业务子包
//（file/share/drive/user）只依赖接口而非主包，从而子包之间、子包与主包之间无循环。
package invoker

import (
	"context"
	"encoding/json"
	"fmt"
)

// APIError 是阿里云盘业务错误的统一封装。注意错误体字段是 camelCase。
type APIError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	RequestID  string `json:"requestId"`
	ResultCode string `json:"resultCode"`
}

// NewAPIError 构造 APIError。
func NewAPIError(statusCode int, code, message string) *APIError {
	return &APIError{StatusCode: statusCode, Code: code, Message: message}
}

// Error 实现 error 接口。
func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.RequestID != "" {
		return fmt.Sprintf("alipan: status=%d code=%s message=%s requestId=%s", e.StatusCode, e.Code, e.Message, e.RequestID)
	}
	return fmt.Sprintf("alipan: status=%d code=%s message=%s", e.StatusCode, e.Code, e.Message)
}

// Is 支持 errors.Is，按 StatusCode + Code 判等。
func (e *APIError) Is(target error) bool {
	t, ok := target.(*APIError)
	if !ok {
		return false
	}
	return e.StatusCode == t.StatusCode && e.Code == t.Code
}

// ParseAPIError 从 HTTP 响应体解析出 APIError。
func ParseAPIError(statusCode int, body []byte) *APIError {
	e := &APIError{StatusCode: statusCode}
	if len(body) == 0 {
		e.Code = "HTTPError"
		e.Message = fmt.Sprintf("http status %d with empty body", statusCode)
		return e
	}
	tmp := struct {
		Code       string `json:"code"`
		Message    string `json:"message"`
		RequestID  string `json:"requestId"`
		ResultCode string `json:"resultCode"`
	}{}
	if err := json.Unmarshal(body, &tmp); err != nil {
		e.Code = "HTTPError"
		msg := string(body)
		if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
		e.Message = fmt.Sprintf("http status %d: %s", statusCode, msg)
		return e
	}
	e.Code = tmp.Code
	e.Message = tmp.Message
	e.RequestID = tmp.RequestID
	e.ResultCode = tmp.ResultCode
	if e.Code == "" {
		e.Code = "HTTPError"
	}
	if e.Message == "" {
		e.Message = fmt.Sprintf("http status %d", statusCode)
	}
	return e
}

// Invoker 是各业务子包依赖的调用接口。主包 Client 实现它。
//
// 调用约定：
//   - Post 发 POST JSON 到 hostAPI+path，自动注入 default_drive_id（若实现了该逻辑）。
//   - 返回响应体字节和 HTTP 状态码；失败时 err 为 *APIError。
//   - okStatus 指定视为成功的状态码集合（默认 {200}）。
type Invoker interface {
	Post(ctx context.Context, path string, body any, extraHeaders map[string]string, okStatus []int) ([]byte, int, error)
	// PostAnon 发匿名 POST（不带 access_token），用于访问他人分享等场景。
	PostAnon(ctx context.Context, path string, body any, extraHeaders map[string]string, okStatus []int) ([]byte, int, error)
	// PostRaw 发 POST 但 path 已是完整 URL（用于刷新 token 等不走 hostAPI 的场景）。
	PostRaw(ctx context.Context, fullURL string, body any, withAuth bool, extraHeaders map[string]string, okStatus []int) ([]byte, int, error)
	// DefaultDriveID 返回默认网盘 ID（请求体 drive_id 为空时用）。
	DefaultDriveID() string
	// AccessToken 返回当前 access_token（上传 proof_code 计算需要）。
	AccessToken() string
	// UserID 返回当前用户 ID（分享列表接口 creator 字段必需）。
	UserID() string
	// ResourceDriveID 返回资源盘 ID（快传分享需要资源盘，备份盘不允许分享）。
	// 若账号无资源盘，返回空串。
	ResourceDriveID() string
}

// Decode 把响应字节解码到 out，空体不报错。
func Decode(data []byte, out any) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

// PostAndDecode 是最常用的封装：POST + 反序列化。各 service 包复用。
func PostAndDecode(ctx context.Context, inv Invoker, path string, body, out any, okStatus []int) error {
	data, _, err := inv.Post(ctx, path, body, nil, okStatus)
	if err != nil {
		return err
	}
	if out != nil {
		return Decode(data, out)
	}
	return nil
}

// PostAndDecodeWithHeaders 同上，但支持额外 header（如 x-share-token）。
func PostAndDecodeWithHeaders(ctx context.Context, inv Invoker, path string, body, out any, headers map[string]string, okStatus []int) error {
	data, _, err := inv.Post(ctx, path, body, headers, okStatus)
	if err != nil {
		return err
	}
	if out != nil {
		return Decode(data, out)
	}
	return nil
}

// PostAndDecodeAnon 匿名 POST + 反序列化（访问他人分享用，不带 access_token）。
func PostAndDecodeAnon(ctx context.Context, inv Invoker, path string, body, out any, okStatus []int) error {
	data, _, err := inv.PostAnon(ctx, path, body, nil, okStatus)
	if err != nil {
		return err
	}
	if out != nil {
		return Decode(data, out)
	}
	return nil
}

// PostAndDecodeAnonWithHeaders 匿名 POST + 额外 header + 反序列化（如匿名 + x-share-token）。
func PostAndDecodeAnonWithHeaders(ctx context.Context, inv Invoker, path string, body, out any, headers map[string]string, okStatus []int) error {
	data, _, err := inv.PostAnon(ctx, path, body, headers, okStatus)
	if err != nil {
		return err
	}
	if out != nil {
		return Decode(data, out)
	}
	return nil
}
