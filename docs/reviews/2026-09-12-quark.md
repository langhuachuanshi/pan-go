# 代码审查报告 2026-09-12

- **范围**：`quark/` 全部源码（client/auth/file/share/upload/download/qrcode/types/invoker）+ `example/`
- **方法**：核心包源码通读；`go build` / `go vet` / `go test`；死代码以全仓 grep 零引用为准；gofmt / 行尾检查
- **分级**：P0 = 正确性/安全问题 · P1 = 应尽快处理 · P2 = 建议
- **状态标注**：☑ 本轮已修复 · ☐ 待修复（见修复计划）

## 结论摘要

整体架构清晰（invoker 接口模式解耦业务子包），无 P0 问题。主要短板：
**全仓零单元测试**、入库了 5 个一次性调试脚本、若干死代码/占位代码。
协议层的已知缺口（__puus 保活、转存反风控参数等）已在 `INTERFACE.md`
跟踪，本报告不重复。

## 已修复（本轮）

- ☑ **P1 · 报错文案与校验规则不符**：`quark/client.go:70` 原文「缺少 __puus」，
  但 `auth.IsValid`（auth.go:102）已放宽为 `__puus`/`__pus` 任一即可
  （扫码换发的 cookie 只有 `__pus`）。已改为「须含 __puus 或 __pus，
  请扫码登录或从浏览器复制」。
- ☑ **P1 · `.gitignore` 缺 `.env` 与日志**：已增补（见仓库根 .gitignore）。

## 发现

### P1

1. **全仓零单元测试**（`go test ./...` 无任何 `_test.go`）。
   大量纯逻辑不依赖真机即可测：`invoker.IsAuthError` / `APIError.Error`、
   分片区间划分（upload.go）、`genPasscode` 字符集与长度（create.go:15）、
   `auth.Load` 的三级回退与 cookie.json 解析、`qrcode.newUUID` 格式。
   验收：上述至少各 1 个表驱动测试，`go test ./...` 通过。
2. **`example/diag`、`diag2`~`diag5` 为一次性调试脚本入库**（文件头即写明
   「dump 响应 JSON 找字段名」等）。其结论已沉淀进 `INTERFACE.md`，脚本
   本身无复用价值且拉低 example/ 信噪比。
   验收：删除，或移入 `docs/research/` 归档并注明用途。

### P2

1. **死字段**：`quark/client.go:54` `cookieMu sync.Mutex` 全仓零引用。
2. **占位保 import**：`quark/client.go:184` `var _ = types.File{}` 仅为
   保住 types import，主包并无真实使用；应删 import + 该行。
3. **非惯用写法**：`quark/client.go:75` `Timeout: 60e9` 应为
   `60 * time.Second`。
4. **零调用公开 API**：`auth.DeleteCookieFile`（auth.go:62）仓库与示例均无
   调用。公开 API 删除属破坏性变更，需先评估外部使用再决定去留。
5. **错误忽略**：`quark/share/create.go:18` `rand.Read(b)` 返回值未检查
   （crypto/rand 实际不会失败，属习惯问题）。
6. **行尾无规范**：仓库无 `.gitattributes`，Windows 检出为 CRLF 导致
   `gofmt -l` 全量误报（含本次修改前的大部分文件）。应加
   `*.go text eol=lf` 统一。

## 修复计划

- [ ] P1-1 补核心纯逻辑单元测试（IsAuthError / genPasscode / 分片划分 / auth.Load / newUUID）
- [ ] P1-2 清理 example/diag~diag5（删除或归档至 docs/research/）
- [ ] P2-1 删除 `cookieMu` 死字段
- [ ] P2-2 删除 `var _ = types.File{}` 占位及 types import
- [ ] P2-3 `Timeout: 60e9` → `60 * time.Second`
- [ ] P2-4 评估 `auth.DeleteCookieFile` 去留（先查外部引用）
- [ ] P2-5 `genPasscode` 检查 `rand.Read` 错误
- [ ] P2-6 加 `.gitattributes`（`* text=auto`，`*.go` `*.md` 强制 LF）后全量 `gofmt`
