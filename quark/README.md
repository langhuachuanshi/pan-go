# quark

夸克网盘 Go SDK。基于 cookie 鉴权（无 token、无签名），支持扫码登录、
文件管理、上传（秒传+分片）、下载、创建分享、转存他人分享（核心场景）、聚合发货。

## 安装

```sh
go get github.com/langhuachuanshi/pan-go/quark@latest
```

## 获取 Cookie

方式二选一：

**方式一：扫码登录**（免手动抓包）

```go
import "github.com/langhuachuanshi/pan-go/quark/qrcode"

login, _ := qrcode.Create(ctx)
fmt.Println(login.QRURL)                    // 二维码内容短链，贴到任意二维码生成器出图
cookie, _ := login.Wait(ctx, 2*time.Minute) // 夸克App扫码确认后返回完整 cookie
```

**方式二：浏览器复制**

浏览器登录 https://pan.quark.cn → F12 → Network → 任意请求 → 复制 `Cookie:` 完整值
（`__puus=xxx; __pus=yyy; ...`），保存到 `~/.quark/cookie.json`（内容 `{"cookie":"..."}`）。

## 快速开始

```go
import (
    "github.com/langhuachuanshi/pan-go/quark"
    "github.com/langhuachuanshi/pan-go/quark/file"
)

// 默认从 ~/.quark/cookie.json 读 cookie；也可 quark.WithCookie("...")
c, err := quark.New(ctx)
if err != nil { log.Fatal(err) }

files, _ := c.Files().List(ctx, &file.ListRequest{PDirFID: "0"})
for _, f := range files {
    fmt.Println(f.FileName, f.Size)
}
```

核心场景：转存他人分享 `c.Share().Transfer(...)`；创建分享 `c.Share().Create(...)`。

接口协议真相与已知差异见 [INTERFACE.md](./INTERFACE.md)，模块约定见
[AGENTS.md](./AGENTS.md)，版本历史见 [CHANGELOG.md](./CHANGELOG.md)。
