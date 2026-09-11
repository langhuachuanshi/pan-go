package file

import (
	"context"
	"fmt"

	"github.com/langhuachuanshi/alipan-go/alipan/invoker"
	"github.com/langhuachuanshi/alipan-go/alipan/types"
)

// CreateFolderRequest 创建文件夹请求。
type CreateFolderRequest struct {
	Name          string            `json:"name"`
	ParentFileID  string            `json:"parent_file_id"`
	DriveID       string            `json:"drive_id,omitempty"`
	CheckNameMode types.CheckNameMode `json:"check_name_mode,omitempty"`
}

// CreateFolderResp 创建文件夹响应。
type CreateFolderResp struct {
	FileID       string `json:"file_id"`
	DriveID      string `json:"drive_id"`
	ParentFileID string `json:"parent_file_id"`
	Status       string `json:"status"`
}

// CreateFolder 创建文件夹。POST /adrive/v2/file/createWithFolders，期望 201。
func (s *Service) CreateFolder(ctx context.Context, req *CreateFolderRequest) (*CreateFolderResp, error) {
	if req == nil || req.Name == "" {
		return nil, invoker.NewAPIError(0, "InvalidArgument", "name is required")
	}
	body := map[string]any{
		"name":            req.Name,
		"type":            "folder",
		"parent_file_id":  defaultStr(req.ParentFileID, "root"),
		"drive_id":        req.DriveID,
		"check_name_mode": defaultStr(string(req.CheckNameMode), "auto_rename"),
	}
	var resp CreateFolderResp
	if err := invoker.PostAndDecode(ctx, s.inv, pathFileCreateWithFolders, body, &resp, []int{201}); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CopyRequest 复制文件请求。
type CopyRequest struct {
	FileID         string `json:"file_id"`
	DriveID        string `json:"drive_id,omitempty"`
	ToParentFileID string `json:"to_parent_file_id"`
	NewName        string `json:"new_name,omitempty"`
	AutoRename     bool   `json:"auto_rename,omitempty"`
	Overwrite      bool   `json:"overwrite,omitempty"`
	ToDriveID      string `json:"to_drive_id,omitempty"`
}

// CopyResponse 复制响应。
type CopyResponse struct {
	FileID      string `json:"file_id"`
	DriveID     string `json:"drive_id"`
	DomainID    string `json:"domain_id"`
	AsyncTaskID string `json:"async_task_id"`
}

// Copy 复制文件。期望 201/202。
func (s *Service) Copy(ctx context.Context, req *CopyRequest) (*CopyResponse, error) {
	if req == nil || req.FileID == "" {
		return nil, invoker.NewAPIError(0, "InvalidArgument", "file_id is required")
	}
	body := map[string]any{
		"file_id":           req.FileID,
		"drive_id":          req.DriveID,
		"to_parent_file_id": defaultStr(req.ToParentFileID, "root"),
		"auto_rename":       req.AutoRename,
		"overwrite":         req.Overwrite,
	}
	if req.NewName != "" {
		body["new_name"] = req.NewName
	}
	if req.ToDriveID != "" {
		body["to_drive_id"] = req.ToDriveID
	}
	var resp CopyResponse
	if err := invoker.PostAndDecode(ctx, s.inv, pathFileCopy, body, &resp, []int{201, 202}); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CopyBatch 批量复制。
func (s *Service) CopyBatch(ctx context.Context, fileIDs []string, toParentFileID, driveID string) ([]BatchItem, error) {
	bodies := make([]map[string]any, len(fileIDs))
	for i, fid := range fileIDs {
		bodies[i] = map[string]any{
			"drive_id":          driveID,
			"file_id":           fid,
			"to_parent_file_id": defaultStr(toParentFileID, "root"),
			"auto_rename":       true,
		}
	}
	return s.batch(ctx, subURLFileCopy, "file", fileIDs, bodies, false)
}

// MoveRequest 移动文件请求。
type MoveRequest struct {
	FileID         string `json:"file_id"`
	DriveID        string `json:"drive_id,omitempty"`
	ToParentFileID string `json:"to_parent_file_id"`
	NewName        string `json:"new_name,omitempty"`
	AutoRename     bool   `json:"auto_rename,omitempty"`
	Overwrite      bool   `json:"overwrite,omitempty"`
	ToDriveID      string `json:"to_drive_id,omitempty"`
}

// MoveResponse 移动响应。
type MoveResponse struct {
	FileID      string `json:"file_id"`
	DriveID     string `json:"drive_id"`
	DomainID    string `json:"domain_id"`
	AsyncTaskID string `json:"async_task_id"`
}

// Move 移动文件。
func (s *Service) Move(ctx context.Context, req *MoveRequest) (*MoveResponse, error) {
	if req == nil || req.FileID == "" {
		return nil, invoker.NewAPIError(0, "InvalidArgument", "file_id is required")
	}
	body := map[string]any{
		"file_id":           req.FileID,
		"drive_id":          req.DriveID,
		"to_parent_file_id": defaultStr(req.ToParentFileID, "root"),
		"auto_rename":       req.AutoRename,
		"overwrite":         req.Overwrite,
	}
	if req.NewName != "" {
		body["new_name"] = req.NewName
	}
	if req.ToDriveID != "" {
		body["to_drive_id"] = req.ToDriveID
	}
	var resp MoveResponse
	if err := invoker.PostAndDecode(ctx, s.inv, pathFileMove, body, &resp, []int{200}); err != nil {
		return nil, err
	}
	return &resp, nil
}

// MoveBatch 批量移动。
func (s *Service) MoveBatch(ctx context.Context, fileIDs []string, toParentFileID, driveID string) ([]BatchItem, error) {
	bodies := make([]map[string]any, len(fileIDs))
	for i, fid := range fileIDs {
		bodies[i] = map[string]any{
			"drive_id":          driveID,
			"file_id":           fid,
			"to_parent_file_id": defaultStr(toParentFileID, "root"),
			"auto_rename":       true,
		}
	}
	return s.batch(ctx, subURLFileMove, "file", fileIDs, bodies, false)
}

// Trash 移到回收站。期望 202/204。
func (s *Service) Trash(ctx context.Context, fileID, driveID string) error {
	body := map[string]any{"file_id": fileID, "drive_id": driveID}
	return invoker.PostAndDecode(ctx, s.inv, pathRecyclebinTrash, body, nil, []int{202, 204})
}

// Restore 从回收站恢复。期望 204。
func (s *Service) Restore(ctx context.Context, fileID, driveID string) error {
	body := map[string]any{"file_id": fileID, "drive_id": driveID}
	return invoker.PostAndDecode(ctx, s.inv, pathRecyclebinRestore, body, nil, []int{204})
}

// TrashBatch 批量移到回收站。
func (s *Service) TrashBatch(ctx context.Context, fileIDs []string, driveID string) ([]BatchItem, error) {
	bodies := make([]map[string]any, len(fileIDs))
	for i, fid := range fileIDs {
		bodies[i] = map[string]any{"drive_id": driveID, "file_id": fid}
	}
	return s.batch(ctx, subURLRecyclebinTrash, "file", fileIDs, bodies, false)
}

// RestoreBatch 批量恢复。
func (s *Service) RestoreBatch(ctx context.Context, fileIDs []string, driveID string) ([]BatchItem, error) {
	bodies := make([]map[string]any, len(fileIDs))
	for i, fid := range fileIDs {
		bodies[i] = map[string]any{"drive_id": driveID, "file_id": fid}
	}
	return s.batch(ctx, subURLRecyclebinRestore, "file", fileIDs, bodies, false)
}

// ListRecycleBin 列出回收站文件，自动分页。
func (s *Service) ListRecycleBin(ctx context.Context, driveID string) ([]*types.BaseFile, error) {
	body := map[string]any{
		"drive_id":                driveID,
		"limit":                   200,
		"order_direction":         "ASC",
		"order_by":                "name",
		"url_expire_sec":          14400,
		"image_thumbnail_process": "image/resize,w_400/format,jpeg",
		"image_url_process":       "image/resize,w_1920/format,jpeg",
		"video_thumbnail_process": "video/snapshot,t_0,f_jpg,ar_auto,w_800",
	}
	var all []*types.BaseFile
	for {
		var resp fileListResponse
		if err := invoker.PostAndDecode(ctx, s.inv, pathRecyclebinList, body, &resp, []int{200}); err != nil {
			return nil, err
		}
		all = append(all, resp.Items...)
		if resp.NextMarker == "" {
			break
		}
		body["marker"] = resp.NextMarker
	}
	return all, nil
}

// UpdateRequest 更新文件属性请求。
type UpdateRequest struct {
	FileID        string             `json:"file_id"`
	DriveID       string             `json:"drive_id,omitempty"`
	Name          string             `json:"name,omitempty"`
	CheckNameMode types.CheckNameMode `json:"check_name_mode,omitempty"`
	Starred       *bool              `json:"starred,omitempty"`
	Description   string             `json:"description,omitempty"`
	Hidden        *bool              `json:"hidden,omitempty"`
}

// Rename 重命名。
func (s *Service) Rename(ctx context.Context, fileID, newName, driveID string) (*types.BaseFile, error) {
	if newName == "" {
		return nil, invoker.NewAPIError(0, "InvalidArgument", "new name is required")
	}
	body := map[string]any{
		"file_id":         fileID,
		"drive_id":        driveID,
		"name":            newName,
		"check_name_mode": "refuse",
	}
	var f types.BaseFile
	if err := invoker.PostAndDecode(ctx, s.inv, pathFileUpdate, body, &f, []int{200}); err != nil {
		return nil, err
	}
	return &f, nil
}

// Update 更新文件属性。
func (s *Service) Update(ctx context.Context, req *UpdateRequest) (*types.BaseFile, error) {
	if req == nil || req.FileID == "" {
		return nil, fmt.Errorf("alipan: file_id is required")
	}
	body := map[string]any{
		"file_id":         req.FileID,
		"drive_id":        req.DriveID,
		"check_name_mode": defaultStr(string(req.CheckNameMode), "refuse"),
	}
	if req.Name != "" {
		body["name"] = req.Name
	}
	if req.Starred != nil {
		body["starred"] = *req.Starred
	}
	if req.Description != "" {
		body["description"] = req.Description
	}
	if req.Hidden != nil {
		body["hidden"] = *req.Hidden
	}
	var f types.BaseFile
	if err := invoker.PostAndDecode(ctx, s.inv, pathFileUpdate, body, &f, []int{200}); err != nil {
		return nil, err
	}
	return &f, nil
}

func boolPtr(b bool) *bool { return &b }

// Star 收藏文件。
func (s *Service) Star(ctx context.Context, fileID, driveID string) error {
	_, err := s.Update(ctx, &UpdateRequest{FileID: fileID, DriveID: driveID, Starred: boolPtr(true)})
	return err
}

// Unstar 取消收藏。
func (s *Service) Unstar(ctx context.Context, fileID, driveID string) error {
	_, err := s.Update(ctx, &UpdateRequest{FileID: fileID, DriveID: driveID, Starred: boolPtr(false)})
	return err
}
