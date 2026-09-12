# lanzou 模块约定

> 公共规范（文档与计划 / TODO / 更新日志 / Tag / 提交与发版 / 行为准则）见仓库根 `AGENTS.md`，本文件只写本模块特有内容。

## 模块概览

蓝奏云 Go SDK，逆向协议实现，零第三方依赖。全量 API 参考：`docs/lanzou/API.md`；
版本历史：`docs/lanzou/CHANGELOG.md`。

## 目录结构

```
lanzou/            # module 根 = 主包 lanzou（会话生命周期 / 挑战求解 / invoker 实现）
├── invoker/       # 共享调用接口 + 哨兵错误定义（设计同其他三模块）
├── account/       # 用户信息 / 帐号详情（Login/Logout 在主包 Client）
├── file/          # 列表 / 分享链接 / 移动 / 删除 / 设密码
├── folder/        # 列表 / 创建 / 删除 / 移动
├── upload/        # 本地文件 File / 流式进度 Stream / 网盘转存 ByURL
├── download/      # 文件 File / 自动命名 FileAuto / 文件夹递归 Dir / 直链 ByURL
├── recycle/       # 回收站
├── resolve/       # 直链解析（无需登录）
├── client.go      # Client + Login/Logout + cookie 会话 + invoker 实现
├── http.go        # HTTP 收发底层（get/post/multipart/cookie 合并）
├── challenge.go   # acw_sc__v2 挑战求解
├── stream.go      # 流式 multipart 上传（progressReader）
├── config.go      # 域名/端点/默认参数/挑战默认值
└── errors.go      # 哨兵错误别名（定义在 invoker）
```

## 接口约定

- **登录走新账号系统**：`accounts.woozooo.com`（POST /accounts.php，task=uselogin，
  成功后跟随 msgs 中转跳转链落登录态 cookie）。旧 `pc.woozooo.com/account/loginajax`
  已 404 废弃，不要往回改。登录登出是会话生命周期，在主包 `Client.Login/Logout`。
- **反爬挑战参数集中管理**：acw_sc__v2 置换表/XORKey 在 `ChallengeConfig`
  （config.go 的 `DefaultChallengeConfig`）。蓝奏云换 JS 混淆时只更新参数，不改解析逻辑。
- **错误处理**：哨兵错误（ErrPasswordWrong 等）+ `errors.Is`；API 层错误包装为 ErrAPIError。
- **HTTP 执行走 core**：主包 Client 实现 core/invoker 标准菜单（执行层=core/httpx），
  本模块只保留 cookie 会话与挑战页方言；业务方法统一带 ctx。
- **cookie 会话**：`SetCookies` / `SetCookiesFromMap` 注入，`GetCookieString` 导出持久化。
- **上传**：优先 `Upload().Stream`（流式、进度回调）；`Upload().File` 仅向后兼容保留。

## 常用命令

```bash
cd lanzou && go build ./... && go vet ./... && go test ./...
```

## 模块特有规矩

- `Resolve().GetDurlByURLAndFolder` 的 folderID 参数当前被忽略（审查报告 P1-1），不要依赖。
- 已知问题与修复计划见 `docs/reviews/2026-09-12-lanzou.md`（位于仓库根 docs/）。
