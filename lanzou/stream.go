package lanzou

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
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

// PostMultipartStream 流式 multipart 上传。
//
// 用 io.Pipe：goroutine 里用 multipart.NewWriter 写 pipe writer，
// HTTP 客户端从 pipe reader 读取并发送。文件字段用 progressReader 包装，
// 边写边触发 onProgress。
//
// 注意：流式上传走 chunked transfer encoding（无 Content-Length）。
// 蓝奏 html5up.php 是标准 PHP 上传接口，支持 chunked。
func (c *Client) PostMultipartStream(rawURL string, fields map[string]string, fileField, fileName string, fileReader io.Reader, fileSize int64, onProgress func(uploaded, total int64), headers map[string]string) ([]byte, http.Header, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	// goroutine：写 multipart body 到 pipe writer
	writeErr := make(chan error, 1)
	go func() {
		defer pw.Close()
		defer mw.Close()
		// 写普通字段
		for k, v := range fields {
			if err := mw.WriteField(k, v); err != nil {
				writeErr <- err
				return
			}
		}
		// 写文件字段（用 progressReader 包装，触发回调）
		part, err := mw.CreateFormFile(fileField, fileName)
		if err != nil {
			writeErr <- err
			return
		}
		wrapped := &progressReader{
			r:          fileReader,
			total:      fileSize,
			onProgress: onProgress,
		}
		if _, err := io.Copy(part, wrapped); err != nil {
			writeErr <- err
			return
		}
		writeErr <- nil
	}()

	req, err := http.NewRequest("POST", rawURL, pr)
	if err != nil {
		return nil, nil, err
	}
	c.setCommonHeaders(req)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for _, cookie := range c.cookies {
		req.AddCookie(cookie)
	}

	body, header, err := c.doRequest(req)
	if err != nil {
		return nil, nil, err
	}
	// 检查写 goroutine 是否出错（如果 body 写到一半失败，请求会报错，但这里再确认一次）
	if err := <-writeErr; err != nil {
		return nil, nil, fmt.Errorf("write multipart body failed: %w", err)
	}
	return body, header, nil
}
