// Package drive 实现网盘相关 API，对应该 aligo 的 Drive.py。
package drive

import (
	"context"

	"github.com/langhuachuanshi/alipan-go/alipan/invoker"
	"github.com/langhuachuanshi/alipan-go/alipan/types"
)

// Service 网盘相关操作。
type Service struct {
	inv invoker.Invoker
}

// New 创建 drive Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

const (
	pathDriveGet            = "/v2/drive/get"
	pathDriveGetDefault     = "/v2/drive/get_default_drive"
	pathDriveListMyDrives   = "/v2/drive/list_my_drives"
	pathDriveCapacityDetail = "/adrive/v1/user/driveCapacityDetails"
)

// Get 获取指定网盘详情。
func (s *Service) Get(ctx context.Context, driveID string) (*types.BaseDrive, error) {
	body := map[string]any{}
	if driveID != "" {
		body["drive_id"] = driveID
	}
	var d types.BaseDrive
	if err := invoker.PostAndDecode(ctx, s.inv, pathDriveGet, body, &d, []int{200}); err != nil {
		return nil, err
	}
	return &d, nil
}

// GetDefault 获取默认网盘。
func (s *Service) GetDefault(ctx context.Context) (*types.BaseDrive, error) {
	var d types.BaseDrive
	if err := invoker.PostAndDecode(ctx, s.inv, pathDriveGetDefault, map[string]any{}, &d, []int{200}); err != nil {
		return nil, err
	}
	return &d, nil
}

// ListMyDrives 列出我的所有网盘。
func (s *Service) ListMyDrives(ctx context.Context) ([]*types.BaseDrive, error) {
	body := map[string]any{}
	var drives []*types.BaseDrive
	for {
		var resp struct {
			Items      []*types.BaseDrive `json:"items"`
			NextMarker string             `json:"next_marker"`
		}
		if err := invoker.PostAndDecode(ctx, s.inv, pathDriveListMyDrives, body, &resp, []int{200}); err != nil {
			return nil, err
		}
		drives = append(drives, resp.Items...)
		if resp.NextMarker == "" {
			break
		}
		body["marker"] = resp.NextMarker
	}
	return drives, nil
}

// Capacity 获取容量详情。
func (s *Service) Capacity(ctx context.Context) (*types.DriveCapacityDetail, error) {
	var d types.DriveCapacityDetail
	if err := invoker.PostAndDecode(ctx, s.inv, pathDriveCapacityDetail, map[string]any{}, &d, []int{200}); err != nil {
		return nil, err
	}
	return &d, nil
}
