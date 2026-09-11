# alipan-go

阿里云盘（alipan / aliyundrive）的 Go SDK，对应 Python 库 [aligo](https://github.com/foyoux/aligo) 的核心子集 + 分享功能。

Go 惯用风格，按业务域子包组织，线程安全，开箱即用。

## 目录结构

```
alipan-go/
├── alipan/              # 主包：Client 入口 + Option + 错误类型别名
│   ├── client.go        #   Client（实现 invoker.Invoker，聚合各 service）
│   ├── option.go        #   With* functional options
│   └── error.go         #   APIError 类型别名
├── alipan/auth/         # 登录认证：扫码/refresh_token/持久化/二维码展示
├── alipan/file/         # 文件操作：列表/搜索/复制/移动/上传/下载/批量/秒传
├── alipan/share/        # 分享：创建/管理/保存他人分享/CustomShare
├── alipan/drive/        # 网盘：详情/默认/列表/容量
├── alipan/user/         # 用户信息
├── alipan/types/        # 所有数据模型（BaseFile/Token/BaseUser/ShareLinkSchema...）
├── alipan/invoker/      # 子包共享的调用接口（打破循环依赖）
└── example/quickstart/  # 示例
```

**依赖关系（单向，无循环）**：`types` ← `invoker` ← 各 service 子包 ← 主包 `alipan`。各 service 通过 `invoker.Invoker` 接口访问 HTTP 能力，不依赖主包。

## 特性

- ✅ 多种登录：终端二维码、网页二维码、refresh_token 直登、本地配置复用（兼容 `~/.aligo/<name>.json`）
- ✅ access_token 自动刷新 + 持久化
- ✅ 文件：列表/搜索/获取/路径、复制/移动/回收站/恢复/重命名/收藏/建文件夹（含批量）
- ✅ 上传（秒传 + 分片 + URL 续期）、下载（断点续传 + 进度）
- ✅ 分享：创建/更新/取消/列出我的分享、保存他人分享（含批量）、CustomShare（aligo:// 协议）
- ✅ 用户/网盘/容量信息

## 安装

```sh
go get github.com/langhuachuanshi/alipan-go
```

## 快速入门

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/langhuachuanshi/alipan-go/alipan"
    "github.com/langhuachuanshi/alipan-go/alipan/file"
)

func main() {
    ctx := context.Background()

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
}
```

## 登录方式

```go
alipan.New(ctx, alipan.WithRefreshToken("xxxx"))   // refresh_token 直登
alipan.New(ctx, alipan.WithQRCodeTerminal())        // 终端二维码（默认）
alipan.New(ctx, alipan.WithQRCodeWeb(8080))         // 网页扫码：浏览器访问 http://<IP>:8080
alipan.New(ctx, alipan.WithTokenFile("work"))       // 复用 ~/.aligo/work.json
```

## 分享

```go
// 创建分享链接（含提取码）
resp, _ := c.Share().Create(ctx, &share.CreateShareLinkRequest{
    FileIDList: []string{"file_id"},
    SharePwd:   "1234",
})
fmt.Println(resp.ShareURL, resp.SharePwd)  // https://www.alipan.com/s/xxx 1234

// 列出/更新/取消我的分享
links, _ := c.Share().ListMyShare(ctx, &share.ListMyShareRequest{})
c.Share().Update(ctx, &share.UpdateShareLinkRequest{ShareID: "id", SharePwd: "new"})
c.Share().Cancel(ctx, "share_id")
c.Share().CancelBatch(ctx, []string{"id1", "id2"})

// 保存他人分享到自己网盘
token, _ := c.Share().GetShareToken(ctx, "share_id", "")  // 无密码传空串
c.Share().SaveToDrive(ctx, &share.SaveToDriveRequest{
    ShareToken: token, FileID: "file_id_in_share",
})
// 批量保存整个分享
c.Share().SaveToDriveBatch(ctx, token, []string{"f1", "f2"}, "root", c.DefaultDriveID())

// CustomShare（aligo:// 协议，不依赖官方分享接口）
cs := share.NewCustomSharer(c.Files())
code, _ := cs.ShareFile(ctx, someFile, driveID)        // 编码
cs.SaveFromCode(ctx, code, "root", driveID)            // 解码并秒传保存
```

## 主要 API

```go
// 文件（alipan/file 包）
c.Files().List / ListPage / Get / GetPath / Search / SearchByName
c.Files().CreateFolder / Copy / Move / Rename / Update / Star / Unstar
c.Files().Trash / Restore / TrashBatch / RestoreBatch / ListRecycleBin
c.Files().CopyBatch / MoveBatch
c.Files().Upload / Download / GetDownloadURL / CreateByHash

// 分享（alipan/share 包）
c.Share().Create / Update / Cancel / CancelBatch / ListMyShare
c.Share().GetShareInfo / GetShareToken / ListFiles / SearchInShare
c.Share().SaveToDrive / SaveToDriveBatch
share.NewCustomSharer(c.Files()).ShareFile / ShareFiles / ShareFolder / SaveFromCode

// 网盘（alipan/drive 包）/ 用户（alipan/user 包）
c.Drives().Get / GetDefault / ListMyDrives / Capacity
c.Users().Get
```

## 错误处理

业务错误为 `*alipan.APIError`（`alipan/invoker.APIError` 的别名），支持 `errors.As`：

```go
files, err := c.Files().List(ctx, req)
if err != nil {
    var apiErr *alipan.APIError
    if errors.As(err, &apiErr) {
        fmt.Println(apiErr.StatusCode, apiErr.Code, apiErr.Message, apiErr.RequestID)
    }
}
```

## 设计说明

对齐 aligo 的网络协议与数据结构（host、URL、header、proof_code、bizExt gb18030 解码、分片大小、x-share-token 机制等均原样复刻），但用 Go 惯用风格重新组织：

| aligo (Python) | alipan-go (Go) |
|---|---|
| `core/Auth.py`（HTTP引擎+登录） | `alipan/auth` + `alipan/client.go` |
| `core/BaseAligo.py`（post/_result/batch） | `alipan/invoker` + 各 service 包 |
| `Aligo` 类 | `alipan.Client` + functional options |
| `Null` 失败哨兵 | `(T, error)` + `*APIError` |
| `apis/*` | `alipan/{file,share,drive,user}` |
| `types/*` | `alipan/types` |

## License

同 aligo，仅供学习交流。
