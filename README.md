# pan-go

四个网盘的 Go SDK 统一仓库（monorepo）：**阿里云盘 / 百度网盘 / 蓝奏云 / 夸克网盘**。
每个网盘是独立 Go module：独立 go.mod、独立版本（tag 带网盘前缀），互不依赖。

## 模块

| 目录 | 网盘 | 鉴权模型 | 第三方依赖 | import 前缀 |
|---|---|---|---|---|
| `alipan/` | 阿里云盘 | token（扫码 / refresh_token）+ secp256k1 设备签名 | 有（go-qrcode、x/text、secp256k1） | `github.com/langhuachuanshi/pan-go/alipan` |
| `baidu/` | 百度网盘 | BDUSS + STOKEN cookie（网页端抓包） | 零 | `github.com/langhuachuanshi/pan-go/baidu` |
| `lanzou/` | 蓝奏云 | cookie + acw_sc__v2 反爬挑战 | 零 | `github.com/langhuachuanshi/pan-go/lanzou` |
| `quark/` | 夸克网盘 | cookie + 扫码登录（quark/qrcode） | 零 | `github.com/langhuachuanshi/pan-go/quark` |

## 安装（以夸克为例，其余同理换路径）

```sh
go get github.com/langhuachuanshi/pan-go/quark@latest
```

```go
import (
    "github.com/langhuachuanshi/pan-go/quark"
    "github.com/langhuachuanshi/pan-go/quark/file"
)

c, _ := quark.New(ctx, quark.WithCookie("..."))
files, _ := c.Files().List(ctx, &file.ListRequest{PDirFID: "0"})
```

各模块的使用要点、接口约定与红线见对应目录的 `AGENTS.md` 与 `docs/<模块>/`。

## 开发

```sh
go build all          # 仓库根：工作区全量构建（依赖 go.work）

# 质量门：逐模块（go vet/test all 会连带第三方依赖自己的测试，不作门禁）
for m in alipan baidu lanzou quark; do (cd $m && go vet ./... && go test ./...); done
```

模块与模块之间不互相 import；跨模块复用靠各自实现 invoker 模式，不建公共包。

## 文档

| 位置 | 内容 |
|---|---|
| `AGENTS.md`（根） | 公共规范：文档与计划 / TODO / 更新日志 / Tag / 提交与发版 / 行为准则 |
| `<模块>/AGENTS.md` | 各网盘单独规范：概览、接口约定、特有规矩 |
| `docs/plans/` `docs/reviews/` | 开发计划 · 代码审查报告 |
| `docs/lanzou/` | API.md · CHANGELOG.md |
| `docs/quark/` | INTERFACE.md（接口参考索引） |

仅供学习交流。
