// 本文件实现夸克上传的 OSS 直传部分（步骤 4b 分片 PUT、5b commit POST）。
//
// 夸克上传分两套请求：
//   - 夸克 API（/file/upload/pre 等）：走 invoker，cookie 鉴权，JSON body。
//   - OSS 直传（阿里云对象存储）：完全独立的 HTTP，用 Authorization 头鉴权，
//     二进制 PUT / XML POST body，且需要读响应头（Etag）。
// 所以这两步不能复用 invoker，在本包内独立发 HTTP。
//
// 签名机制：客户端不自己算 OSS 签名，而是把 CanonicalRequest 风格的
// "auth_meta" 字符串发给夸克 /file/upload/auth，夸克用 auth_info 里的密钥
// 算出 Authorization（auth_key）返回，客户端再原样塞进 OSS 请求头。
//
// auth_meta 是 OSS V1 CanonicalRequest 格式（极易错，照抄 AList 验证过的写法）。
package upload

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/langhuachuanshi/quark-go/quark/invoker"
)

// ossUserAgent 是 OSS 签名里的固定魔术串，必须照抄（参与 CanonicalizedOSSHeaders 签名）。
const ossUserAgent = "aliyun-sdk-js/6.6.1 Chrome 98.0.4758.80 on Windows 10 64-bit"

// ossHTTP 直传 OSS 用的 client（不走 cookie，独立于夸克 API）。
var ossHTTP = &http.Client{Timeout: 10 * time.Minute}

// partAuthMeta 构造分片 PUT 的签名串（CanonicalRequest 格式）。
// Content-MD5 行为空（分片 PUT 不算 body 的 MD5）。
func partAuthMeta(bucket, objKey, mime, date string, partNumber int, uploadID string) string {
	return fmt.Sprintf("PUT\n\n%s\n%s\nx-oss-date:%s\nx-oss-user-agent:%s\n/%s/%s?partNumber=%d&uploadId=%s",
		mime, date, date, ossUserAgent, bucket, objKey, partNumber, uploadID)
}

// commitAuthMeta 构造 commit(CompleteMultipartUpload) 的签名串。
// Content-MD5 行有值（XML body 的 base64 MD5），并多一个 x-oss-callback 头。
func commitAuthMeta(bucket, objKey, contentMD5, date, callbackBase64, uploadID string) string {
	return fmt.Sprintf("POST\n%s\napplication/xml\n%s\nx-oss-callback:%s\nx-oss-date:%s\nx-oss-user-agent:%s\n/%s/%s?uploadId=%s",
		contentMD5, date, callbackBase64, date, ossUserAgent, bucket, objKey, uploadID)
}

// ossHost 用 pre 响应里的 bucket + upload_url 拼出 OSS 直传的 host。
// upload_url 形如 "https://cp1-quark.xstore.alicdn.com"，剥掉 "https://"(7字符) 后
// 拼成 https://{bucket}.{裸域名}/{objKey}（AList 的标准写法）。
func ossHost(pre *upPreRespData) string {
	host := pre.UploadURL
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	return "https://" + pre.Bucket + "." + host
}

// requestAuth 调夸克 /file/upload/auth，把 auth_meta 交给夸克签名，拿回 OSS 的 Authorization。
func (s *Service) requestAuth(ctx context.Context, pre *upPreRespData, authMeta string) (string, error) {
	body := map[string]any{
		"auth_info": pre.AuthInfo,
		"auth_meta": authMeta,
		"task_id":   pre.TaskID,
	}
	var resp upAuthResp
	if err := invoker.PostAndDecode(ctx, s.inv, "/file/upload/auth", body, nil, &resp); err != nil {
		return "", err
	}
	if resp.Code != 0 {
		return "", invoker.NewAPIError(resp.Code, resp.Msg)
	}
	return resp.Data.AuthKey, nil
}

// putPart 完成单个分片的 OSS 直传：换 auth → PUT 分片字节 → 返回 ETag。
func (s *Service) putPart(ctx context.Context, pre *upPreRespData, mime string, partNumber int, body io.Reader) (string, error) {
	date := time.Now().UTC().Format(http.TimeFormat)
	authKey, err := s.requestAuth(ctx, pre, partAuthMeta(pre.Bucket, pre.ObjKey, mime, date, partNumber, pre.UploadID))
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("%s/%s?partNumber=%d&uploadId=%s", ossHost(pre), pre.ObjKey, partNumber, pre.UploadID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", authKey)
	req.Header.Set("Content-Type", mime)
	req.Header.Set("Referer", "https://pan.quark.cn/")
	req.Header.Set("x-oss-date", date)
	req.Header.Set("x-oss-user-agent", ossUserAgent)

	resp, err := ossHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("oss put 分片 %d 失败: status=%d body=%s", partNumber, resp.StatusCode, string(b))
	}
	return resp.Header.Get("Etag"), nil
}

// commitParts 合并所有分片：构造 XML → 算 Content-MD5 → 换 auth → POST 给 OSS。
func (s *Service) commitParts(ctx context.Context, pre *upPreRespData, etags []string) error {
	// 1. 构造 CompleteMultipartUpload XML。
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n<CompleteMultipartUpload>\n")
	for i, etag := range etags {
		fmt.Fprintf(&b, "<Part>\n<PartNumber>%d</PartNumber>\n<ETag>%s</ETag>\n</Part>\n", i+1, etag)
	}
	b.WriteString("</CompleteMultipartUpload>")
	xmlBody := b.String()

	// 2. Content-MD5 = base64(md5(xml))。
	h := md5.New()
	h.Write([]byte(xmlBody))
	contentMD5 := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// 3. callback = base64(json(pre.callback))。
	cbBytes, err := json.Marshal(pre.Callback)
	if err != nil {
		return err
	}
	callbackBase64 := base64.StdEncoding.EncodeToString(cbBytes)

	// 4. 换 commit 的 auth（带 Content-MD5 + callback 的签名串）。
	date := time.Now().UTC().Format(http.TimeFormat)
	authKey, err := s.requestAuth(ctx, pre, commitAuthMeta(pre.Bucket, pre.ObjKey, contentMD5, date, callbackBase64, pre.UploadID))
	if err != nil {
		return err
	}

	// 5. POST 给 OSS 合并。
	url := fmt.Sprintf("%s/%s?uploadId=%s", ossHost(pre), pre.ObjKey, pre.UploadID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(xmlBody))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", authKey)
	req.Header.Set("Content-MD5", contentMD5)
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Referer", "https://pan.quark.cn/")
	req.Header.Set("x-oss-callback", callbackBase64)
	req.Header.Set("x-oss-date", date)
	req.Header.Set("x-oss-user-agent", ossUserAgent)

	resp, err := ossHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("oss commit 失败: status=%d body=%s", resp.StatusCode, string(b))
	}
	return nil
}
