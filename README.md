# quark-go

夸克网盘的 Go SDK。基于 cookie 鉴权（无 token、无签名），cookie 可扫码登录获取，也可从浏览器复制。

> 已完成全部核心功能（均实测通过）：扫码登录取 cookie、文件列表/详情/管理、上传（秒传+分片）、下载、创建分享、转存他人分享（核心场景）。

## 安装

```sh
go get github.com/langhuachuanshi/quark-go
```

## 获取 Cookie

获取方式二选一：

**方式一：扫码登录**（免手动抓包，完整示例见 `example/test_qrlogin`）：

```go
login, _ := qrcode.Create(ctx)
fmt.Println(login.QRURL)                    // 二维码内容短链，贴到任意二维码生成器出图
cookie, _ := login.Wait(ctx, 2*time.Minute) // 夸克App扫码确认后返回完整 cookie
```

**方式二：浏览器复制**：

1. 浏览器登录 https://pan.quark.cn
2. 按 F12 → Network 标签
3. 刷新页面，点击任意一个请求
4. 在 Request Headers 里找到 `Cookie:`，复制完整值（`__puus=xxx; __pus=yyy; ...`）
5. 保存到 `~/.quark/cookie.json`，内容 `{"cookie":"复制的完整cookie"}`

## 快速入门

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/langhuachuanshi/quark-go/quark"
    "github.com/langhuachuanshi/quark-go/quark/file"
)

func main() {
    ctx := context.Background()
    // 默认从 ~/.quark/cookie.json 读 cookie；也可 quark.WithCookie("...")
    c, err := quark.New(ctx)
    if err != nil { log.Fatal(err) }

    files, _ := c.Files().List(ctx, &file.ListRequest{PDirFID: "0"})
    for _, f := range files {
        fmt.Println(f.FileName, f.Size)
    }
}
```

## 目录结构

```
quark-go/
├── quark/          主包 Client（持 cookie，实现 invoker）
├── quark/auth/     cookie 管理 + 持久化
├── quark/file/     文件列表/详情/管理（建目录、重命名、移动、删除）
├── quark/share/    创建分享 + 转存 + 聚合发货
├── quark/upload/   上传（秒传 + 分片直传 OSS）
├── quark/download/ 下载
├── quark/qrcode/   扫码登录（换发登录 cookie）
├── quark/types/    数据模型
└── quark/invoker/  共享调用接口
```

## 功能

| 模块 | 方法 | 说明 |
|---|---|---|
| 文件 | `Files().List` / `ListPage` | 列目录（自动/手动分页） |
| 文件 | `Files().Get` | 文件详情 |
| 文件 | `Files().MakeDir` / `Rename` / `Move` / `Delete` | 文件管理（移动/删除天然批量） |
| 上传 | `Upload().Upload` | 秒传 + 分片直传 OSS，流式（`io.ReaderAt`），支持进度回调 |
| 下载 | `Download().Download` / `GetDownloadURL` | 流式下载（`io.Writer`）/ 拿临时直链 |
| 扫码登录 | `qrcode.Create` / `Login.Wait` | 生成二维码内容短链并轮询换发登录 cookie |
| 分享 | `Share().Create` | 创建公开/私密分享（含提取码） |
| 转存 | `Share().Transfer` | 一键转存他人分享，返回新 fid 列表 |
| 转存 | `Share().GetShareToken` / `ListShareFiles` / `SaveShare` / `WaitTask` | 转存子步骤（高级用法） |
| 聚合发货 | `Share().DeliverByShareURLs` | 多个商品分享 → 一个限时带码新分享（每订单临时发货） |
| 聚合发货 | `Share().ResolveFIDs` | 从自己的分享反查 FID（批量回填商品库/高级用法） |

## 夸克 vs 阿里云盘

| 维度 | 阿里云盘 | 夸克网盘 |
|---|---|---|
| 鉴权 | token（扫码） | cookie（浏览器复制） |
| 签名 | 需 x-signature | 不需要 |
| 核心场景 | 上传+分享 | **转存他人分享** |

## License

仅供学习交流。
