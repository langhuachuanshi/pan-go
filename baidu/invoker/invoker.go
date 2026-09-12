// Package invoker 定义 baidu 业务子包依赖的调用接口与错误辅助。
//
// 接口 = core 标准菜单 + baidu 方言扩展（Raw 全 URL 系列、PCS 分片上传、body+query
// 分离的 PostFormQuery）；实现方是主包 Client（执行走 core/httpx，cookie 由 jar 携带）。
// 业务码（errno）判定留在业务子包，经 NewAPIError 构造统一语义错误。
package invoker

import (
	"context"
	"encoding/json"
	"net/http"

	coreerrors "github.com/langhuachuanshi/pan-go/core/errors"
	coreinvoker "github.com/langhuachuanshi/pan-go/core/invoker"
)

// APIError 统一错误（coreerrors.APIError 别名）。
type APIError = coreerrors.APIError

// NewAPIError 构造 baidu 业务错误（errno 码表 → 语义 Kind：-6=登录态失效，其余 Other）。
func NewAPIError(errno int, message string) *APIError {
	kind := coreerrors.KindOther
	if errno == -6 {
		kind = coreerrors.KindAuth
	}
	return coreerrors.New("baidu", errno, 200, message, kind)
}

// IsAuthError 登录态失效判定（coreerrors.IsAuth 别名）。
func IsAuthError(err error) bool { return coreerrors.IsAuth(err) }

// Invoker baidu 业务子包依赖的接口：core 标准菜单 + baidu 方言扩展。
type Invoker interface {
	coreinvoker.Invoker

	// —— 方言扩展 ——
	// GetRaw 发 GET 到完整 URL，不注入通用 query。用于 share/record 等特殊接口。
	GetRaw(ctx context.Context, fullURL string) ([]byte, int, error)
	// PostFormRaw 发 POST form 到完整 URL，不注入通用 query/bdstoken。用于 share、PCS 等接口。
	PostFormRaw(ctx context.Context, fullURL string, body map[string]string) ([]byte, int, error)
	// PostMultipartForm 发 POST multipart/form-data（字段模式，非文件上传）。
	PostMultipartForm(ctx context.Context, fullURL string, fields map[string]string) ([]byte, int, error)
	// PostMultipart 分片上传（superfile2 走 pcs 域名；params 进 query，data 为分片内容）。
	PostMultipart(ctx context.Context, baseURL, path string, params map[string]string, fieldName, fileName string, data []byte) ([]byte, int, error)
	// PostFormQuery POST form 且 body 与 query 分离（/api/filemanager 的 opera 在 query）。
	PostFormQuery(ctx context.Context, path string, body, params map[string]string, out any) error
	// HTTPClient 返回内部客户端（携带 BDUSS jar；下载/PanHome 用）。
	HTTPClient() *http.Client
}

// Decode 反序列化，空体不报错。
func Decode(data []byte, out any) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

// GetAndDecode GET + 解码（core 菜单形态：判错/解码在实现方，调用方零改动）。
func GetAndDecode(ctx context.Context, inv Invoker, path string, params map[string]string, out any) error {
	return inv.Get(ctx, path, params, out)
}

// PostFormAndDecode POST form + 解码（body 与 query 分离，方言经 PostFormQuery）。
func PostFormAndDecode(ctx context.Context, inv Invoker, path string, body, params map[string]string, out any) error {
	return inv.PostFormQuery(ctx, path, body, params, out)
}

// PostMultipartAndDecode POST multipart + 解码（方言）。
func PostMultipartAndDecode(ctx context.Context, inv Invoker, baseURL, path string, params map[string]string, fieldName, fileName string, data []byte, out any) error {
	respData, _, err := inv.PostMultipart(ctx, baseURL, path, params, fieldName, fileName, data)
	if err != nil {
		return err
	}
	if out != nil {
		return Decode(respData, out)
	}
	return nil
}
