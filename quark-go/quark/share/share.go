// Package share 实现夸克网盘分享相关操作（占位，后续实现）。
// 夸克核心场景：转存他人分享（get_share_stoken → detail → save）+ 创建分享。
package share

import (
	"github.com/langhuachuanshi/quark-go/quark/invoker"
)

// Service 分享操作入口。
type Service struct {
	inv invoker.Invoker
}

// New 创建 share Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }
