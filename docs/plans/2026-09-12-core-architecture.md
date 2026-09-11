# 通用网盘基础模块（统一架构）计划

- 状态：**待讨论**——按用户要求只落计划，未批准不执行
- 日期：2026-09-12
- 背景：四个网盘模块是 vibe coding 分四次独立写成的，功能形态一致但写法各异；
  用户决定新建一个通用基础模块统一架构，以后新增网盘照模板接入

## 一、现状盘点（问题证据）

四个模块已完成的部分统一：invoker + Service 分层、`New(inv)` 构造、
`c.Files().List(...)` 风格访问器、模块内三件套文档。**尚未统一的：**

| 维度 | alipan | baidu | lanzou | quark |
|---|---|---|---|---|
| HTTP 执行位置 | 主包 client.go `request()` | 主包 client.go `doRequest` | 主包 http.go `doRequest` | 主包 client.go `request()` |
| 请求体编码 | JSON | form + multipart | form + multipart（表单为主） | JSON |
| 鉴权注入位置 | Authorization header（token 自动刷新） | cookiejar + query 追加 bdstoken | 手动 AddCookie + cookie 合并 | 直接设 Cookie header |
| 公共头（UA/Referer/Origin） | 各写一份 | 各写一份 | 各写一份 | 各写一份 |
| 错误判定 | code != 0（信封） | errno != 0 | zt != 1（doupload）/ zt != 1 且 mess（ajaxm） | code != 0 或 status != 200 |
| 登录态失效判定 | 各自判断 | 无哨兵 | 无哨兵 | IsAuthError(31001/31003) |
| 分片上传 | 有（秒传+分片+续期） | 有（precreate+superfile2） | 无 | 有（秒传+OSS 分片） |
| 直链下载鉴权头 | 各自实现 | DownloadHeaders() | 调用方自理 | DownloadHeaders() |

**结论**：同样的逻辑（执行请求 / 注入凭证 / 判错 / 分片上传 / 直链下载头）写了四遍，
修一处 bug 要改四处——这是抽象层缺位的直接代价。

## 二、目标

新增一个基础模块（暂名 **`core/`**，命名待定），沉淀四模块的公共能力；
四个现有模块逐步迁移其上；**今后新增网盘模块 = 实现 core 定义的钩子接口 + 照模板填目录**。

统一后每个网盘模块只保留"差异"：

| 保留在各模块（差异） | 上收到 core（共性） |
|---|---|
| 鉴权方式（cookie / token / 签名） | HTTP 执行器（超时/重试/限速/日志/UA） |
| 端点与请求体编码 | 编码钩子接口（JSON/form/multipart 序列化） |
| 业务错误码表 | 统一错误类型与判定链（IsAuthError 等） |
| 业务 Service（文件/分享/上传…） | 通用文件模型、分片上传骨架、直链下载鉴权头约定 |
| 模块 AGENTS / README / API.md / CHANGELOG | 新模块接入模板与 checklist |

## 三、core 模块设计草案（讨论稿，接口签名可改）

```
core/
├── go.mod              # module github.com/langhuachuanshi/pan-go/core（tag: core/vX.Y.Z）
├── invoker/            # 统一 Invoker 接口（服务子包依赖的最小面）
├── httpx/              # HTTP 执行器：请求构建/UA/超时/重试/限速/日志
├── errors/             # APIError 基类 + 错误语义（Auth/RateLimit/NotFound...）+ Is 判定链
├── types/              # 通用文件模型接口（Name/Size/IsFolder/FileID）
├── upload/             # 分片上传骨架（哈希/分片切片/进度/断点；prepare/part/commit 钩子）
└── template/           # 新网盘模块接入模板 + checklist（说明文档）
```

接口草案（示意）：

```go
// core/invoker
type Invoker interface {
    Get(ctx, path string, q Query) ([]byte, error)
    Post(ctx, path string, body Encodable) ([]byte, error)
    Multipart(ctx, path string, m MultipartBody) ([]byte, error)
    DownloadHeaders() map[string]string   // 直链下载统一鉴权头
}
// 模块侧差异通过构造 invoker 时注入的三个钩子表达：
//   Encode(body) → 请求体（JSON/form）  Authorize(req) → 注入凭证
//   Judge(status, body) → 业务错误判定
```

```go
// core/errors —— 各模块把自家错误码映射到统一语义
type APIError struct{ Code int; Status int; Message string }
func IsAuth(err error) bool        // 31001/31003（夸克）、-6（百度）…由各模块注册
func IsRateLimited(err error) bool
```

```go
// core/types —— 通用文件模型（接口而非结构体，避免破坏各模块自有类型）
type File interface {
    ID() string
    Name() string
    Size() int64
    IsFolder() bool
}
```

## 四、迁移策略

1. **先建 core，不动现有代码**（core 上线即向后兼容）。
2. 四模块逐个迁移，顺序建议 **lanzou → quark → baidu → alipan**
   （lanzou 刚重构完最干净；alipan 最复杂放最后）。每模块独立提交、独立验证。
3. 每迁移完一个模块，其业务子包的 `inv` 换成 core/invoker；自家 HTTP 底层删除。
4. 迁移全部完成后，`core/template/` 输出新网盘接入模板（目录骨架 + checklist），
   此后新增网盘（如天翼、115、迅雷）照模板实现。
5. 发版：core 独立版本（`core/v26.37.x`），各模块 require 它并按需升级。

## 五、风险与代价

- 四模块**再次全量破坏性变更**（import 路径 + 接口签名），外部引用需跟着改。
- core 作为第五个 module：同仓互赖在 go.work 下本地无缝；发布后跨模块依赖
  用带前缀 tag（`core/vX.Y.Z`），依赖方 require 对应版本。
- 过度抽象风险：接口面若过大，反而让新网盘接入变难——钩子只留
  「编码 / 鉴权 / 判错」三个，能不加就不加。

## 六、阶段划分（批准后执行）

1. [ ] 阶段 1：core 接口定稿（invoker/httpx/errors/types 接口评审，出正式签名）
2. [ ] 阶段 2：实现 core 模块 + 单测（httpx 用 httptest 覆盖）
3. [ ] 阶段 3：lanzou 迁移（试点，验证 core 抽象是否够用，反哺接口修正）
4. [ ] 阶段 4：quark → baidu → alipan 依次迁移
5. [ ] 阶段 5：core/template 新网盘接入模板 + checklist；全模块文档同步
6. [ ] 阶段 6：全量验证 + 提交推送（tag 等用户明确指令）

## 七、待讨论决策点

1. core 模块命名：`core/`（建议）还是 `pancore/`、`base/`？
2. 通用文件模型：接口（建议）还是要求各模块直接复用 core 结构体？
3. 分片上传骨架是否本期纳入（也可先只统一 HTTP/错误/模型，上传骨架下期）？
4. 迁移顺序是否按建议（lanzou 试点先行）？
5. 各模块对外 API 是否借机再统一一轮方法名（当前已基本一致）？
