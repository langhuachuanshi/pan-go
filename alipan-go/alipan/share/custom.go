package share

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/langhuachuanshi/alipan-go/alipan/file"
	"github.com/langhuachuanshi/alipan-go/alipan/invoker"
	"github.com/langhuachuanshi/alipan-go/alipan/types"
)

// 本文件实现 CustomShare（aligo 的自定义分享），对应该 aligo 的 apis/CustomShare.py。
//
// 这不是官方分享 API，而是 aligo 自创的 aligo:// 协议：
//   - 分享：把文件元信息（name/content_hash/size/download_url）序列化成 JSON，
//     base64 编码，加前缀 aligo://，得到一串可传播的"分享码"。
//   - 保存：反向解码，对每个文件用 CreateByHash 秒传；对文件夹递归 CreateFolder。
//
// 优点：不依赖官方分享接口、无数量限制。
// 缺点：要求接收方网盘能秒传（即阿里云盘服务端已有该 hash）；大文件下载链接 4 小时过期。

// customScheme 是 aligo 自定义分享的前缀。
const customScheme = "aligo://"

// CustomFile 单个自定义分享文件的元信息。
type CustomFile struct {
	Name        string `json:"name"`
	ContentHash string `json:"content_hash"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"download_url"`
}

// CustomSharer 提供 aligo:// 自定义分享的编解码。
type CustomSharer struct {
	files *file.Service
}

// NewCustomSharer 创建 CustomSharer。files 用于保存时调用秒传/建目录。
func NewCustomSharer(files *file.Service) *CustomSharer { return &CustomSharer{files: files} }

// ShareFile 把单个文件编码成 aligo:// 分享码。
// 如果 f 没有 download_url，会先调用 GetDownloadURL 获取（需 file_id+drive_id）。
func (cs *CustomSharer) ShareFile(ctx context.Context, f *types.BaseFile, driveID string) (string, error) {
	if f == nil {
		return "", invoker.NewAPIError(0, "InvalidArgument", "file is nil")
	}
	dlURL := f.DownloadURL
	if dlURL == "" && f.FileID != "" {
		var err error
		dlURL, err = cs.files.GetDownloadURL(ctx, f.FileID, defaultStr(driveID, f.DriveID))
		if err != nil {
			return "", err
		}
	}
	cf := CustomFile{
		Name:        f.Name,
		ContentHash: f.ContentHash,
		Size:        f.Size,
		DownloadURL: dlURL,
	}
	return encodeCustom(cf)
}

// ShareFiles 把多个文件编码成一个分享码（JSON 数组）。
func (cs *CustomSharer) ShareFiles(ctx context.Context, fs []*types.BaseFile, driveID string) (string, error) {
	items := make([]CustomFile, 0, len(fs))
	for _, f := range fs {
		dlURL := f.DownloadURL
		if dlURL == "" && f.FileID != "" {
			u, err := cs.files.GetDownloadURL(ctx, f.FileID, defaultStr(driveID, f.DriveID))
			if err != nil {
				return "", err
			}
			dlURL = u
		}
		items = append(items, CustomFile{
			Name: f.Name, ContentHash: f.ContentHash, Size: f.Size, DownloadURL: dlURL,
		})
	}
	data, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return customScheme + base64.StdEncoding.EncodeToString(data), nil
}

// ShareFolder 把整个文件夹递归编码成分享码。
// 结构：[[root, [[子文件夹名, [...]], 文件...]]]
func (cs *CustomSharer) ShareFolder(ctx context.Context, rootFileID, driveID string) (string, error) {
	tree, err := cs.buildFolderTree(ctx, rootFileID, driveID)
	if err != nil {
		return "", err
	}
	// 包装成 [[root, tree]]
	wrapped := []any{[]any{"root", tree}}
	data, err := json.Marshal(wrapped)
	if err != nil {
		return "", err
	}
	return customScheme + base64.StdEncoding.EncodeToString(data), nil
}

// buildFolderTree 递归构建文件夹树（嵌套 [name, children]）。
func (cs *CustomSharer) buildFolderTree(ctx context.Context, parentID, driveID string) ([]any, error) {
	files, err := cs.files.List(ctx, &file.ListRequest{ParentFileID: parentID, DriveID: driveID})
	if err != nil {
		return nil, err
	}
	var result []any
	for _, f := range files {
		if f.IsFolder() {
			children, err := cs.buildFolderTree(ctx, f.FileID, driveID)
			if err != nil {
				return nil, err
			}
			result = append(result, []any{f.Name, children})
		} else if f.IsFile() {
			dlURL := f.DownloadURL
			if dlURL == "" {
				u, err := cs.files.GetDownloadURL(ctx, f.FileID, driveID)
				if err == nil {
					dlURL = u
				}
			}
			result = append(result, CustomFile{
				Name: f.Name, ContentHash: f.ContentHash, Size: f.Size, DownloadURL: dlURL,
			})
		}
	}
	return result, nil
}

// SaveFromCode 解码 aligo:// 分享码并保存到指定目录。
//
// 单文件码 → 秒传到 toParentFileID。
// 文件夹码 → 递归 CreateFolder + 每个文件 CreateByHash。
func (cs *CustomSharer) SaveFromCode(ctx context.Context, code, toParentFileID, driveID string) error {
	if !strings.HasPrefix(code, customScheme) {
		return invoker.NewAPIError(0, "InvalidCustomShare", "missing aligo:// prefix")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(code, customScheme))
	if err != nil {
		return fmt.Errorf("alipan: decode custom share base64 failed: %w", err)
	}

	// 尝试解析为文件夹结构 [[root, tree]]。
	var asWrapped []json.RawMessage
	if err := json.Unmarshal(raw, &asWrapped); err == nil && len(asWrapped) == 2 {
		var first string
		if json.Unmarshal(asWrapped[0], &first) == nil {
			// 文件夹结构。
			return cs.saveFolderTree(ctx, asWrapped[1], defaultStr(toParentFileID, "root"), driveID)
		}
	}

	// 尝试解析为单个文件（dict）。
	var single CustomFile
	if err := json.Unmarshal(raw, &single); err == nil && single.Name != "" {
		return cs.saveOneFile(ctx, single, defaultStr(toParentFileID, "root"), driveID)
	}

	// 尝试解析为文件数组。
	var arr []CustomFile
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, cf := range arr {
			if err := cs.saveOneFile(ctx, cf, defaultStr(toParentFileID, "root"), driveID); err != nil {
				return err
			}
		}
		return nil
	}
	return invoker.NewAPIError(0, "InvalidCustomShare", "cannot parse custom share code")
}

// saveFolderTree 递归保存文件夹树。
func (cs *CustomSharer) saveFolderTree(ctx context.Context, treeRaw json.RawMessage, parentID, driveID string) error {
	var items []json.RawMessage
	if err := json.Unmarshal(treeRaw, &items); err != nil {
		return err
	}
	for _, item := range items {
		// 可能是 [name, children] 或 CustomFile(dict)。
		var pair []json.RawMessage
		if json.Unmarshal(item, &pair) == nil && len(pair) == 2 {
			var name string
			if json.Unmarshal(pair[0], &name) != nil {
				continue
			}
			created, err := cs.files.CreateFolder(ctx, &file.CreateFolderRequest{
				Name: name, ParentFileID: parentID, DriveID: driveID,
			})
			if err != nil {
				return err
			}
			if err := cs.saveFolderTree(ctx, pair[1], created.FileID, driveID); err != nil {
				return err
			}
			continue
		}
		var cf CustomFile
		if json.Unmarshal(item, &cf) == nil && cf.Name != "" {
			if err := cs.saveOneFile(ctx, cf, parentID, driveID); err != nil {
				return err
			}
		}
	}
	return nil
}

// saveOneFile 用秒传保存单个文件。
func (cs *CustomSharer) saveOneFile(ctx context.Context, cf CustomFile, parentID, driveID string) error {
	if cf.ContentHash == "" {
		// 没有 hash 无法秒传。
		return invoker.NewAPIError(0, "RapidUploadUnavailable", fmt.Sprintf("file %q has no content_hash", cf.Name))
	}
	_, err := cs.files.CreateByHash(ctx, &file.CreateByHashRequest{
		Name:        cf.Name,
		ParentFileID: parentID,
		DriveID:     driveID,
		Size:        cf.Size,
		ContentHash: cf.ContentHash,
	})
	return err
}

// encodeCustom 编码单个文件为分享码。
func encodeCustom(cf CustomFile) (string, error) {
	data, err := json.Marshal(cf)
	if err != nil {
		return "", err
	}
	return customScheme + base64.StdEncoding.EncodeToString(data), nil
}
