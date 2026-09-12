# core 模块 API 参考

> core 是库不是网盘 SDK，"接口形式"为 Go 函数签名。面向模块作者。

## 全局约定

- core 零第三方依赖、零网盘知识；鉴权与业务码判定永远在模块侧。
- import 别名惯例：`coreerrors "github.com/langhuachuanshi/pan-go/core/errors"`。

---

## httpx 执行器

### 创建执行器

**方法名**：`New`

**方法签名**：`func New(cfg Config) *Executor`

**调用示例**：

```go
exec := httpx.New(httpx.Config{
    UserAgent: "quark-cloud-drive/2.5.20 ...", // 模块自己的 UA
    Timeout:   30 * time.Second,
    Retries:   1,
    Logf:      log.Printf,
})
```

**参数说明**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| UserAgent | string | 否 | 空=默认 `pan-go/core` |
| Timeout | time.Duration | 否 | 单请求超时，0=30s |
| Retries | int | 否 | 传输层错误重试次数，0=不重试 |
| Logf | func(string, ...any) | 否 | 请求日志钩子 |

**返回值**：`*Executor`（配置不可变，可并发复用）。

**失败**：无（构造不报错）。

**注意**：Executor 并发安全；Retries>0 时 Body.Encode 会被多次调用。

### 执行请求

**方法名**：`Do`

**方法签名**：`func (e *Executor) Do(ctx context.Context, r *Request) (*Response, error)`

**调用示例**：

```go
resp, err := exec.Do(ctx, &httpx.Request{
    Method: http.MethodPost,
    URL:    "https://drive-pc.quark.cn/1/clouddrive/file/sort",
    Query:  url.Values{"pr": {"ucpro"}, "fr": {"pc"}},
    Body:   httpx.JSONBody{V: map[string]any{"fid_list": fids}},
    Headers: map[string]string{"Cookie": cookie},
})
```

**参数说明**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| r.URL | string | 是 | 完整 URL |
| r.Method | string | 否 | 空=GET |
| r.Query | url.Values | 否 | 拼 URL |
| r.Body | Body | 否 | JSONBody / FormBody / RawBody |
| r.Headers | map[string]string | 否 | 鉴权等模块差异头 |

**返回值**：`resp.StatusCode / Header / Body`。

**失败**：仅传输层错误（超时/断连，按 Retries 重试）；**HTTP 非 2xx 不是错误**，原样返回。

**注意**：业务码判定属于模块方言，在调用方完成（配合 core/errors）。

---

## errors 统一错误

### 构造

**方法名**：`New`

**方法签名**：`func New(module string, code, status int, message string, kind Kind) *APIError`

**调用示例**：

```go
return coreerrors.New("quark", 31003, status, "cookie 失效", coreerrors.KindAuth)
```

**参数说明**：module（模块名）、code（业务码）、status（HTTP 码）、message、kind（语义）。

**Kind 取值**：`KindOther / KindAuth（登录态失效）/ KindRateLimited / KindNotFound / KindDenied`。

### 语义判定

| 方法 | 说明 |
|------|------|
| `IsAuth(err) bool` | 登录态失效（errors.As 穿透 wrap） |
| `IsRateLimited(err) bool` / `IsNotFound(err) bool` / `IsDenied(err) bool` | 其余语义 |
| `(*APIError).Kind() Kind` | 取语义 |

**注意**：非 `*APIError`（含 nil、普通 error）一律返回 false，不 panic。

---

## invoker 统一接口

业务子包依赖的"标准菜单"，各模块主包 Client 实现（契约见 package doc 与
template/GUIDE.md）：

| 方法 | 说明 |
|------|------|
| `Get(ctx, path, query, out) error` | GET + 判错 + JSON 解码到 out |
| `Post(ctx, path, body, query, out) error` | POST JSON |
| `PostForm(ctx, path, form, out) error` | POST 表单 |
| `Multipart(ctx, path, form, field, filename, file, out) error` | 单体 multipart 上传 |
| `DownloadHeaders() map[string]string` | 直链下载鉴权头 |
