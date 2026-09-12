// Package errors 定义 pan-go 各网盘模块的统一错误类型与语义判定。
//
// 各模块把自己的业务码（夸克 code / 百度 errno / 蓝奏 zt…）映射为 Kind 语义，
// 调用方按语义判断（IsAuth / IsRateLimited…），一份逻辑通吃所有网盘。
// 注意 import 时与标准库 errors 冲突，惯例别名 coreerrors。
package errors

import (
	"fmt"
	stderrors "errors"
)

// Kind 错误语义分类。模块判错时打标签，调用方按语义而非码值判断。
type Kind uint8

const (
	KindOther      Kind = iota // 未分类
	KindAuth                   // 登录态失效 → 引导重新登录/扫码
	KindRateLimited            // 限频
	KindNotFound               // 资源不存在
	KindDenied                 // 无权限 / 风控拦截
)

func (k Kind) String() string {
	switch k {
	case KindAuth:
		return "auth"
	case KindRateLimited:
		return "rate_limited"
	case KindNotFound:
		return "not_found"
	case KindDenied:
		return "denied"
	default:
		return "other"
	}
}

// APIError 统一错误类型。
type APIError struct {
	Module  string // "quark" / "baidu" / "lanzou" / "alipan"
	Code    int    // 业务码（夸克 code、百度 errno、蓝奏 zt…）
	Status  int    // HTTP 状态码
	Message string
	kind    Kind
}

// New 构造统一错误。
func New(module string, code, status int, message string, kind Kind) *APIError {
	return &APIError{Module: module, Code: code, Status: status, Message: message, kind: kind}
}

// Error 实现 error。
func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s: code=%d status=%d kind=%s: %s", e.Module, e.Code, e.Status, e.kind, e.Message)
}

// Kind 返回语义分类。
func (e *APIError) Kind() Kind { return e.kind }

// from 从错误链中提取 *APIError。
func from(err error) (*APIError, bool) {
	var ae *APIError
	if stderrors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

// IsAuth 判断错误是否为登录态失效（各模块码表在模块侧映射为 KindAuth）。
func IsAuth(err error) bool { return isKind(err, KindAuth) }

// IsRateLimited 判断是否限频。
func IsRateLimited(err error) bool { return isKind(err, KindRateLimited) }

// IsNotFound 判断是否资源不存在。
func IsNotFound(err error) bool { return isKind(err, KindNotFound) }

// IsDenied 判断是否无权限/风控拦截。
func IsDenied(err error) bool { return isKind(err, KindDenied) }

func isKind(err error, k Kind) bool {
	if ae, ok := from(err); ok {
		return ae.kind == k
	}
	return false
}
