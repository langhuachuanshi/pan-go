package file

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/langhuachuanshi/alipan-go/alipan/invoker"
	"github.com/langhuachuanshi/alipan-go/alipan/types"
)

// UploadRequest 上传请求。
type UploadRequest struct {
	FilePath      string
	ParentFileID  string
	DriveID       string
	Name          string
	CheckNameMode types.CheckNameMode
	OnProgress    func(sent, total int64)
}

type createFileResp struct {
	FileID       string                 `json:"file_id"`
	UploadID     string                 `json:"upload_id"`
	DriveID      string                 `json:"drive_id"`
	ParentFileID string                 `json:"parent_file_id"`
	DomainID     string                 `json:"domain_id"`
	PartInfoList []types.UploadPartInfo `json:"part_info_list"`
	RapidUpload  bool                   `json:"rapid_upload"`
	Exist        bool                   `json:"exist"`
	Status       string                 `json:"status"`
	Code         string                 `json:"code"`
	Message      string                 `json:"message"`
	PreHash      string                 `json:"pre_hash"`
	RevisionID   string                 `json:"revision_id"`
	Location     string                 `json:"location"`
}

// Upload 上传文件（含秒传、分片、断点续传）。
//
// 断点续传：上传中断后，再次调用 Upload 会自动从断点继续，不重头上传。
// 进度按文件 SHA1 持久化到 ~/.aligo/.upload-<sha1>.json。
// 文件内容变化（sha1 变）则视为新文件，从头上传。
func (s *Service) Upload(ctx context.Context, req *UploadRequest) (*types.BaseFile, error) {
	if req == nil || req.FilePath == "" {
		return nil, invoker.NewAPIError(0, "InvalidArgument", "file_path is required")
	}
	fi, err := os.Stat(req.FilePath)
	if err != nil {
		return nil, fmt.Errorf("alipan: stat file failed: %w", err)
	}
	fileSize := fi.Size()
	name := req.Name
	if name == "" {
		name = filepath.Base(req.FilePath)
	}
	parentID := defaultStr(req.ParentFileID, "root")
	driveID := req.DriveID

	contentHash, preHash, err := computeSHA1Upper(req.FilePath)
	if err != nil {
		return nil, fmt.Errorf("alipan: compute sha1 failed: %w", err)
	}

	// —— 断点续传：检查是否有未完成的上传记录 ——
	if prog := loadProgress(contentHash); prog != nil && prog.FileSize == fileSize && prog.FilePath == req.FilePath {
		// 有匹配的进度记录，尝试续传。
		f, err := s.resumeUpload(ctx, prog, req)
		if err == nil {
			return f, nil // 续传成功
		}
		// 续传失败（upload_id 失效等），删除记录，走全新上传。
		removeProgress(contentHash)
	}

	// —— 全新上传（含秒传判断）——
	proofCode := ""
	if fileSize > 1024 {
		proofCode, err = computeProofCode(s.inv.AccessToken(), req.FilePath, fileSize)
		if err != nil {
			return nil, fmt.Errorf("alipan: compute proof_code failed: %w", err)
		}
	}
	createResp, err := s.createFileWithRapid(ctx, name, parentID, driveID, req.CheckNameMode,
		fileSize, contentHash, preHash, proofCode)
	if err != nil {
		return nil, err
	}
	if createResp.RapidUpload || createResp.Exist {
		removeProgress(contentHash)
		return s.Get(ctx, createResp.FileID, driveID)
	}

	// 建立进度记录。
	prog := &uploadProgress{
		FilePath: req.FilePath, FileSize: fileSize, SHA1: contentHash,
		DriveID: createResp.DriveID, FileID: createResp.FileID, UploadID: createResp.UploadID,
		ParentFileID: parentID, Name: name, ChunkSize: chunkSize(fileSize),
	}
	saveProgress(prog)

	if err := s.uploadPartsResume(ctx, req, createResp.PartInfoList, prog, fileSize); err != nil {
		return nil, err
	}
	f, err := s.completeUpload(ctx, createResp.FileID, createResp.UploadID, driveID, createResp.PartInfoList)
	if err != nil {
		return nil, err
	}
	removeProgress(contentHash) // 上传完成，清理进度
	return f, nil
}

// resumeUpload 从进度记录续传：用记录的 upload_id 重新拿分片 URL，跳过已传分片。
func (s *Service) resumeUpload(ctx context.Context, prog *uploadProgress, req *UploadRequest) (*types.BaseFile, error) {
	// 计算全部分片号。
	partCount := int((prog.FileSize + prog.ChunkSize - 1) / prog.ChunkSize)
	allParts := make([]types.UploadPartInfo, partCount)
	for i := 0; i < partCount; i++ {
		allParts[i] = types.UploadPartInfo{PartNumber: i + 1}
	}
	// 用记录的 upload_id 重新获取各分片 URL。
	body := map[string]any{
		"drive_id":       prog.DriveID,
		"file_id":        prog.FileID,
		"upload_id":      prog.UploadID,
		"part_info_list": allParts,
	}
	var resp struct {
		PartInfoList []types.UploadPartInfo `json:"part_info_list"`
	}
	if err := invoker.PostAndDecode(ctx, s.inv, pathFileGetUploadURL, body, &resp, []int{200}); err != nil {
		return nil, err
	}
	if len(resp.PartInfoList) == 0 {
		return nil, fmt.Errorf("alipan: resume get_upload_url returned empty")
	}
	// 调用统一的分片上传（会跳过已传的）。
	if err := s.uploadPartsResume(ctx, req, resp.PartInfoList, prog, prog.FileSize); err != nil {
		return nil, err
	}
	f, err := s.completeUpload(ctx, prog.FileID, prog.UploadID, prog.DriveID, resp.PartInfoList)
	if err != nil {
		return nil, err
	}
	removeProgress(prog.SHA1)
	return f, nil
}

func (s *Service) createFileWithRapid(ctx context.Context, name, parentID, driveID string,
	mode types.CheckNameMode, fileSize int64, contentHash, preHash, proofCode string) (*createFileResp, error) {

	body := map[string]any{
		"name":              name,
		"type":              "file",
		"parent_file_id":    parentID,
		"drive_id":          driveID,
		"check_name_mode":   defaultStr(string(mode), "auto_rename"),
		"size":              fileSize,
		"content_hash_name": "sha1",
		"proof_version":     "v1",
	}
	if preHash != "" {
		body["pre_hash"] = preHash
	}
	csize := chunkSize(fileSize)
	partCount := int((fileSize + csize - 1) / csize)
	parts := make([]types.UploadPartInfo, partCount)
	for i := 0; i < partCount; i++ {
		parts[i] = types.UploadPartInfo{PartNumber: i + 1}
	}
	body["part_info_list"] = parts

	data, status, err := s.inv.Post(ctx, pathFileCreateWithFolders, body, nil, []int{201, 409})
	if err != nil {
		return nil, err
	}
	var resp createFileResp
	if status == 409 || (status == 201 && bytes.Contains(data, []byte(`"PreHashMatched"`))) {
		if err := json.Unmarshal(data, &resp); err == nil && resp.Code == "PreHashMatched" {
			body["content_hash"] = contentHash
			body["proof_code"] = proofCode
			delete(body, "pre_hash")
			data, status, err = s.inv.Post(ctx, pathFileCreateWithFolders, body, nil, []int{201})
			if err != nil {
				return nil, err
			}
		}
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("alipan: decode create response failed: %w", err)
	}
	return &resp, nil
}

// uploadPartsResume 上传分片，支持断点续传：跳过 prog.DoneParts 中已记录的分片，
// 每传完一片立即更新进度记录。
func (s *Service) uploadPartsResume(ctx context.Context, req *UploadRequest, parts []types.UploadPartInfo, prog *uploadProgress, fileSize int64) error {
	f, err := os.Open(req.FilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	csize := prog.ChunkSize
	if csize == 0 {
		csize = chunkSize(fileSize)
	}
	// 已传字节数（用于进度回调）。
	doneBytes := int64(len(prog.DoneParts)) * csize
	if doneBytes > fileSize {
		doneBytes = fileSize
	}
	if req.OnProgress != nil {
		req.OnProgress(doneBytes, fileSize)
	}
	for i, part := range parts {
		if part.UploadURL == "" {
			continue
		}
		// 跳过已传分片（断点续传核心）。
		if containsPart(prog.DoneParts, part.PartNumber) {
			continue
		}
		partSize := csize
		remaining := fileSize - int64(i)*csize
		if remaining < partSize {
			partSize = remaining
		}
		// 定位到该分片偏移读取（不能顺序读，因为可能跳过了前面的）。
		if _, err := f.Seek(int64(i)*csize, io.SeekStart); err != nil {
			return fmt.Errorf("alipan: seek part %d failed: %w", part.PartNumber, err)
		}
		buf := make([]byte, partSize)
		if _, err := io.ReadFull(f, buf); err != nil {
			return fmt.Errorf("alipan: read part %d failed: %w", part.PartNumber, err)
		}
		uploadURL := part.UploadURL
		if err := s.putPart(ctx, uploadURL, buf); err != nil {
			if is403(err) {
				newURL, rerr := s.renewUploadURL(ctx, prog.DriveID, prog.FileID, prog.UploadID, part.PartNumber)
				if rerr != nil {
					return rerr
				}
				if err := s.putPart(ctx, newURL, buf); err != nil {
					return fmt.Errorf("alipan: upload part %d failed after renew: %w", part.PartNumber, err)
				}
			} else {
				return fmt.Errorf("alipan: upload part %d failed: %w", part.PartNumber, err)
			}
		}
		// 该分片传完，记录进度（断点续传的关键持久化）。
		prog.DoneParts = append(prog.DoneParts, part.PartNumber)
		saveProgress(prog)
		doneBytes += int64(len(buf))
		if req.OnProgress != nil {
			req.OnProgress(doneBytes, fileSize)
		}
	}
	return nil
}

func (s *Service) putPart(ctx context.Context, uploadURL string, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	setCommonHeaders(req)
	hc := &http.Client{Timeout: 10 * time.Minute}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return invoker.ParseAPIError(resp.StatusCode, body)
}

func (s *Service) renewUploadURL(ctx context.Context, driveID, fileID, uploadID string, partNumber int) (string, error) {
	body := map[string]any{
		"drive_id":  driveID,
		"file_id":   fileID,
		"upload_id": uploadID,
		"part_info_list": []types.UploadPartInfo{{PartNumber: partNumber}},
	}
	var resp struct {
		PartInfoList []types.UploadPartInfo `json:"part_info_list"`
	}
	if err := invoker.PostAndDecode(ctx, s.inv, pathFileGetUploadURL, body, &resp, []int{200}); err != nil {
		return "", err
	}
	if len(resp.PartInfoList) == 0 {
		return "", fmt.Errorf("alipan: renew upload url returned empty part_info_list")
	}
	return resp.PartInfoList[0].UploadURL, nil
}

func (s *Service) completeUpload(ctx context.Context, fileID, uploadID, driveID string, parts []types.UploadPartInfo) (*types.BaseFile, error) {
	body := map[string]any{
		"file_id":   fileID,
		"upload_id": uploadID,
		"drive_id":  driveID,
	}
	if parts != nil {
		body["part_info_list"] = parts
	}
	var f types.BaseFile
	if err := invoker.PostAndDecode(ctx, s.inv, pathFileComplete, body, &f, []int{200}); err != nil {
		return nil, err
	}
	return &f, nil
}

func is403(err error) bool {
	if e, ok := err.(*invoker.APIError); ok {
		return e.StatusCode == http.StatusForbidden
	}
	return false
}
