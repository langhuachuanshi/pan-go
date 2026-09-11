# baidu 模块 API 参考

> 面向使用者的方法文档。接口约定（域名/公共参数/bdstoken）见模块 `AGENTS.md`。

## 全局约定

- **Client 构造**：`c, err := baidu.New(ctx, &baidu.Config{BDUSS: "...", STOKEN: "..."})`。BDUSS 必填，STOKEN 写操作必需（list 可空）。
- **鉴权**：BDUSS + STOKEN cookie（网页端登录态），由 SDK 注入 cookiejar；写操作自动携带 bdstoken（auth 包获取并缓存）。
- **获取凭证**：浏览器 F12 复制，或扫码登录 `baidu/qrcode`（`qrcode.Create(ctx)` → `login.Wait(ctx, timeout)` 换发 `*Credentials`）。
- **业务入口**：`c.Files() / c.Management() / c.Upload() / c.Download() / c.Share() / c.User() / c.CloudDL()`。
- **错误处理**：响应 `errno != 0` 返回错误，消息含接口 errno；用 `errors.Is` 无法区分业务码，按错误文本对号（后续版本计划补错误哨兵）。

---

## 文件（c.Files()）

### 列出文件

**方法名**：`List`

**方法签名**：`func (s *Service) List(ctx context.Context, req *ListRequest) ([]*types.File, error)`

**调用示例**：

```go
files, err := c.Files().List(ctx, &file.ListRequest{Dir: "/", Num: 20})
for _, f := range files {
    if f.IsFolder() {
        fmt.Println("[文件夹]", f.Name())
    } else {
        fmt.Println("[文件]", f.Name(), f.Size, f.FSID)
    }
}
```

**参数说明**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| Dir | string | 是 | 目录绝对路径，根目录 `"/"` |
| Num | int | 否 | 每页数量 |
| Page | int | 否 | 页码 |

**返回值**：`[]*types.File`；`f.Name()`、`f.Size`、`f.FSID`、`f.IsFolder()`、路径与 md5 等字段。

**失败**：BDUSS 无效返回登录跳转类错误。

**注意**：仅需 BDUSS，不需要 STOKEN。

### 其他文件方法

| 方法 | 说明 |
|------|------|
| `Search(ctx, req *SearchRequest) ([]*types.File, error)` | 全盘搜索 |
| `Meta(ctx, fsid) (*types.File, error)` / `BatchMeta(ctx, fsids)` | 单个/批量元信息 |
| `RecurseList(ctx, dir) ([]*types.File, error)` | 递归列目录 |
| `MatchPath(ctx, pattern) ([]*types.File, error)` | 模式匹配路径 |

---

## 管理（c.Management()）

| 方法 | 说明 |
|------|------|
| `MakeDir(ctx, dirPath) (*types.File, error)` | 建目录（含多级路径） |
| `MakeDirIfNotExist(ctx, dirPath) (*types.File, error)` | 先查后建，已存在直接返回（父目录分页拉全量精确匹配） |
| `Copy(ctx, sourcePaths, destDir) error` | 复制（批量） |
| `Move(ctx, sourcePaths, destDir) error` | 移动（批量） |
| `Rename(ctx, oldPath, newPath) error` | 重命名 |
| `Delete(ctx, paths) error` | 删除（进回收站，批量） |
| `RecycleList(ctx, page) ([]*RecycleFile, error)` | 回收站列表 |
| `RecycleRestore(ctx, fsIDs) error` | 回收站恢复 |

**注意**：以上均为写操作，必须提供 STOKEN。

---

## 上传（c.Upload()）

### 上传文件

**方法名**：`Upload`

**方法签名**：`func (s *Service) Upload(ctx context.Context, req *UploadRequest) (*types.File, error)`

**调用示例**：

```go
f, err := c.Upload().Upload(ctx, &upload.UploadRequest{
    ReaderAt: bytes.NewReader(data),
    FileName: "test.txt",
    Size:     int64(len(data)),
    DestPath: "/apps/xxx",
})
```

**参数说明**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| ReaderAt | io.ReaderAt | 是 | 可定位读取源 |
| FileName | string | 是 | 文件名 |
| Size | int64 | 是 | 文件大小 |
| DestPath | string | 是 | 目标目录绝对路径 |
| OnProgress | func(uploaded, total int64) | 否 | 分片上传进度回调 |

**返回值**：`*types.File`（含 fs_id、路径）。

**失败**：STOKEN 缺失；路径不可写；分片校验失败。

**注意**：流程为 precreate → superfile2 分片 → create；命中秒传直接建文件。需 STOKEN。

### 其他

| 方法 | 说明 |
|------|------|
| `RapidUploadCheck(ctx, path, size, blockMD5s) (rapid bool, uploadID string, err error)` | 秒传探测 |
| `CreateFile(ctx, path, size, uploadID, blockMD5s) (*types.File, error)` | 分片完成后建文件 |

---

## 下载（c.Download()）

| 方法 | 说明 |
|------|------|
| `GetDownloadURL(ctx, req *DownloadRequest) (string, error)` | 取下载直链（有 FSID+PanHome 缓存走 PanAPI，否则 PCS 直链） |
| `Download(ctx, httpClient, req) error` | 下载内容到 `req.Writer`（支持 `OnProgress`） |
| `DownloadStream(ctx, httpClient, path, writer, rangeStart, rangeEnd, onProgress) error` | 流媒体/Range 下载 |
| `SetPanHomeCache(httpClient)` | 使用 PanAPI 下载前必须调用（httpClient 需带 BDUSS） |

**注意**：PCS 直链 GET 需带 BDUSS cookie，`httpClient` 由调用方传入且必须携带与 Client 相同的 cookie。

---

## 分享（c.Share()）与转存

| 方法 | 说明 |
|------|------|
| `ShareSet(ctx, paths, opt *ShareOption) (*Shared, error)` | 创建分享（含时限/密码选项） |
| `ShareCancel(ctx, shareIDs) error` / `ShareList(ctx, page)` / `ShareSURLInfo(ctx, shareID)` | 分享管理 |
| `TransferSave(ctx, httpClient, surl, pwd, destDir) ([]*TransferResult, error)` | 转存他人分享（surl 为分享链接尾部标识） |
| `TransferQuery(ctx, httpClient, shareURL, destDir) ([]*TransferResult, error)` | 查询转存结果 |

**注意**：`TransferSave / TransferQuery` 的 `httpClient` 需带 BDUSS cookie（同下载）。

---

## 用户（c.User()）与离线下载（c.CloudDL()）

| 方法 | 说明 |
|------|------|
| `User().Quota(ctx) (*QuotaInfo, error)` | 容量信息 |
| `User().User(ctx, httpClient) (*UserInfo, error)` | 用户信息 |
| `User().CheckLogin(ctx, httpClient) (bool, error)` | 登录态探活（区分登录页重定向与改版跳转） |
| `CloudDL().AddTask(ctx, sourceURL, savePath) (int64, error)` | 添加离线下载任务 |
| `CloudDL().QueryTask / ListTasks / CancelTask / DeleteTask / ClearTasks` | 任务查询与管理 |
