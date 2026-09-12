package lanzou

import (
	"io"
	"sync/atomic"
)

// progressReader 包装 io.Reader，Read 时累计已读字节并回调。
// 回调在每次 Read 后触发（频率取决于 HTTP 客户端读取 buffer 大小）。
type progressReader struct {
	r          io.Reader
	total      int64
	uploaded   int64
	onProgress func(uploaded, total int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	if n > 0 {
		done := atomic.AddInt64(&pr.uploaded, int64(n))
		if pr.onProgress != nil {
			pr.onProgress(done, pr.total)
		}
	}
	return n, err
}
