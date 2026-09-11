package alipan

import "github.com/langhuachuanshi/alipan-go/alipan/invoker"

// 本文件重导出错误类型，方便用户直接用 alipan.APIError 而不必 import invoker 包。
//
// 错误处理约定：
//   - 所有 API 失败返回的 error，底层都是 *invoker.APIError（主包以别名 APIError 暴露）。
//   - 业务错误可经 errors.As(&apiErr) 取出，含 StatusCode/Code/Message/RequestID。
//   - 认证类失败有哨兵错误：auth.ErrLoginFailed / auth.ErrRefreshFailed。

// APIError 是阿里云盘业务错误的统一封装（invoker.APIError 的别名）。
//
// 用法：
//
//	var apiErr *alipan.APIError
//	if errors.As(err, &apiErr) {
//	    fmt.Println(apiErr.StatusCode, apiErr.Code)
//	}
type APIError = invoker.APIError

// NewAPIError 构造一个 APIError。
func NewAPIError(statusCode int, code, message string) *APIError {
	return invoker.NewAPIError(statusCode, code, message)
}

// ParseAPIError 从 HTTP 响应体解析出 APIError。
func ParseAPIError(statusCode int, body []byte) *APIError {
	return invoker.ParseAPIError(statusCode, body)
}
