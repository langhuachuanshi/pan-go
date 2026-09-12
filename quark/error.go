package quark

import "github.com/langhuachuanshi/pan-go/quark/invoker"

// APIError 夸克业务错误（coreerrors.APIError 别名，经 invoker 透出）。
//
// 错误语义判定用 coreerrors.IsAuth / IsRateLimited / IsNotFound
// （码表映射在 invoker 包：31001/31003→Auth，41013→RateLimited，31005→NotFound）。
type APIError = invoker.APIError

// NewAPIError 构造错误。
func NewAPIError(code int, message string) *APIError {
	return invoker.NewAPIError(code, message)
}
