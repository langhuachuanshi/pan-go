# lanzou-go

> 蓝奏云网盘 Go SDK，基于逆向协议实现，零第三方依赖。

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](./LICENSE)

## 安装

```bash
go get github.com/langhuachuanshi/lanzou-go
```

## 快速开始

```go
import "github.com/langhuachuanshi/lanzou-go"

client := lanzou.NewClient()

// 无需登录：解析分享链接获取直链
durl, _ := client.GetDurlByURL("https://pan.lanzoul.com/xxxxx", "")

// 登录后：管理文件
client.Login("user", "pass")
files, _ := client.GetFileList(-1)
for _, f := range files.Text {
    fmt.Printf("[%s] %s (%s)\n", f.ID, f.NameAll, f.Size)
}
```

## 功能

| 模块 | 功能 |
|------|------|
| 🔗 直链解析 | 分享链接 → 真实下载直链（无需登录） |
| 👤 账号 | 登录/登出、用户信息、容量查询、Cookie 导出/恢复 |
| 📁 文件 | 列表、上传（含流式进度上传）、下载、删除、移动、设密码、获取分享链接 |
| 📂 文件夹 | 创建、删除、移动、列表 |
| 🗑️ 回收站 | 移入、列表、恢复、清空 |

## 配置

```go
client := lanzou.NewClient(
    lanzou.WithTimeout(30),                    // HTTP 超时
    lanzou.WithMaxSize(100 * 1024 * 1024),     // 单文件大小限制
    lanzou.WithMaxDownloadCount(5),            // 并发下载数
    lanzou.WithChallengeConfig(&...),          // 反爬参数（蓝奏云换混淆时更新）
)
```

## 错误处理

```go
durl, err := client.GetDurlByURL(url, pwd)
if errors.Is(err, lanzou.ErrPasswordWrong) {
    // 密码错误
}
```

## 蓝奏云换混淆时如何更新

蓝奏云每年会更换几次 JS 混淆方式，导致直链解析失败。只需更新挑战参数：

```go
client.SetChallengeConfig(&lanzou.ChallengeConfig{
    Perm:   [40]int{/* 从新版JS提取的置换表 */},
    XORKey: "/* 从新版JS提取的XOR密钥 */",
})
```

不需要升级 SDK 版本。

## 📖 完整 API 文档

详见 [API.md](./API.md)

## 参考

- [zaxtyson/LanZouCloud-API](https://github.com/zaxtyson/LanZouCloud-API) — Python 原版
- [iuroc/go-lanzou](https://github.com/iuroc/go-lanzou) — Go 直链解析参考

## License

MIT
