# alipan

阿里云盘（alipan / aliyundrive）Go SDK，对应 Python 库 [aligo](https://github.com/foyoux/aligo)
的核心子集 + 分享功能。token 鉴权（扫码 / refresh_token），按业务域子包组织，线程安全。

## 安装

```sh
go get github.com/langhuachuanshi/pan-go/alipan@latest
```

## 快速入门

```go
import (
    "github.com/langhuachuanshi/pan-go/alipan"
    "github.com/langhuachuanshi/pan-go/alipan/file"
)

// 默认读取 ~/.aligo/aligo.json 复用登录；不存在则终端扫码。
c, err := alipan.New(ctx)
if err != nil { log.Fatal(err) }

user, _ := c.Users().Get(ctx)
fmt.Println(user.UserName, user.NickName)

files, _ := c.Files().List(ctx, &file.ListRequest{
    ParentFileID: "root",
    DriveID:      c.DefaultDriveID(),
})
for _, f := range files {
    fmt.Println(f.Name, f.Size)
}
```

## 登录方式

```go
alipan.New(ctx, alipan.WithRefreshToken("xxxx")) // refresh_token 直登
alipan.New(ctx, alipan.WithQRCodeTerminal())     // 终端二维码（默认）
alipan.New(ctx, alipan.WithQRCodeWeb(8080))      // 网页扫码：浏览器访问 http://<IP>:8080
alipan.New(ctx, alipan.WithTokenFile("work"))    // 复用 ~/.aligo/work.json
```

## 主要能力

- 文件：列表/搜索/获取/路径、复制/移动/回收站/恢复/重命名/收藏/建文件夹（含批量）
- 上传（秒传 + 分片 + URL 续期）、下载（断点续传 + 进度）
- 分享：创建/更新/取消/列出我的分享、保存他人分享（含批量）、CustomShare（aligo:// 协议）
- 用户 / 网盘 / 容量信息；access_token 自动刷新 + 持久化

模块约定与已知注意点（如备份盘不能分享）见 [AGENTS.md](./AGENTS.md)，
版本历史见 [CHANGELOG.md](./CHANGELOG.md)。
