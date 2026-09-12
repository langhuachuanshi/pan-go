# core 模块约定

> 公共规范（文档与计划 / TODO / 更新日志 / Tag / 提交与发版 / 行为准则）见仓库根 `AGENTS.md`，本文件只写本模块特有内容。

## 模块概览

pan-go 的公共基础层：HTTP 执行器（httpx）、统一错误语义（errors）、统一 Invoker
接口（invoker）、新网盘接入指南（template）。**零业务、零网盘知识**——任何网盘
的方言（鉴权方式、错误码表、端点）都不进 core。

## 目录结构

```
core/
├── httpx/      # HTTP 执行器：Request/Response、JSONBody/FormBody/RawBody、重试、日志
├── invoker/    # 业务子包依赖的统一调用接口（"标准菜单"），各模块主包 Client 实现
├── errors/     # 统一 APIError + Kind 语义（Auth/RateLimited/NotFound/Denied）+ Is 判定
└── template/   # 新网盘接入指南（文档）
```

## 接口约定

- httpx 只管执行，不管鉴权与业务判错：鉴权头由模块放进 `Request.Headers`；
  HTTP 非 2xx **不是错误**，原样返回 Response 由调用方判语义。
- `Body.Retries > 0` 时 `Encode()` 会被多次调用，实现需保证可重复读。
- invoker 契约（详见 package doc）：path 相对模块 base（`http(s)://` 开头=完整 URL）；
  HTTP ≥400 由实现方转成 `*coreerrors.APIError`（Status 填充）；成功解 JSON 到 out；
  **业务码判定是模块方言，留在业务子包**，构造 `*coreerrors.APIError` 时带 Kind。
- import 惯例：标准库 errors 冲突，本包别名 `coreerrors`。

## 常用命令

```bash
cd core && go build ./... && go vet ./... && go test ./...
```

## 模块特有规矩

- **克制红线**：core 只收"各网盘都在重复写、且差异能用参数描述"的逻辑。
  新想法先进 `docs/plans/` 评审，不直接加代码（文件模型接口、分片上传骨架
  均已裁决推迟/不做，见 core 架构计划决议 3/6）。
- 新增公开 API 必须带单测；httpx 的行为变更必须先改/加 httptest 用例。
