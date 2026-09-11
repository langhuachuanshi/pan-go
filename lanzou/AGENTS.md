# lanzou 模块约定

> 公共规范（文档与计划 / TODO / 更新日志 / Tag / 提交与发版 / 行为准则）见仓库根 `AGENTS.md`，本文件只写本模块特有内容。

## 模块概览

蓝奏云 Go SDK，逆向协议实现，零第三方依赖。全量 API 参考：`docs/lanzou/API.md`；
版本历史：`docs/lanzou/CHANGELOG.md`。

## 目录结构

```
lanzou/            # 根包平铺（module 根即包 lanzou）
├── client.go      # Client 主体：配置项、HTTP 收发、cookie 合并
├── config.go      # 域名常量、默认参数、ChallengeConfig（acw_sc__v2 挑战参数）
├── account.go     # 登录/登出/用户信息/帐号信息
├── resolve.go     # 直链解析（无需登录，含挑战自适应）
├── file.go / folder.go / download.go / recycle.go
├── upload.go      # 上传（一次性载入内存，兼容保留）
├── upload_stream.go # 流式上传（io.Pipe + 进度回调）
├── models.go / errors.go（哨兵错误，errors.Is 判断）/ doc.go / utils.go
```

## 接口约定

- **登录走新账号系统**：`accounts.woozooo.com`（POST /accounts.php，task=uselogin，
  成功后跟随 msgs 中转跳转链落登录态 cookie）。旧 `pc.woozooo.com/account/loginajax`
  已 404 废弃，不要往回改。
- **反爬挑战参数集中管理**：acw_sc__v2 置换表/XORKey 在 `ChallengeConfig`
  （config.go 的 `DefaultChallengeConfig`）。蓝奏云换 JS 混淆时只更新参数，不改解析逻辑。
- **错误处理**：哨兵错误（ErrPasswordWrong 等）+ `errors.Is`；API 层错误包装为 ErrAPIError。
- **cookie 会话**：`SetCookies` / `SetCookiesFromMap` 注入，`GetCookieString` 导出持久化。

## 常用命令

```bash
cd lanzou && go build ./... && go vet ./... && go test ./...
```

## 模块特有规矩

- 上传优先 `UploadFileWithProgress`（流式、进度回调）；`UploadFile` 仅向后兼容保留。
- `GetDurlByURLAndFolder` 的 folderID 参数当前被忽略（见审查报告 P1-1），不要依赖。
- 已知问题与修复计划见 `docs/reviews/2026-09-12-lanzou.md`。
