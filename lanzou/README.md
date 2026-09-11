# lanzou

蓝奏云网盘 Go SDK（基于逆向协议，零第三方依赖）。支持直链解析（无需登录）、
账号登录、文件/文件夹管理、上传（含流式进度）、下载、回收站。

## 安装

```sh
go get github.com/langhuachuanshi/pan-go/lanzou@latest
```

## 快速开始

```go
import "github.com/langhuachuanshi/pan-go/lanzou"

c := lanzou.NewClient()

// 无需登录：解析分享链接获取直链
durl, _ := c.Resolve().GetDurlByURL("https://pan.lanzoul.com/xxxxx", "")

// 登录后管理文件
c.Login("user", "pass")
files, _ := c.Files().List(-1)          // 根目录传 -1
for _, f := range files.Text {
    fmt.Printf("[%s] %s (%s)\n", f.ID, f.NameAll, f.Size)
}
folders, _ := c.Folders().List(-1)
c.Upload().Stream("local.txt", -1, nil)  // 流式上传 + 进度回调
c.Download().File("local.bin", "https://pan.lanzoul.com/xxxxx")
```

## 配置

```go
c := lanzou.NewClient(
    lanzou.WithTimeout(30),                    // HTTP 超时
    lanzou.WithMaxSize(100 * 1024 * 1024),     // 单文件大小限制
    lanzou.WithMaxDownloadCount(5),            // 并发下载数
    lanzou.WithChallengeConfig(&...),          // 反爬参数（蓝奏云换混淆时更新）
)
```

## 错误处理

```go
durl, err := c.Resolve().GetDurlByURL(url, pwd)
if errors.Is(err, lanzou.ErrPasswordWrong) {
    // 密码错误
}
```

## 蓝奏云换混淆时如何更新

蓝奏云每年会更换几次 JS 混淆方式，导致直链解析失败。只需更新挑战参数：

```go
c.SetChallengeConfig(&lanzou.ChallengeConfig{
    Perm:   [40]int{/* 从新版JS提取的置换表 */},
    XORKey: "/* 从新版JS提取的XOR密钥 */",
})
```

不需要升级 SDK 版本。完整 API 见 [API.md](./API.md)，版本历史见
[CHANGELOG.md](./CHANGELOG.md)，模块约定见 [AGENTS.md](./AGENTS.md)。

## 参考

- [zaxtyson/LanZouCloud-API](https://github.com/zaxtyson/LanZouCloud-API) — Python 原版
- [iuroc/go-lanzou](https://github.com/iuroc/go-lanzou) — Go 直链解析参考
