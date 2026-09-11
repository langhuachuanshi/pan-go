# 代码审查报告 2026-09-12

- **范围**：根包 `lanzou` 全部源码（client/account/file/folder/upload/upload_stream/download/recycle/resolve/utils/config/models/errors/doc）+ `_example/`
- **方法**：源码通读；`go build` / `go vet` / `go test`（均通过）；死代码以全仓 grep 零引用为准；gofmt / 行尾检查
- **分级**：P0 = 正确性/安全问题 · P1 = 应尽快处理 · P2 = 建议
- **状态标注**：☑ 本轮已修复 · ☐ 待修复（见修复计划）

## 结论摘要

代码质量整体良好：哨兵错误统一、挑战参数集中可配置、上传新旧双轨清晰、
已有 account/upload_stream 单测。未发现 P0。主要问题集中在**少量死代码、
两个语义不符的公开 API、工程化配套缺失（.gitignore/.gitattributes）**。

## 已修复（本轮）

- ☑ **P1 · 仓库无 `.gitignore`**：已新建（构建产物、IDE、OS、本地 cookie/账号凭据）。

## 发现

### P1

1. **`GetDurlByURLAndFolder` 的 `folderID` 参数被忽略**（resolve.go:67-70）：
   函数体直接 `return c.GetDurlByURL(shareURL, pwd)`，签名承诺的「带文件夹
   参数」并未实现，误导调用方。API.md 已如实写「等价于 GetDurlByURL」，
   但参数仍在。处置：标记 `Deprecated` 并注明，或真正实现；二选一，
   不应维持现状。

### P2

1. **死常量**：`config.go:12` `pathLogin = "/account/loginajax"` 全仓零引用，
   注释已标「已废弃」，历史信息可由 git 与 baseURLAccount 注释承载，应删除。
2. **纯别名公开 API**：`file.go:44` `GetFileInfoByURL` 仅转发 `GetFileInfo`。
   与 Python 原版命名兼容可保留，但应加注释说明不新增行为（现注释已写
   「委托给」，建议补 `Deprecated:` 引导或保持现状并知悉）。
3. **godoc 代码块损坏**：`doc.go:25` 行首多了一个 tab（`	//	files, ...`），
   该行脱离注释块对齐，godoc 渲染的示例代码会多出杂散 `//`。
4. **cookie 顺序不稳定**：`client.go mergeCookies`（client.go:205）用 map
   去重后无序输出，`GetCookieString` 每次顺序可能不同，不利于持久化对比
   与测试断言。可按首次出现顺序稳定排序。
5. **行尾无规范**：无 `.gitattributes`，Windows 检出全量 CRLF，`gofmt -l`
   对所有文件误报（本次核查全部 .go 文件均被标记，diff 仅为行尾）。
6. **测试空白区**：`resolve`（含 solveAcwScV2 纯函数）与 `download` 无测试。
   `solveAcwScV2`（utils.go:19）输入输出确定，最易补表驱动测试。

## 修复计划

- [ ] P1-1 处置 `GetDurlByURLAndFolder`：实现 folderID 或标 Deprecated（同步 API.md）
- [ ] P2-1 删除 `pathLogin` 死常量
- [ ] P2-2 `GetFileInfoByURL` 注释明确「兼容别名，无新增行为」
- [ ] P2-3 修复 `doc.go:25` 前导 tab
- [ ] P2-4 `mergeCookies` 保持首次出现顺序，稳定 `GetCookieString` 输出
- [ ] P2-5 加 `.gitattributes`（`* text=auto`，`*.go` `*.md` 强制 LF）后全量 `gofmt`
- [ ] P2-6 为 `solveAcwScV2` 补表驱动单测
