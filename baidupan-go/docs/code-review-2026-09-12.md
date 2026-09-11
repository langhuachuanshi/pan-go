# 代码审查报告 2026-09-12

- **范围**：`baidu/` 全部源码（client/auth/file/management/upload/download/share/clouddl/user/qrcode/panhome/sign/types/invoker）+ `example/`
- **方法**：核心包源码通读；`go build` / `go vet` / `go test`（baidu、baidu/upload、baidu/user 三包测试均 ok）；死代码以全仓 grep 零引用为准；gofmt / 行尾检查
- **分级**：P0 = 正确性/安全问题 · P1 = 应尽快处理 · P2 = 建议
- **状态标注**：☑ 本轮已修复 · ☐ 待修复（见修复计划）

## 结论摘要

架构与 quark-go 同款（invoker 接口模式），接口约定有实测注释支撑，
无 P0 问题。主要问题：下载 API 要求调用方自备带 cookie 的 http.Client
（易误用）、若干疑似闲置公开 API 与死参数。另核实：根目录的
`test_qrdiag.exe` **从未被 git 跟踪**（`git ls-files` 无、全历史无提交），
仅是本地遗留文件，已被既有 `*.exe` 规则忽略，无需出库。

## 已修复（本轮）

- ☑ **P2 · `.gitignore` 增补**：补 `.env.*`、`*_cookie.txt`
  （示例读取的 `baidu_cookie.txt` 若落在仓库内会被拦）、`cookie.json/txt`、
  `*.log`、`tmp/`、OS 杂项（.DS_Store/Thumbs.db）。

> 勘误：早前曾认为 `test_qrdiag.exe` 已入库，经 `git ls-files` 与全历史
> 核查，该文件从未被跟踪，本条撤销。

## 发现

### P1

1. **下载 API 的 httpClient 依赖外置**：`Download` / `DownloadStream`
   （download.go:96/187）要求调用方传入「携带 BDUSS cookie 的
   http.Client」，而主包 `Client` 自持配置好 cookiejar 的 client
   （client.go:88）却不复用——调用方极易传错导致 403/未登录，示例也只能
   另行构造。方案：Service 记住主包 client 作为默认值，参数保留为覆盖项；
   或至少在 godoc 与 README 给出可直接复制的正确用法。
2. **测试空白区**：现有测试集中在 management-mkdir / upload 进度回调 /
   user 登录态判断三处；`file` / `share` / `clouddl` / `download` 主路径
   无测试。真机依赖的部分难测，但纯函数（`buildQuery`、`sign.Sign2`、
   `types.File` 判定、错误映射）可先补。验收：上述纯函数各有表驱动测试。

### P2

1. **死参数**：`management.go:211` `filemanager(ctx, opera, items,
   _ map[string]string)`——第 4 个参数匿名且 3 个调用方（copy/move/rename）
   全部传 nil，应删除。
2. **疑似闲置公开 API**：`download.GetPCSLocateURL`（download.go:292）与
   `sign.LocateDownloadSign` / `sign.DevUID` 仓库内零调用（唯一出现是
   `example/test_download/main.go:38` 的一行打印文案）。多节点下载场景若
   无规划，建议标记 Deprecated 或移除，避免维护面虚增。
3. **`SetPanHomeCache` 接线靠调用方自觉**（download.go:44）：仓库内无调用
   方，忘接时 PanAPI 下载会静默回退 PCS 直链（GetDownloadURL 有 nil 判断，
   行为正确但不可见）。建议：client 初始化时自动接线，或在 godoc/示例中
   给出完整演示（现示例只打印了一行提示）。
4. **非惯用写法**：`client.go:88` `Timeout: 60 * 1e9` 应为
   `60 * time.Second`（quark-go 同款问题）。
5. **行尾无规范**：无 `.gitattributes`，Windows 检出全量 CRLF，`gofmt -l`
   对全部 .go 文件误报（diff 仅为行尾）。

## 修复计划

- [ ] P1-1 下载 Service 复用主包 http.Client 作默认值（保留参数覆盖），补示例
- [ ] P1-2 为 buildQuery / Sign2 / types.File / 错误映射补表驱动单测
- [ ] P2-1 删除 `filemanager` 死参数
- [ ] P2-2 评估 GetPCSLocateURL / LocateDownloadSign / DevUID 去留并标注
- [ ] P2-3 PanHome 缓存自动接线或示例补全
- [ ] P2-4 `60 * 1e9` → `60 * time.Second`
- [ ] P2-5 加 `.gitattributes`（`* text=auto`，`*.go` `*.md` 强制 LF）后全量 `gofmt`
