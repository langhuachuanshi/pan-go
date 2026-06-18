// Package download 实现百度网盘文件下载（网页端 / BDUSS 方案）。
//
// 当前状态：未实现（占位）。
//
// 网页端下载是 BDUSS 方案里最复杂的一环，需要额外的 RC4 签名（Sign2）：
//   - 主路径 PCS locatedownload：需要 uid + NewLocateDownloadSign 签名
//   - 备用 /api/download：需要 panhome 签名（抓 disk/home HTML 提取 sign1/sign3/timestamp）
// 这两个签名算法本分支尚未移植，所以下载暂不可用。
//
// 如需下载，请切到 openapi 分支（OAuth + dlink，已实测字节级一致）。
package download

import (
	"context"
	"errors"
	"io"

	"github.com/langhuachuanshi/panbaidu-go/baidu/invoker"
)

// ErrNotImplemented 下载未实现。
var ErrNotImplemented = errors.New("download: 网页端下载暂未实现（需 RC4 签名），请使用 openapi 分支")

// Service 下载入口。
type Service struct {
	inv invoker.Invoker
}

// New 创建 download Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// DownloadRequest 下载请求。
type DownloadRequest struct {
	FSID       int64                         // 要下载的文件 fs_id
	Writer     io.Writer                     // 下载内容写入目标
	OnProgress func(downloaded, total int64) // 可选进度回调
}

// GetDownloadURL 获取下载直链（未实现）。
func (s *Service) GetDownloadURL(ctx context.Context, fsid int64) (string, error) {
	return "", ErrNotImplemented
}

// Download 下载文件内容到 Writer（未实现）。
func (s *Service) Download(ctx context.Context, req *DownloadRequest) error {
	return ErrNotImplemented
}
