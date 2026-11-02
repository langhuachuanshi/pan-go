# 更新日志

记录 lanzou 模块的全部对外变更；条目与 tag 一一对应，先补条目后打 tag。

## [0.2645.3] - 2026-11-03

### 修复

- ctx 贯通：HTTP 底层 get/post 与 PostFormHeaders 原先用（或等效于）`context.Background()`，
  调用方传入的 ctx 对全部请求不生效——外层超时只能靠客户端 15s 兜底。新增
  `FetchPageWithChallengeCtx` / `getCtx` / `postCtx`，直链解析三跳（分享页/iframe/ajax POST）
  全程受调用方 ctx 控制

### 变更

- 直链解析 ajax 端点由串行逐个重试改为**延迟对冲**：先发首个候选，`hedgeDelay`(400ms) 内无结果
  或已发请求全数快速失败时补发其余候选，先成功者胜；密码错/文件失效等终局结果立即返回不等候选。
  动机与实测：串行时首个端点慢/挂会把整段解析拖到 5~10s（尾延迟大头），对冲后该场景封顶
  ~450ms（单测覆盖）；平静网络不多发请求、p50 与旧版打平（真网 A/B 各 32 次解析验证）
- 实测否决并撤销：默认 Transport 克隆 + TLS 会话缓存——每客户端独立连接池冷启动 p50 反而
  +170ms（蹭全局 DefaultTransport 的共享连接池更快），收益场景（连接回收后重连）在生产
  单例 client 下几乎遇不到

## [0.2645.2] - 2026-11-02

### 修复

- 直链解析适配蓝奏新页面：ajax 端点改成绝对地址（`https://apifile.woozooo.com/ajaxfile.php` /
  `https://apifile.lanzouw.com/ajaxfile.php`），页面新增 `kdns` 参数；旧相对端点继续兼容，双端点逐个重试
- 下载直链改走浏览器特征请求：补 `Accept` / `Accept-Language` 头、不带登录 cookie（带登录态会被风控拦成
  「网络异常」验证页），并跟随 302 的 `Location` 取真实 CDN 地址
- 流式下载统一处理 `acw_sc__v2` 挑战页（下载域同样会先给挑战页）

### 新增

- `invoker.ErrVerifyRequired`：蓝奏 CDN 要求浏览器验证时返回该哨兵错误，调用方据此回退其它下载源

## [0.2645.1] - 2026-11-02

### 变更

- `go` 指令由 1.26.4 降到 1.20，依赖 core 升到 v0.2645.1（同为 1.20 口径）：使用方可编回 Go 1.20（Win7）

### 修复

- `absorbCookies` 改用 `http.Response.Cookies()` 解析 Set-Cookie：`http.ParseSetCookie` 是 Go 1.23 才有的标准库 API，1.20 下编不过

## [0.2637.3] - 2026-09-12

### 修复

- `FolderList` / `RecycleList` 的 `Info` 改 `RawMessage`：成功响应 `info` 是数组，按字符串解会失败

## [0.2637.2] - 2026-09-12

### 修复

- 直链解析适配蓝奏 ajax 端点变更：`/ajaxm.php` → `/ajaxfile.php`

## [0.2637.1] - 2026-09-12

### 变更

- 仓库并入 pan-go 多模块统一仓库，import 路径变更为
  `github.com/langhuachuanshi/pan-go/lanzou`（破坏性）
- 模块重构为 invoker + Service 子包结构（account/file/folder/upload/download/recycle/resolve），
  调用方式改为 `c.Files().List(ctx, ...)` 风格，业务方法统一带 ctx（破坏性）；
  HTTP 执行迁移到 `pan-go/core`（httpx/invoker），哨兵错误与挑战参数用法不变

## [0.3.2] - 2026-09-12

### 新增

- `GetCookieString()`：导出当前会话 cookie（`"k1=v1; k2=v2"` 单行格式），账号密码 `Login`
  或注入浏览器 Cookie 后可持久化会话，下次 `SetCookiesFromMap` 免登恢复

## [0.3.1] - 2026-09-10

### 修复

- **登录协议迁移**: 蓝奏云已把账号系统从 `pc.woozooo.com/account/loginajax` 迁至
  `accounts.woozooo.com`，旧接口返回 404 HTML 导致 `Login()` 必报 `invalid login response`。
  新流程（与新版登录页 `uselogin()` 逆向对齐）：请求新登录页自动过 `acw_sc__v2` JS 挑战
  （挑战算法/置换表/XORKey 均未变，复用 `fetchPageWithChallenge`）→ `POST /accounts.php`
  （`task=uselogin`，字段 `username`/`password`/`ref`）→ 成功后跟随响应 `msgs` 中的中转
  鉴权跳转链（≤5 跳）把登录态 cookie 落到最终域。`zt` 兼容数字/字符串两种返回。
  实测假账号返回「用户名不正确」（旧版为 `invalid login response`），链路已通。

### 新增

- `account_test.go`：`ztIsOne`（zt 形态兼容）/ `followLoginRedirect`（跳转链 + Set-Cookie 吸收）单测

## [0.3.0] - 2026-07-18

### 新增

- `UploadFileWithProgress` 流式上传：`progressReader`（io.Reader 包装，边读边回调真实网络进度）
  + `postMultipartStream`（io.Pipe + multipart 直写 request body），原 `UploadFile` 保留向后兼容

## [0.2.0] - 2026-07-16

- 首个 tag：直链解析（含 acw_sc__v2 挑战自适应）、登录/账号、文件/文件夹、回收站、分享管理
