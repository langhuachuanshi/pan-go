// Package invoker 定义 pan-go 各网盘模块业务子包依赖的统一调用接口（"标准菜单"）。
//
// 实现方 = 各模块主包 Client；实现约定详见 core/template/GUIDE.md：
//   - path 相对模块 base；以 http(s):// 开头则视为完整 URL
//   - 实现方负责：base 拼装、公共 query、鉴权注入、UA；HTTP 状态 >= 400 时
//     返回 *coreerrors.APIError（Status 已填充，Kind=KindOther）
//   - 成功时把 JSON 响应体解到 out（nil 忽略）
//   - 业务码（code/errno/zt）判定是模块方言：由业务子包自行判定，
//     并构造 coreerrors.APIError（带 Kind）返回
package invoker

import (
	"context"
	"io"
)

// Invoker 业务子包依赖的最小调用接口。
type Invoker interface {
	// Get 发 GET，成功时把 JSON 响应解到 out（nil 忽略）。
	Get(ctx context.Context, path string, query map[string]string, out any) error
	// Post 发 POST JSON，成功时把 JSON 响应解到 out（nil 忽略）。
	Post(ctx context.Context, path string, body any, query map[string]string, out any) error
	// PostForm 发 POST 表单，成功时把 JSON 响应解到 out（nil 忽略）。
	PostForm(ctx context.Context, path string, form map[string]string, out any) error
	// Multipart 发 multipart 文件上传（单体），成功时把 JSON 响应解到 out（nil 忽略）。
	Multipart(ctx context.Context, path string, form map[string]string,
		field, filename string, file io.Reader, out any) error
	// DownloadHeaders 返回直链下载所需的鉴权头（Cookie/Referer/UA 或 Authorization）。
	DownloadHeaders() map[string]string
}
