package share

import (
	"context"

	"github.com/langhuachuanshi/alipan-go/alipan/invoker"
)

// 本文件实现快传分享（PrivateShare），对应该 aligo 的 /adrive/v1/share/create。
//
// 重要：快传是阿里云盘网页端可用的官方分享方式，生成 /t/ 链接。
// 关键约束：分享的文件必须在【资源盘】，备份盘的文件不允许分享。
// 因此本实现默认用 invoker.ResourceDriveID()。

// QuickShareRequest 快传分享请求。
type QuickShareRequest struct {
	// DriveFileList 要分享的文件列表（drive_id + file_id）。
	// 若不填，用 Files + DriveID 自动构造。
	DriveFileList []DriveFileItem
	// Files 文件 ID 列表（DriveFileList 为空时用，配合 DriveID）。
	Files []string
	// DriveID 网盘 ID。为空时自动用资源盘 ID（快传要求资源盘）。
	DriveID string
}

// DriveFileItem drive+file 标识对。
type DriveFileItem struct {
	DriveID string `json:"drive_id"`
	FileID  string `json:"file_id"`
}

// QuickShareResponse 快传响应。
type QuickShareResponse struct {
	ShareID       string `json:"share_id"`
	ShareURL      string `json:"share_url"`
	ShareName     string `json:"share_name"`
	ShareTitle    string `json:"share_title"`
	ShareSubtitle string `json:"share_subtitle"`
	Expiration    string `json:"expiration"`
	Expired       bool   `json:"expired"`
	Thumbnail     string `json:"thumbnail"`
}

// QuickShare 创建快传分享，返回官方分享链接（https://www.alipan.com/t/xxx）。
//
// 注意：快传要求文件在【资源盘】。若 DriveID 为空，自动用资源盘 ID；
// 若账号无资源盘，返回错误。
func (s *Service) QuickShare(ctx context.Context, req *QuickShareRequest) (*QuickShareResponse, error) {
	if req == nil || (len(req.DriveFileList) == 0 && len(req.Files) == 0) {
		return nil, invoker.NewAPIError(0, "InvalidArgument", "files is required")
	}

	// 确定-drive_id：优先用 req.DriveID，否则用资源盘。
	driveID := req.DriveID
	if driveID == "" {
		driveID = s.inv.ResourceDriveID()
		if driveID == "" {
			return nil, invoker.NewAPIError(0, "NoResourceDrive", "快传需要资源盘，但账号未找到资源盘 drive")
		}
	}

	// 构造 drive_file_list。
	var list []DriveFileItem
	if len(req.DriveFileList) > 0 {
		list = req.DriveFileList
	} else {
		list = make([]DriveFileItem, 0, len(req.Files))
		for _, fid := range req.Files {
			list = append(list, DriveFileItem{DriveID: driveID, FileID: fid})
		}
	}

	body := map[string]any{
		"drive_file_list": list,
	}
	var resp QuickShareResponse
	if err := invoker.PostAndDecode(ctx, s.inv, pathQuickShareCreate, body, &resp, []int{200}); err != nil {
		return nil, err
	}
	return &resp, nil
}

// pathQuickShareCreate 快传分享接口路径。
const pathQuickShareCreate = "/adrive/v1/share/create"
