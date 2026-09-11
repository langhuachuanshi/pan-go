// Package upload 实现夸克网盘文件上传。
//
// 夸克上传流程（跨 AList/Rust/Python 三份实现核对一致）：
//  1. 本地算全文件 md5 + sha1（流式，零额外内存）
//  2. POST /file/upload/pre  → 拿 task_id/obj_key/bucket/upload_url/auth_info/callback/upload_id + 分片大小
//  3. POST /file/update/hash → 秒传判断：finish=true 直接完成，不上传字节
//  4. 分片循环（partNumber 从 1）：换 auth → OSS PUT 分片 → 收集 ETag
//  5. commit：构造 XML → 换 auth → OSS POST 合并
//  6. POST /file/upload/finish → 完成
//
// 其中步骤 4/5 的 OSS 直传不走夸克 API（不同域名/鉴权/body 类型），见 oss.go。
package upload

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"time"

	"github.com/langhuachuanshi/quark-go/quark/invoker"
	"github.com/langhuachuanshi/quark-go/quark/types"
)

// Service 上传操作入口。
type Service struct {
	inv invoker.Invoker
}

// New 创建 upload Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// UploadRequest 上传请求。
type UploadRequest struct {
	ReaderAt io.ReaderAt // 必填。可定位读取源（os.File/bytes.Reader 天然满足）
	FileName string      // 必填。上传后的文件名
	Size     int64       // 必填。文件字节数
	PDirFID  string      // 父目录 fid，根目录留空或用 "0"
	MIME     string      // 文件类型，空则默认 application/octet-stream
	OnProgress func(uploaded, total int64) // 可选进度回调（0~total）
}

// Upload 上传文件，秒传或正常上传完成后返回新文件的简要信息。
//
// 流式处理：用 ReaderAt 边算哈希边读，不把整个文件读进内存（超大文件安全）。
// 分片大小取自服务端（pre 响应的 metadata.part_size），不本地硬编码。
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
	pdir := req.PDirFID
	if pdir == "" {
		pdir = "0"
	}
	mime := req.MIME
	if mime == "" {
		mime = "application/octet-stream"
	}
	total := req.Size
	progress := req.OnProgress
	if progress == nil {
		progress = func(int64, int64) {}
	}

	// —— 步骤1：流式算 md5 + sha1 ——
	md5Hex, sha1Hex, err := hashFile(req.ReaderAt, total)
	if err != nil {
		return nil, err
	}

	// —— 步骤2：POST /file/upload/pre ——
	now := time.Now().UnixMilli()
	preBody := map[string]any{
		"ccp_hash_update": true,
		"dir_name":        "",
		"file_name":       req.FileName,
		"format_type":     mime,
		"l_created_at":    now,
		"l_updated_at":    now,
		"pdir_fid":        pdir,
		"size":            total,
	}
	var pre upPreResp
	if err := invoker.PostAndDecode(ctx, s.inv, "/file/upload/pre", preBody, nil, &pre); err != nil {
		return nil, err
	}
	if pre.Code != 0 {
		return nil, invoker.NewAPIError(pre.Code, pre.Msg)
	}
	preData := &pre.Data

	// —— 步骤3：POST /file/update/hash 秒传判断 ——
	hashBody := map[string]any{
		"md5":     md5Hex,
		"sha1":    sha1Hex,
		"task_id": preData.TaskID,
	}
	var hashR hashResp
	if err := invoker.PostAndDecode(ctx, s.inv, "/file/update/hash", hashBody, nil, &hashR); err != nil {
		return nil, err
	}
	if hashR.Code != 0 {
		return nil, invoker.NewAPIError(hashR.Code, hashR.Msg)
	}
	if hashR.Data.Finish {
		// 秒传命中：文件已存在服务端，无需上传字节。
		progress(total, total)
		return s.finishUpload(ctx, preData)
	}

	// —— 步骤4：分片循环 OSS 直传 ——
	partSize := int64(pre.Metadata.PartSize)
	if partSize <= 0 {
		partSize = 4 * 1024 * 1024 // 兜底 4MB（服务端正常会返回，此处防御）
	}
	etags := make([]string, 0, int(total/partSize)+1)
	partNumber := 1
	var uploaded int64
	for uploaded < total {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		end := uploaded + partSize
		if end > total {
			end = total
		}
		// SectionReader 零拷贝定位到本分片区间。
		body := io.NewSectionReader(req.ReaderAt, uploaded, end-uploaded)
		etag, err := s.putPart(ctx, preData, mime, partNumber, body)
		if err != nil {
			return nil, err
		}
		etags = append(etags, etag)
		uploaded = end
		partNumber++
		progress(uploaded, total)
	}

	// —— 步骤5：commit 合并 ——
	if err := s.commitParts(ctx, preData, etags); err != nil {
		return nil, err
	}

	// —— 步骤6：finish ——
	return s.finishUpload(ctx, preData)
}

// finishUpload 调 /file/upload/finish 完成上传，返回新文件信息。
func (s *Service) finishUpload(ctx context.Context, pre *upPreRespData) (*types.File, error) {
	body := map[string]any{
		"obj_key": pre.ObjKey,
		"task_id": pre.TaskID,
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"message"`
		Data types.File `json:"data"`
	}
	if err := invoker.PostAndDecode(ctx, s.inv, "/file/upload/finish", body, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, invoker.NewAPIError(resp.Code, resp.Msg)
	}
	f := resp.Data
	// finish 响应的 fid 可能空（秒传时），用 pre 的兜底。
	if f.FID == "" {
		f.FID = pre.FID
	}
	return &f, nil
}

// hashFile 流式计算 ReaderAt 的 md5 + sha1（hex 小写），按 32KB 块循环，零额外内存。
func hashFile(r io.ReaderAt, size int64) (string, string, error) {
	var md5H hash.Hash = md5.New()
	var sha1H hash.Hash = sha1.New()
	buf := make([]byte, 32*1024)
	var off int64
	for off < size {
		end := off + int64(len(buf))
		if end > size {
			end = size
		}
		n, err := r.ReadAt(buf[:end-off], off)
		if n > 0 {
			md5H.Write(buf[:n])
			sha1H.Write(buf[:n])
			off += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", fmt.Errorf("hash read failed: %w", err)
		}
	}
	return hex.EncodeToString(md5H.Sum(nil)), hex.EncodeToString(sha1H.Sum(nil)), nil
}

// —— 内部响应类型（夸克 API 响应结构，不导出）——

// upPreRespData 是 /file/upload/pre 返回的 data。
// callback 是 OSS 回调信息（commit 步骤要用）。
type upPreRespData struct {
	TaskID    string `json:"task_id"`
	ObjKey    string `json:"obj_key"`
	Bucket    string `json:"bucket"`
	UploadURL string `json:"upload_url"` // 形如 "https://cp1-quark.xstore.alicdn.com"
	UploadID  string `json:"upload_id"`
	AuthInfo  string `json:"auth_info"` // 夸克持有的 OSS 授权凭证
	FID       string `json:"fid"`       // 秒传成功时返回
	Callback struct {
		CallbackURL  string `json:"callbackUrl"`
		CallbackBody string `json:"callbackBody"`
	} `json:"callback"`
}

type upPreResp struct {
	Code int            `json:"code"`
	Msg  string         `json:"message"`
	Data upPreRespData  `json:"data"`
	Metadata struct {
		PartSize int `json:"part_size"` // 分片大小（字节）
	} `json:"metadata"`
}

type hashResp struct {
	Code int `json:"code"`
	Msg  string `json:"message"`
	Data struct {
		Finish bool `json:"finish"` // true=秒传成功
		FID    string `json:"fid"`
	} `json:"data"`
}

type upAuthResp struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
	Data struct {
		AuthKey string `json:"auth_key"` // 签名后的 OSS Authorization
	} `json:"data"`
}
