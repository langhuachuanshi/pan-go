// Package invoker 定义 quark 业务子包依赖的调用接口与错误辅助。
//
// 接口即 core 标准菜单（实现方=主包 Client，执行走 core/httpx）；
// 错误统一 coreerrors，本包提供 quark 码表便捷构造与兼容别名。
package invoker

import (
	"context"
	"fmt"

	coreerrors "github.com/langhuachuanshi/pan-go/core/errors"
	coreinvoker "github.com/langhuachuanshi/pan-go/core/invoker"
)

// Invoker 业务子包依赖的调用接口（core 标准菜单）。
type Invoker = coreinvoker.Invoker

// APIError 统一错误（coreerrors.APIError 别名）。
type APIError = coreerrors.APIError

// quark 业务码 → 语义映射（实测确认，见模块 AGENTS）。
func kindOf(code int) coreerrors.Kind {
	switch code {
	case 31001, 31003:
		return coreerrors.KindAuth
	case 41013:
		return coreerrors.KindRateLimited
	case 31005:
		return coreerrors.KindNotFound
	}
	return coreerrors.KindOther
}

// NewAPIError 构造 quark 业务错误（自动按码表打语义 Kind）。
func NewAPIError(code int, message string) *APIError {
	return coreerrors.New("quark", code, 200, message, kindOf(code))
}

// IsAuthError 登录态失效判定（coreerrors.IsAuth 别名，兼容既有调用方）。
func IsAuthError(err error) bool { return coreerrors.IsAuth(err) }

// GetAndDecode GET + 解码（助手保留既有签名，业务包零改动）。
func GetAndDecode(ctx context.Context, inv Invoker, path string, params, out any) error {
	return inv.Get(ctx, path, toStrMap(params), out)
}

// PostAndDecode POST + 解码。
func PostAndDecode(ctx context.Context, inv Invoker, path string, body, params, out any) error {
	return inv.Post(ctx, path, body, toStrMap(params), out)
}

// toStrMap 参数归一化（历史签名兼容：any 值统一转字符串）。
func toStrMap(v any) map[string]string {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]string); ok {
		return m
	}
	if am, ok := v.(map[string]any); ok {
		out := make(map[string]string, len(am))
		for k, val := range am {
			out[k] = fmt.Sprintf("%v", val)
		}
		return out
	}
	return nil
}
