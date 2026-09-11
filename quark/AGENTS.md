# quark 模块约定

> 公共规范（文档与计划 / TODO / 更新日志 / Tag / 提交与发版 / 行为准则）见仓库根 `AGENTS.md`，本文件只写本模块特有内容。

## 模块概览

夸克网盘 Go SDK。cookie 鉴权（无 token、无签名）+ 扫码登录（quark/qrcode，换发登录 cookie）。
**查接口 / 补功能 / 查 bug 先看 `docs/quark/INTERFACE.md`**（接口参考索引，含与
AList / quark-auto-save 的对照矩阵和已知差异）。

## 目录结构

```
quark/            # 主包：Client（实现 invoker.Invoker）+ Option
├── auth/         # cookie 管理 + 持久化（~/.quark/cookie.json）
├── file/         # 文件列表/详情/管理
├── share/        # 创建分享 + 转存 + 聚合发货
├── upload/       # 上传（秒传 + 分片直传 OSS）
├── download/     # 下载
├── qrcode/       # 扫码登录（uop.quark.cn CAS 换发 cookie）
├── types/        # 数据模型
└── invoker/      # 共享调用接口与 APIError
```

## 接口约定

- base `https://drive-pc.quark.cn/1/clouddrive`，公共 query `pr=ucpro&fr=pc`
  （`uqm` 会被当游客返回 31001）。
- 响应 `{status, code, message, ...}`，`code==0 && status==200` 才算成功；
  错误统一 `invoker.APIError`，登录态失效用 `invoker.IsAuthError`（31001/31003）。
- cookie 校验：`__puus` 或 `__pus` 任一存在即有效（扫码换发的只有 `__pus`，
  drive 接口实测认）；响应刷新的 `__puus` 要合并回本地 cookie。

## 常用命令

```bash
cd quark && go build ./... && go vet ./... && go test ./...
```

## 模块特有规矩

- 创建分享：提取码第 1 步就传；无密码也必须调 `/share/password`，否则链接"已删除"；
  文件夹分享是异步任务，需轮询 `/task` 拿 share_id。
- 转存缺反风控参数 `__dt`/`__t`、缺 `__puus` 保活——已知风险跟踪在
  `docs/quark/INTERFACE.md`「关键差异与潜在问题」。
- 已知问题与修复计划见 `docs/reviews/2026-09-12-quark.md`。
