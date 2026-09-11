// Package user 实现用户相关 API，对应该 aligo 的 User.py。
package user

import (
	"context"

	"github.com/langhuachuanshi/alipan-go/alipan/invoker"
	"github.com/langhuachuanshi/alipan-go/alipan/types"
)

// Service 用户相关操作。
type Service struct {
	inv invoker.Invoker
}

// New 创建 user Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

const pathUserGet = "/v2/user/get"

// Get 获取当前登录用户信息。
func (s *Service) Get(ctx context.Context) (*types.BaseUser, error) {
	var u types.BaseUser
	if err := invoker.PostAndDecode(ctx, s.inv, pathUserGet, map[string]any{}, &u, []int{200}); err != nil {
		return nil, err
	}
	return &u, nil
}
