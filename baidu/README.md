# baidu

百度网盘网页端（抓包）Go SDK。基于 BDUSS + STOKEN cookie 鉴权，
支持文件管理、分片上传（秒传）、下载、分享/转存、离线下载、回收站、扫码登录。

## 安装

```sh
go get github.com/langhuachuanshi/pan-go/baidu@latest
```

## 获取凭证

- 浏览器登录 pan.baidu.com → F12 → Application → Cookies → 复制 `BDUSS` 与 `STOKEN`
- 或使用扫码登录：`baidu/qrcode`（getqrcode → unicast 轮询 → qrbdusslogin 换发 BDUSS/STOKEN）

## 快速开始

```go
import (
    "github.com/langhuachuanshi/pan-go/baidu"
    "github.com/langhuachuanshi/pan-go/baidu/file"
)

c, err := baidu.New(ctx, &baidu.Config{BDUSS: "...", STOKEN: "..."})
if err != nil { log.Fatal(err) }

files, err := c.Files().List(ctx, &file.ListRequest{Dir: "/", Num: 20})
for _, f := range files {
    if f.IsFolder() {
        fmt.Println("[文件夹]", f.Name())
    } else {
        fmt.Println("[文件]", f.Name(), f.Size)
    }
}
```

更多：`c.Management().MakeDir / Move / Rename / Delete`、`c.Upload().Upload`（秒传+分片）、
`c.Download()`、`c.Share().ShareSet / TransferSave`、`c.User().Quota / CheckLogin`。

接口约定（域名/公共参数/bdstoken）见模块 [AGENTS.md](./AGENTS.md)，
版本历史见 [CHANGELOG.md](./CHANGELOG.md)。
