// Package upload 实现百度网盘文件上传（网页端 / BDUSS 方案）。
//
// 上传三阶段：
//  1. precreate（预上传）：POST pan.baidu.com/api/precreate
//     传 path/size/block_list(分片md5数组)，拿 uploadid
//     - return_type=2 表示秒传成功，直接跳到第3步
//     - return_type=1 需要上传分片，返回 block_list 是待上传分片序号
//  2. superfile2（分片上传）：每片 POST 到 pcs.baidu.com/rest/2.0/pcs/superfile2，拿该片 md5
//  3. create（创建文件）：POST pan.baidu.com/api/create，传 uploadid + block_list 落库
//
// 鉴权靠 BDUSS cookie + app_id（不是 access_token）。分片固定 4MB，block_list 是 hex 小写 md5 数组。
package upload

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/langhuachuanshi/panbaidu-go/baidu/invoker"
	"github.com/langhuachuanshi/panbaidu-go/baidu/types"
)

// 分片大小（百度固定 4MB）。
const partSize = 4 * 1024 * 1024

// 分片上传域名（superfile2，走 PCS）。
const pcsUploadBase = "https://d.pcs.baidu.com/rest/2.0/pcs/superfile2"

// Service 上传入口。
type Service struct {
	inv invoker.Invoker
}

// New 创建 upload Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// UploadRequest 上传请求。
type UploadRequest struct {
	ReaderAt io.ReaderAt // 必填。可定位读取源
	FileName string      // 必填。上传后的文件名
	Size     int64       // 必填。文件字节数
	DestPath string      // 必填。目标目录绝对路径（如 /apps/），文件会落到 DestPath/FileName
}

// Upload 上传文件，返回新文件信息。
// 支持秒传（return_type=2 时跳过分片）。
func (s *Service) Upload(ctx context.Context, req *UploadRequest) (*types.File, error) {
	if req == nil || req.ReaderAt == nil {
		return nil, invoker.NewAPIError(0, "ReaderAt is required")
	}
	if req.FileName == "" {
		return nil, invoker.NewAPIError(0, "FileName is required")
	}
	if req.Size < 0 {
		return nil, invoker.NewAPIError(0, "invalid Size")
	}
	if req.DestPath == "" {
		return nil, invoker.NewAPIError(0, "DestPath is required")
	}
	filePath := joinPath(req.DestPath, req.FileName)

	// 1. 计算各分片 md5（block_list）。
	blockMD5s, err := computeBlockMD5s(req.ReaderAt, req.Size)
	if err != nil {
		return nil, err
	}

	// 2. precreate。
	preBody := map[string]string{
		"path":       filePath,
		"size":       strconv.FormatInt(req.Size, 10),
		"isdir":      "0",
		"autoinit":   "1",
		"block_list": toJSONStr(blockMD5s),
		"rtype":      "3", // 重命名策略：3=遇到冲突自动改名
	}
	var pre struct {
		Errno      int    `json:"errno"`
		UploadID   string `json:"uploadid"`
		ReturnType int    `json:"return_type"` // 1=需上传 2=秒传
		BlockList  []int  `json:"block_list"`  // 待上传分片序号（return_type=1 时）
	}
	if err := invoker.PostFormAndDecode(ctx, s.inv, "/api/precreate", preBody, nil, &pre); err != nil {
		return nil, err
	}
	if pre.Errno != 0 {
		return nil, invoker.NewAPIError(pre.Errno, errnoMsg(pre.Errno))
	}

	uploadID := pre.UploadID

	// 3. 分片上传（仅 return_type=1）。
	if pre.ReturnType == 1 {
		if uploadID == "" {
			return nil, invoker.NewAPIError(pre.Errno, "precreate 返回 return_type=1 但无 uploadid")
		}
		uploadedMD5s, err := s.uploadParts(ctx, req.ReaderAt, req.Size, filePath, uploadID, pre.BlockList)
		if err != nil {
			return nil, err
		}
		// 用实际上传返回的 md5 替换（一般和本地算的一致）。
		blockMD5s = uploadedMD5s
	}

	// 4. create。
	createBody := map[string]string{
		"path":       filePath,
		"size":       strconv.FormatInt(req.Size, 10),
		"isdir":      "0",
		"uploadid":   uploadID,
		"block_list": toJSONStr(blockMD5s),
		"rtype":      "3",
	}
	var cre struct {
		Errno int        `json:"errno"`
		Data  types.File `json:"data"`
	}
	if err := invoker.PostFormAndDecode(ctx, s.inv, "/api/create", createBody, nil, &cre); err != nil {
		return nil, err
	}
	if cre.Errno != 0 {
		return nil, invoker.NewAPIError(cre.Errno, errnoMsg(cre.Errno))
	}
	f := cre.Data
	return &f, nil
}

// uploadParts 上传指定分片序号，返回每片的 md5（全部分片，按序号对齐）。
func (s *Service) uploadParts(ctx context.Context, r io.ReaderAt, size int64, path, uploadID string, partSeqs []int) ([]string, error) {
	// 待上传序号集合。
	need := make(map[int]bool, len(partSeqs))
	for _, seq := range partSeqs {
		need[seq] = true
	}
	totalParts := int(size / partSize)
	if size%partSize > 0 {
		totalParts++
	}
	md5s := make([]string, totalParts)
	buf := make([]byte, partSize)
	for seq := 0; seq < totalParts; seq++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !need[seq] {
			// 服务端已有该片（秒传分片），用本地算的 md5。
			n, _ := r.ReadAt(buf, int64(seq)*partSize)
			md5s[seq] = md5Hex(buf[:n])
			continue
		}
		off := int64(seq) * partSize
		end := off + partSize
		if end > size {
			end = size
		}
		n, err := r.ReadAt(buf[:end-off], off)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("读取分片 %d 失败: %w", seq, err)
		}
		params := map[string]string{
			"method":   "upload",
			"type":     "tmpfile",
			"path":     path,
			"uploadid": uploadID,
			"partseq":  strconv.Itoa(seq),
		}
		var resp struct {
			Errno int    `json:"errno"`
			MD5   string `json:"md5"`
		}
		if err := invoker.PostMultipartAndDecode(ctx, s.inv, pcsUploadBase, "", params, "file", "file", buf[:n], &resp); err != nil {
			return nil, err
		}
		if resp.Errno != 0 {
			return nil, invoker.NewAPIError(resp.Errno, fmt.Sprintf("分片 %d 上传失败: %s", seq, errnoMsg(resp.Errno)))
		}
		md5s[seq] = resp.MD5
	}
	return md5s, nil
}

// computeBlockMD5s 计算各分片的 hex md5（全文件，4MB 一片）。
func computeBlockMD5s(r io.ReaderAt, size int64) ([]string, error) {
	totalParts := int(size / partSize)
	if size%partSize > 0 {
		totalParts++
	}
	if totalParts == 0 {
		totalParts = 1 // 空文件至少 1 片（注：百度开放平台不支持空文件上传，但这里给兜底）
	}
	md5s := make([]string, 0, totalParts)
	buf := make([]byte, partSize)
	for i := 0; i < totalParts; i++ {
		off := int64(i) * partSize
		end := off + partSize
		if end > size {
			end = size
		}
		n, err := r.ReadAt(buf[:end-off], off)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("计算分片 md5 失败: %w", err)
		}
		md5s = append(md5s, md5Hex(buf[:n]))
	}
	return md5s, nil
}

func md5Hex(b []byte) string {
	h := md5.Sum(b)
	return hex.EncodeToString(h[:])
}

// toJSONStr 序列化为 JSON 字符串（block_list 用）。
func toJSONStr(arr []string) string {
	b, _ := json.Marshal(arr)
	return string(b)
}

// joinPath 拼接目录和文件名，确保以 / 开头。
func joinPath(dir, name string) string {
	if dir == "" {
		dir = "/"
	}
	if dir[len(dir)-1] != '/' {
		dir += "/"
	}
	p := dir + name
	if p[0] != '/' {
		p = "/" + p
	}
	return p
}

// errnoMsg 常见 errno 描述。
func errnoMsg(errno int) string {
	switch errno {
	case -6:
		return "BDUSS 失效或权限不足（请重新登录网页端获取新 BDUSS）"
	case -7:
		return "路径不存在或无权访问"
	case -8:
		return "路径已存在"
	case 31034:
		return "接口请求错误（参数有误）"
	case 31211:
		return "分片上传格式错误（可能是 chunked 导致）"
	default:
		return "请求失败"
	}
}
