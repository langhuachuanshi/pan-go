# 通用网盘基础模块（core）统一架构计划 · 定稿

- 状态：**定稿待批准**（设计决策已由 agent 按用户委托定死，批准后按阶段执行）
- 日期：2026-09-12（v2 定稿，取代同日 v1 草案）
- 决策人：用户委托 agent 设计，5 个决策点全部按"低风险、高收益"原则定死，见决议记录

## 决议记录（5 个决策点）

| # | 决策点 | 结论 | 理由 |
|---|---|---|---|
| 1 | 模块命名 | **`core`** | 最短、语义准、import 顺；改名是破坏性变更，一次定对 |
| 2 | 通用文件模型 | **接口**（ID/Name/Size/IsFolder 四方法），不做通用结构体 | 各模块自有类型字段丰富，强推结构体=又一轮破坏性重构；接口零成本接入（加 4 个方法），跨网盘通用代码够用 |
| 3 | 分片上传骨架 | **本期不做**，列二期 | 三个模块的秒传/分片流程差异大，强行抽钩子风险高收益不确定；HTTP/错误/文件模型三层收益大风险小，先落。上传骨架等 core 稳定后另开计划 |
| 4 | 迁移顺序 | **lanzou 试点 → quark → baidu → alipan** | lanzou 刚重构完最干净；alipan 最复杂放最后。每模块独立提交，出问题只回滚该模块 |
| 5 | 方法名统一 | **轻度对齐**：借迁移之机只对齐文件/上传/下载主入口命名，其余保持并记录差异 | 外部使用方（backend/workbench）还没切 pan-go，趁无人引用做轻对齐最便宜；强行 1:1 统一几十个方法=又一大轮破坏性变更 |

## 本期范围（做什么 / 不做什么）

**做**：HTTP 执行器（httpx）、统一 Invoker 接口、统一错误语义（errors）、通用文件模型（types）、新网盘接入指南（template 文档）、四模块逐个迁移。

**不做**（显式排除，防 scope 膨胀）：分片上传骨架（二期）、通用重试策略配置化（httpx 只带简单重试）、统一各模块业务 Service 的方法集合、统一各模块的鉴权实现（鉴权是差异，只统一插槽位置）。

## core 模块设计（定稿）

```
core/
├── go.mod              # module github.com/langhuachuanshi/pan-go/core
├── AGENTS.md / README.md / API.md / CHANGELOG.md   # 同款四件套
├── httpx/              # HTTP 执行器（无鉴权无业务，纯执行）
│   └── httpx.go
├── invoker/            # 业务子包依赖的统一调用接口
│   └── invoker.go
├── errors/             # 统一错误类型 + 语义判定
│   └── errors.go
├── types/              # 通用文件模型接口
│   └── types.go
└── template/           # 新网盘接入指南（文档，不是代码骨架）
    └── GUIDE.md
```

### 接口签名（定稿，实现时不再改形状）

```go
// core/httpx：纯执行器。不知道鉴权、不知道业务，只管发。
package httpx

type Config struct {
    UserAgent string        // 空=默认 UA
    Timeout   time.Duration // 0=30s
    Retries   int           // 传输层错误重试次数，0=不重试
    Logf      func(format string, args ...any) // 可选请求日志（URL/耗时/状态码）
}

type Executor struct{ /* ... */ }
func New(cfg Config) *Executor

// 请求描述。Body 用实现类型表达编码方式。
type Request struct {
    Method  string
    URL     string
    Query   url.Values
    Body    Body              // nil = 无请求体
    Headers map[string]string // 额外头（模块在此注入鉴权/差异头）
}

type Body interface {
    ContentType() string
    Encode() (io.Reader, error)
}
// 现成实现：JSONBody(any)、FormBody(map[string]string)、RawBody(ct, reader)

func (e *Executor) Do(ctx context.Context, r *Request) (*Response, error)
type Response struct {
    StatusCode int
    Header     http.Header
    Body       []byte
}
```

```go
// core/invoker：各模块业务子包依赖的最小接口。主包 Client 实现。
// 每个模块可在此之上扩展模块特有方法（如 lanzou 的 FetchPageWithChallenge），
// 业务子包依赖"core.Invoker + 模块扩展"的本地接口。
package invoker

type Invoker interface {
    // Get GET 并按模块的判错规则校验，成功时把响应体解到 out（nil 则忽略）。
    Get(ctx context.Context, path string, query map[string]string, out any) error
    // Post POST JSON body。
    Post(ctx context.Context, path string, body any, query map[string]string, out any) error
    // PostForm POST 表单（baidu/lanzou 主用）。
    PostForm(ctx context.Context, path string, form map[string]string, out any) error
    // Multipart 单体 multipart 上传（小文件/接口性上传）。
    Multipart(ctx context.Context, path string, form map[string]string,
        field, filename string, file io.Reader, out any) error
    // DownloadHeaders 直链下载所需鉴权头（Cookie/Referer/UA 或 Authorization）。
    DownloadHeaders() map[string]string
}
```

```go
// core/errors：统一错误 + 语义标签。模块判错时打 Kind，调用方按语义判断。
package errors

type Kind uint8
const (
    KindOther Kind = iota
    KindAuth         // 登录态失效 → 引导重新登录/扫码
    KindRateLimited  // 限频
    KindNotFound     // 资源不存在
    KindDenied       // 无权限/风控拦截
)

type APIError struct {
    Module  string // "quark" / "baidu" / "lanzou" / "alipan"
    Code    int    // 业务码（夸克 code、百度 errno、蓝奏 zt…）
    Status  int    // HTTP 状态码
    Message string
    kind    Kind
}
func New(module string, code, status int, message string, kind Kind) *APIError
func (e *APIError) Error() string
func (e *APIError) Kind() Kind

func IsAuth(err error) bool         // e.Kind()==KindAuth
func IsRateLimited(err error) bool
func IsNotFound(err error) bool
func IsDenied(err error) bool
```

**各模块判错映射（接入时登记，写进各模块 AGENTS）**：

| 模块 | 登录失效 → KindAuth | 限频 → KindRateLimited | 备注 |
|---|---|---|---|
| quark | 31001、31003 | 41013 | 现有 IsAuthError 平移 |
| baidu | -6 | -70?（接入时实测确认） | errno 判定 |
| lanzou | 未登录哨兵 | 无 | zt 判定 |
| alipan | 401/token 失效 | Throttling 码 | 接入时确认 |

```go
// core/types：通用文件模型（接口，各模块自有类型加 4 个方法即接入）。
package types

type File interface {
    ID() string
    Name() string
    Size() int64
    IsFolder() bool
}
```

```go
// core/template/GUIDE.md：新网盘接入指南（内容要点）
// 1. 目录骨架：go.mod + 主包（Client 实现 core invoker）+ 业务子包 + 四件套文档
// 2. 三件事：判错映射（自家码→Kind）、鉴权注入（在 Request.Headers 组装）、编码选择（JSON/Form）
// 3. checklist：build/vet/test 全过 → 每个接口真机实测 → 四件套同步 → 打 tag
```

## 迁移阶段（批准后执行，每阶段独立提交独立验证）

- [ ] **阶段 1：实现 core**（httpx/invoker/errors/types + 单测：httpx 用 httptest 覆盖
      GET/POST/Form/Multipart/重试/超时；errors 覆盖 Kind 判定）+ core 四件套文档
      验收：`cd core && go build/vet/test ./...` 全过
- [ ] **阶段 2：lanzou 迁移（试点）**——删除 http.go 自有执行细节改调 httpx；主包实现
      core/invoker（含挑战页扩展）；业务子包换统一接口；错误换 core/errors（lanzou.ErrXxx
      别名保留）；FileInfo 加 4 个模型方法；顺带对齐 Download().File/FileAuto 主入口名
      验收：build/vet/test 全过 + 现有单测不变绿
- [ ] **阶段 3：quark 迁移**——同上；IsAuthError 平移为 core/errors 语义（保留别名）
      验收：同上（含分页三个单测不变绿）
- [ ] **阶段 4：baidu 迁移**——同上（form/multipart 走 httpx Body 实现）
      验收：同上（3 个测试包不变绿）
- [ ] **阶段 5：alipan 迁移**——同上（最复杂，token 刷新逻辑留在模块，只换执行层）
      验收：同上
- [ ] **阶段 6：收尾**——template/GUIDE.md 成稿；四模块 AGENTS 补"判错映射表"；
      根 README/TODO 更新；全量验证 + push（tag 仍等用户明确指令）
      验收：全模块 build/vet/test 绿；grep 确认四模块主包无重复 http.Client.Do

## 依赖与发版规则

- core 是第五个 module：`github.com/langhuachuanshi/pan-go/core`，独立 tag `core/v26.x.y`。
- 同仓内 go.work 保证本地开发无缝；外部使用方 go get 各网盘模块时 core 作为间接依赖
  自动拉取（间接依赖，无需显式 import）。
- 各模块公开 API 中**不直接暴露 core 类型**：错误语义用别名透出（`lanzou.IsAuth`），
  文件模型用接口实现（模块返回自有类型）。使用方零感知，跨网盘代码才 import core。
- 迁移期间 core 与未迁移模块共存；单模块回滚只 revert 该模块的提交。

## 风险与对策

| 风险 | 对策 |
|---|---|
| 抽象面过大导致新网盘接入反而难 | 钩子只留编码/鉴权/判错三处；本期砍掉上传骨架 |
| 迁移引入回归 | 每模块独立提交；现有单测必须保持绿；试点先行（lanzou）验证抽象够用 |
| 判错映射不全（漏码归错类） | 映射表登记进各模块 AGENTS；不确定的码实测后补；KindOther 兜底 |
| 破坏使用方 | 使用方尚未切换 pan-go，现在是零成本窗口；切换前完成全部迁移 |

## 二期候选（本期明确不做，届时另开计划）

分片上传骨架（prepare/part/commit 钩子化）；通用重试/限速策略配置化；`__puus` 类 cookie 保活钩子。
