# alipan 模块 API 参考

> 面向使用者的方法文档。完整签名以 godoc 为准，本页覆盖核心与常用方法。

## 全局约定

- **Client 构造**：`c, err := alipan.New(ctx, opts...)`。登录方式（Option 二选一）：
  - `alipan.WithQRCodeTerminal()` 终端二维码（默认）
  - `alipan.WithQRCodeWeb(8080)` 网页扫码（浏览器访问 http://IP:8080）
  - `alipan.WithRefreshToken("xxxx")` refresh_token 直登
  - `alipan.WithTokenFile("work")` 复用 `~/.aligo/work.json`
- **鉴权**：access_token 自动刷新 + 持久化，调用方不感知过期。
- **业务入口**：`c.Files() / c.Share() / c.Drives() / c.Users() / c.Download()`。
- **常用 Client 信息方法**：`c.DefaultDriveID()`、`c.ResourceDriveID()`、`c.UserID()`、`c.AccessToken()`、`c.Token()`、`c.Logout()`。
- **错误处理**：统一 `*alipan.APIError`（statusCode / code / message）。

---

## 文件（c.Files()）

### 列出文件

**方法名**：`List`

**方法签名**：`func (s *Service) List(ctx context.Context, req *ListRequest) ([]*types.BaseFile, error)`

**调用示例**：

```go
files, err := c.Files().List(ctx, &file.ListRequest{
    ParentFileID: "root",
    DriveID:      c.DefaultDriveID(),
})
for _, f := range files {
    fmt.Println(f.Name, f.Size, f.FileID)
}
```

**参数说明**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| ParentFileID | string | 是 | 父目录 ID，根目录 `"root"` |
| DriveID | string | 是 | 网盘 ID（`c.DefaultDriveID()` 取默认） |
| 其余 | — | 否 | 分页 / 排序字段见 ListRequest 定义 |

**返回值**：`[]*types.BaseFile`（`Name / Size / FileID / Type` 等，Type 含 folder）。

**失败**：`ErrAPIError`（token 失效由 SDK 自动刷新后重试）。

### 其他文件方法

| 方法 | 说明 |
|------|------|
| `ListPage(ctx, req)` / `Get(ctx, ...)` / `GetPath(ctx, ...)` | 手动分页 / 详情 / 路径 |
| `Search(ctx, req)` / `SearchByName(ctx, keyword, driveID)` | 搜索 |
| `CreateFolder(ctx, req) (*CreateFolderResp, error)` | 建文件夹 |
| `Copy(ctx, req)` / `CopyBatch(ctx, fileIDs, toParentFileID, driveID)` | 复制（单个/批量） |
| `Move(ctx, req)` / `MoveBatch(...)` | 移动（单个/批量） |
| `Rename(ctx, ...)` / `Update(...)` | 重命名 / 更新 |
| `Star / Unstar` | 收藏 / 取消收藏 |
| `Trash / Restore / TrashBatch / RestoreBatch` | 回收站 |
| `Upload(ctx, req *UploadRequest) (*types.BaseFile, error)` | 上传（秒传 + 分片 + URL 续期） |
| `CreateByHash(ctx, req) (*types.BaseFile, error)` | 按哈希秒传建文件 |

---

## 分享（c.Share()）

### 创建分享链接

**方法名**：`Create`

**方法签名**：`func (s *Service) Create(ctx context.Context, req *CreateShareLinkRequest) (*CreateShareLinkResponse, error)`

**调用示例**：

```go
resp, err := c.Share().Create(ctx, &share.CreateShareLinkRequest{
    FileIDList: []string{"file_id"},
    SharePwd:   "1234",
})
fmt.Println(resp.ShareURL, resp.SharePwd) // https://www.alipan.com/s/xxx 1234
```

**参数说明**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| FileIDList | []string | 是 | 文件 ID 列表 |
| SharePwd | string | 否 | 提取码 |

**返回值**：`resp.ShareURL`、`resp.SharePwd`。

**失败**：`ErrAPIError`；**备份盘内容不能分享**（2026-09 实测根因）。

### 保存他人分享

| 方法 | 说明 |
|------|------|
| `GetShareToken(ctx, shareID, sharePwd) (*types.ShareToken, error)` | 取分享 token（sharePwd 无密码传 `""`） |
| `ListFiles(ctx, req *ShareFileListRequest) ([]*types.BaseShareFile, error)` | 列分享内文件 |
| `SaveToDrive(ctx, req *SaveToDriveRequest) (*SaveToDriveResponse, error)` | 保存到自己的网盘 |
| `SaveToDriveBatch(ctx, shareToken, fileIDs, toParentFileID, toDriveID) ([]BatchItem, error)` | 批量保存整个分享 |
| `SearchInShare(ctx, req) ([]*types.BaseShareFile, error)` | 分享内搜索 |

### 我的分享与快传

| 方法 | 说明 |
|------|------|
| `ListMyShare(ctx, req) ([]*types.ShareLinkSchema, error)` | 列出我的分享 |
| `Update(ctx, req *UpdateShareLinkRequest) error` | 更新（如改提取码） |
| `Cancel(ctx, shareID) error` / `CancelBatch(ctx, shareIDs) error` | 取消分享 |
| `GetShareInfo(ctx, shareID) (*GetShareInfoResponse, error)` | 分享详情 |
| `QuickShare(ctx, req *QuickShareRequest) (*QuickShareResponse, error)` | 快传分享（官方 /t/ 链接） |
| `share.NewCustomSharer(c.Files())` → `ShareFile / SaveFromCode` | CustomShare（aligo:// 协议，不依赖官方分享接口） |

---

## 网盘 / 用户（c.Drives() / c.Users()）

| 方法 | 说明 |
|------|------|
| `Drives().GetDefault(ctx) (*types.BaseDrive, error)` | 默认网盘 |
| `Drives().ListMyDrives(ctx) ([]*types.BaseDrive, error)` | 网盘列表 |
| `Drives().Capacity(ctx) (*types.DriveCapacityDetail, error)` | 容量 |
| `Users().Get(ctx) (*types.BaseUser, error)` | 用户信息（UserName / NickName / UserID） |

---

## 下载（c.Download()）

| 方法 | 说明 |
|------|------|
| `GetDownloadURL(ctx, fileID, driveID) (string, error)` | 取下载直链 |
| `Download(ctx, req *DownloadRequest) error` | 下载（断点续传 + 进度回调，见 DownloadRequest） |
