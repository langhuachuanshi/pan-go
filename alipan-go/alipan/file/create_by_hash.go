package file

import (
	"context"

	"github.com/langhuachuanshi/alipan-go/alipan/invoker"
	"github.com/langhuachuanshi/alipan-go/alipan/types"
)

// 本文件实现按 hash 秒传创建文件，对应该 aligo 的 create_by_hash。
// 用于 CustomShare 的 save_files_by_aligo：拿到 content_hash+size+name 后直接秒传，无需上传。

// CreateByHashRequest 按 hash 秒传请求。
type CreateByHashRequest struct {
	Name          string
	ParentFileID  string
	DriveID       string
	Size          int64
	ContentHash   string // SHA1 大写
	ProofCode     string // 可选
	CheckNameMode types.CheckNameMode
}

// CreateByHash 用 content_hash 秒传一个文件到指定目录。
// 若服务端已存在相同 hash 的文件，会瞬间完成；否则返回错误（秒传失败）。
//
// POST /adrive/v2/file/createWithFolders，期望 201。
func (s *Service) CreateByHash(ctx context.Context, req *CreateByHashRequest) (*types.BaseFile, error) {
	if req == nil || req.Name == "" || req.ContentHash == "" {
		return nil, invoker.NewAPIError(0, "InvalidArgument", "name and content_hash are required")
	}
	body := map[string]any{
		"name":              req.Name,
		"type":              "file",
		"parent_file_id":    defaultStr(req.ParentFileID, "root"),
		"drive_id":          req.DriveID,
		"check_name_mode":   defaultStr(string(req.CheckNameMode), "auto_rename"),
		"size":              req.Size,
		"content_hash":      req.ContentHash,
		"content_hash_name": "sha1",
		"proof_version":     "v1",
	}
	if req.ProofCode != "" {
		body["proof_code"] = req.ProofCode
	}
	var resp struct {
		FileID       string `json:"file_id"`
		DriveID      string `json:"drive_id"`
		RapidUpload  bool   `json:"rapid_upload"`
		Exist        bool   `json:"exist"`
		Status       string `json:"status"`
	}
	if err := invoker.PostAndDecode(ctx, s.inv, pathFileCreateWithFolders, body, &resp, []int{201}); err != nil {
		return nil, err
	}
	// 秒传成功后获取完整文件对象。
	return s.Get(ctx, resp.FileID, req.DriveID)
}
