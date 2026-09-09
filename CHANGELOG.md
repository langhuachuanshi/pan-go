# Changelog

All notable changes to lanzou-go will be documented in this file.

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
