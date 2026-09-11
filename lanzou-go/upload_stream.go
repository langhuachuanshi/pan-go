package lanzou

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
)

// progressReader 包装 io.Reader，Read 时累计已读字节并回调。
// 回调在每次 Read 后触发（频率取决于 HTTP 客户端读取 buffer 大小）。
type progressReader struct {
	r        io.Reader
	total    int64
	uploaded int64
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

// UploadFileWithProgress 上传本地文件到蓝奏云，支持上传进度回调。
//
// 与 UploadFile 的区别：用流式上传（io.Pipe + multipart.NewWriter 直接写
// request body），文件内容边读边发，不一次性载入内存。onProgress 在网络
// 传输时触发，反映真实上传进度（不是"读到内存"的进度）。
//
// 参数：
//   - filePath: 本地文件路径
//   - fid: 目标文件夹 ID（根目录传 0）
//   - onProgress: 上传进度回调（uploaded, total 字节），可为 nil
//   - desc: 文件描述（可选）
func (c *Client) UploadFileWithProgress(filePath string, fid int, onProgress func(uploaded, total int64), desc ...string) (*UploadResult, error) {
	if !c.isLoggedIn() {
		return nil, ErrNotLoggedIn
	}

	// 检查文件大小
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat file failed: %w", err)
	}
	if info.Size() > int64(c.maxsize) {
		return nil, fmt.Errorf("%w: file size %d exceeds limit %d", ErrFileSizeLimit, info.Size(), c.maxsize)
	}

	// 打开文件
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file failed: %w", err)
	}
	defer f.Close()

	descStr := ""
	if len(desc) > 0 {
		descStr = desc[0]
	}
	fields := map[string]string{
		"task":           "1",
		"vie":            "2",
		"ve":             "2",
		"id":             "WU_FILE_0",
		"folder_id_bb_n": fmt.Sprintf("%d", fid),
		"name":           filepath.Base(filePath),
	}
	if descStr != "" {
		fields["des"] = descStr
	}

	headers := map[string]string{
		"Referer": baseURLPC + "/mydisk.php",
		"Origin":  baseURLPC,
	}

	body, _, err := c.postMultipartStream(
		baseURLPC+pathUpload,
		fields,
		"upload_file",
		filepath.Base(filePath),
		f,
		info.Size(),
		onProgress,
		headers,
	)
	if err != nil {
		return nil, fmt.Errorf("upload request failed: %w", err)
	}

	return parseUploadResult(body)
}

// postMultipartStream 流式 multipart 上传。
//
// 用 io.Pipe：goroutine 里用 multipart.NewWriter 写 pipe writer，
// HTTP 客户端从 pipe reader 读取并发送。文件字段用 progressReader 包装，
// 边写边触发 onProgress。
//
// 注意：流式上传走 chunked transfer encoding（无 Content-Length）。
// 蓝奏 html5up.php 是标准 PHP 上传接口，支持 chunked。
func (c *Client) postMultipartStream(rawURL string, fields map[string]string, fileField, fileName string, fileReader io.Reader, fileSize int64, onProgress func(uploaded, total int64), headers map[string]string) ([]byte, http.Header, error) {
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
		pr := &progressReader{
			r:          fileReader,
			total:      fileSize,
			onProgress: onProgress,
		}
		if _, err := io.Copy(part, pr); err != nil {
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
