package lanzou

import "github.com/langhuachuanshi/pan-go/lanzou/invoker"

// 哨兵错误定义在 invoker 包（子包也能用），此处别名透出，保持 lanzou.ErrXxx 的调用习惯。
var (
	ErrNotLoggedIn    = invoker.ErrNotLoggedIn
	ErrFileExpired    = invoker.ErrFileExpired
	ErrPasswordWrong  = invoker.ErrPasswordWrong
	ErrFileSizeLimit  = invoker.ErrFileSizeLimit
	ErrInvalidURL     = invoker.ErrInvalidURL
	ErrExtractFailed  = invoker.ErrExtractFailed
	ErrUploadFailed   = invoker.ErrUploadFailed
	ErrDownloadFailed = invoker.ErrDownloadFailed
	ErrAPIError       = invoker.ErrAPIError
)
