// Package invoker 定义 panbaidu-go 各业务子包共享的 HTTP 调用接口与错误类型。
//
// 设计同 quark-go：主包 Client 实现 Invoker，各业务子包依赖接口而非主包，
// 避免循环依赖。
//
// 网页端（BDUSS 方案）的特点：
//   - BDUSS + STOKEN cookie 鉴权，由 cookiejar 自动携带
//   - 通用 query（channel=chunlei&web=1&app_id=250528&clienttype=0）由 client 自动注入
//   - 写操作（PostForm/PostMultipart）的 bdstoken 由 client 自动注入
//   - 所有接口走 pan.baidu.com，响应外层 {errno, ...}，errno==0 才成功
package invoker

import (
	"context"
	"encoding/json"
	"fmt"
)

// APIError 百度业务错误。
// 百度错误约定：errno != 0 即失败（errno==0 成功）。
type APIError struct {
	Errno   int    // 0=成功，非0=失败
	Message string // 错误描述
}

func NewAPIError(errno int, message string) *APIError {
	return &APIError{Errno: errno, Message: message}
}

func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("baidu: errno=%d message=%s", e.Errno, e.Message)
}

// Invoker 各业务子包依赖的调用接口。
// path 都是相对 pan.baidu.com 的路径（如 /api/list、/api/create）。
type Invoker interface {
	// Get 发 GET 请求（list 用）。通用 query + cookie 自动注入，无需 bdstoken。
	Get(ctx context.Context, path string, params map[string]string) ([]byte, int, error)

	// PostForm 发 POST form 请求（写操作）。通用 query + cookie + bdstoken 自动注入。
	PostForm(ctx context.Context, path string, body map[string]string, params map[string]string) ([]byte, int, error)

	// PostMultipart 发 POST multipart 请求（仅分片上传用）。通用 query + cookie + bdstoken 自动注入。
	PostMultipart(ctx context.Context, baseURL, path string, params map[string]string, fieldName, fileName string, data []byte) ([]byte, int, error)
}

// Decode 反序列化，空体不报错。
func Decode(data []byte, out any) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

// GetAndDecode GET + 反序列化。
func GetAndDecode(ctx context.Context, inv Invoker, path string, params map[string]string, out any) error {
	data, _, err := inv.Get(ctx, path, params)
	if err != nil {
		return err
	}
	if out != nil {
		return Decode(data, out)
	}
	return nil
}

// PostFormAndDecode POST form + 反序列化。
func PostFormAndDecode(ctx context.Context, inv Invoker, path string, body, params map[string]string, out any) error {
	data, _, err := inv.PostForm(ctx, path, body, params)
	if err != nil {
		return err
	}
	if out != nil {
		return Decode(data, out)
	}
	return nil
}

// PostMultipartAndDecode POST multipart + 反序列化。
func PostMultipartAndDecode(ctx context.Context, inv Invoker, baseURL, path string, params map[string]string, fieldName, fileName string, data []byte, out any) error {
	respData, _, err := inv.PostMultipart(ctx, baseURL, path, params, fieldName, fileName, data)
	if err != nil {
		return err
	}
	if out != nil {
		return Decode(respData, out)
	}
	return nil
}
