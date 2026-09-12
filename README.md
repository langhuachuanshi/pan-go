# pan-go

四个网盘的 Go SDK 统一仓库（monorepo）：**阿里云盘 / 百度网盘 / 蓝奏云 / 夸克网盘**。
每个网盘是独立 Go module：独立 go.mod、独立版本（tag 带网盘前缀），互不依赖。
本 README 只做导航；用法与文档在各模块目录内。

## 模块

| 目录 | 网盘 | 鉴权模型 | 第三方依赖 | import 前缀 |
|---|---|---|---|---|
| [`core/`](./core) | 公共基础层（执行器/错误语义/菜单） | — | 零 | `github.com/langhuachuanshi/pan-go/core` |
| [`alipan/`](./alipan) | 阿里云盘 | token（扫码 / refresh_token）+ secp256k1 设备签名 | 有（go-qrcode、x/text、secp256k1） | `github.com/langhuachuanshi/pan-go/alipan` |
| [`baidu/`](./baidu) | 百度网盘 | BDUSS + STOKEN cookie（网页端抓包） | 零 | `github.com/langhuachuanshi/pan-go/baidu` |
| [`lanzou/`](./lanzou) | 蓝奏云 | cookie + acw_sc__v2 反爬挑战 | 零 | `github.com/langhuachuanshi/pan-go/lanzou` |
| [`quark/`](./quark) | 夸克网盘 | cookie + 扫码登录（quark/qrcode） | 零 | `github.com/langhuachuanshi/pan-go/quark` |

## 每个模块目录内

| 文件 | 内容 |
|---|---|
| `README.md` | 快速上手（安装 / 获取凭证 / 最小示例） |
| `API.md` | 接口文档（api-docs 八段式，陆续补全） |
| `CHANGELOG.md` | 版本历史 |
| `AGENTS.md` | 模块约定：目录结构 / 接口约定 / 特有规矩 |

## 开发

```sh
go build all          # 仓库根：工作区全量构建（依赖 go.work）

# 质量门：逐模块（go vet/test all 会连带第三方依赖自己的测试，不作门禁）
for m in alipan baidu lanzou quark; do (cd $m && go vet ./... && go test ./...); done
```

模块与模块之间不互相 import；跨模块复用靠各自实现 invoker 模式，不建公共包。

## 仓库级文档

| 位置 | 内容 |
|---|---|
| `AGENTS.md`（根） | 公共规范：文档与计划 / TODO / 更新日志 / Tag / 提交与发版 / 注释 / 行为准则 |
| `TODO.md` | 任务索引 |
| `docs/plans/` `docs/reviews/` | 开发计划 · 代码审查报告 |
| `.agents/skills/api-docs/` | 接口文档写作技能（写 API.md 用） |

仅供学习交流。
