package quark

import "github.com/langhuachuanshi/quark-go/quark/invoker"

// APIError 夸克业务错误（invoker.APIError 别名）。
//
// 夸克错误约定：code != 0 或 status != 200 即失败。
// 常见 code：
//   - 31003: cookie 失效/未登录
//   - 31005: 文件不存在
//   - 41013: 转存频率限制
type APIError = invoker.APIError

// NewAPIError 构造错误。
func NewAPIError(code int, message string) *APIError {
	return invoker.NewAPIError(code, message)
}
