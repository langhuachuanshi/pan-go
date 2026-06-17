// Package invoker 定义 panbaidu-go 各业务子包共享的 HTTP 调用接口与错误类型。
//
// 设计同 quark-go：主包 Client 实现 Invoker，各业务子包依赖接口而非主包，
// 避免循环依赖。
//
// 百度开放平台的特点：
//   - OAuth 鉴权（access_token），不是 cookie
//   - 业务接口域名 pan.baidu.com，分片上传域名 d.pcs.baidu.com
//   - 响应外层 {errno, ...}，errno==0 才成功
package invoker

import (
	"context"
	"encoding/json"
	"fmt"
)

// APIError 百度开放平台业务错误。
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
type Invoker interface {
	// Get 发 GET 请求（业务接口，走 pan.baidu.com）。
	// access_token 自动注入，业务参数由 params 提供。
	Get(ctx context.Context, path string, params map[string]string) ([]byte, int, error)

	// PostForm 发 POST form-urlencoded 请求（业务接口）。
	PostForm(ctx context.Context, path string, body map[string]string, params map[string]string) ([]byte, int, error)

	// PostMultipart 发 POST multipart 请求（仅分片上传用，走 PCS 域名）。
	// fieldName/fileName 是文件部分的字段名和文件名，data 是文件内容。
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
