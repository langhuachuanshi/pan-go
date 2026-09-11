# AGENTS.md

## 项目概览

蓝奏云网盘 Go SDK，基于逆向协议实现，零第三方依赖（go.mod 无任何 require）。
模块名 `github.com/langhuachuanshi/lanzou-go`，所有代码在根包 `lanzou`。

## 目录结构

```
lanzou-go/
├── client.go           Client 主体：配置项、HTTP 收发、cookie 合并
├── config.go           域名常量、默认参数、ChallengeConfig（acw_sc__v2 挑战参数）
├── account.go          登录/登出/用户信息/帐号信息
├── resolve.go          直链解析（无需登录，含挑战自适应）
├── file.go             文件列表/分享链接/移动/删除/设密码
├── folder.go           文件夹列表/创建/删除/移动
├── upload.go           上传（一次性载入内存，保留兼容）
├── upload_stream.go    流式上传（io.Pipe + 进度回调）
├── download.go         文件/文件夹/直链下载
├── recycle.go          回收站
├── models.go           响应数据模型
├── errors.go           哨兵错误（errors.Is 判断）
├── doc.go              包文档
└── _example/           可运行示例（下划线开头，go 工具链忽略编译，需 go run 手动运行）
```

## 常用命令

```bash
go build ./...        # 编译（含测试）
go test ./...         # 单元测试（不依赖真实账号）
go vet ./...
go run ./_example/main.go   # 联调示例，需要真实蓝奏云账号
```

## 架构与约定

- **登录走新账号系统**：`accounts.woozooo.com`（`POST /accounts.php`，`task=uselogin`，
  成功后跟随 `msgs` 中转跳转链落登录态 cookie）。旧 `pc.woozooo.com/account/loginajax`
  已 404 废弃，不要往回改。
- **反爬挑战参数集中管理**：acw_sc__v2 的置换表/XORKey 在 `ChallengeConfig`
  （`config.go` 的 `DefaultChallengeConfig`）。蓝奏云换 JS 混淆时只更新参数，
  不改解析逻辑。
- **错误处理**：统一用 `errors.go` 的哨兵错误（`ErrPasswordWrong` 等），调用方
  `errors.Is` 判断；API 层错误包装为 `ErrAPIError`。
- **上传**：优先 `UploadFileWithProgress`（流式、进度回调）；`UploadFile` 保留向后兼容。
- **cookie 会话**：`SetCookies` / `SetCookiesFromMap` 注入，`GetCookieString` 导出
  持久化，下次免登恢复。
- **文档同步**：`README.md`（快速上手）、`API.md`（全量 API 参考）、`CHANGELOG.md`
  （Keep a Changelog 风格，条目与 git tag 版本对应——发 tag 前先补条目）。
  公开 API 变更时三处都要同步。

## 行为准则
以下准则偏向谨慎而非速度，目的是减少常见编码失误。

### 1. 先想清楚再动手
不要想当然。不要掩盖困惑。主动暴露取舍。
动手之前：
- 明确说出你的假设，不确定就问
- 如果有多种理解，列出来让用户选，不要自己悄悄选一个
- 如果存在更简单的方案，说出来。该反驳就反驳
- 遇到不清楚的地方，停下来。指出哪里不明白，然后问

### 2. 简单优先
用最少的代码解决问题。不要写猜测性的代码。
- 不加没要求的功能
- 一次性使用的代码不做抽象
- 没要求的"灵活性"或"可配置性"不加
- 不可能发生的场景不做错误处理
- 如果写了 200 行但 50 行就能搞定，重写
- 问自己："资深工程师会觉得这太复杂了吗？" 如果是，简化

### 3. 精准修改
只动该动的。只清理自己弄乱的。
编辑已有代码时：
- 不要"顺手改进"旁边的代码、注释或格式
- 不要重构没坏的东西
- 匹配已有风格，即使你习惯不同
- 发现无关的死代码，提一句就行，别删
你的改动产生的孤立代码：
- 删除你的改动导致不再使用的 import/变量/函数
- 不要删除之前就存在的死代码，除非被要求
检验标准：每一行改动都应该能追溯到用户的需求。

### 4. 目标驱动
定义成功标准，循环直到验证通过。
把任务转化为可验证的目标：
- "加校验" → "为无效输入写测试，然后让测试通过"
- "修 bug" → "写一个能复现的测试，然后让它通过"
- "重构 X" → "确保重构前后测试都能通过"
多步骤任务，简要列个计划：
1. [步骤] → 验证：[检查点]
2. [步骤] → 验证：[检查点]
3. [步骤] → 验证：[检查点]
