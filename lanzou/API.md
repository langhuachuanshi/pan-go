# lanzou 模块 API 参考

> 面向使用者的方法文档。协议细节与逆向参考见模块 `AGENTS.md`。

## 全局约定

- **Client 构造**：`c := lanzou.NewClient(lanzou.WithTimeout(30))`，可选 `WithMaxSize / WithMaxDownloadCount / WithUploadDelay / WithChallengeConfig / WithHTTPClient`。
- **鉴权**：`c.Login(user, pwd)` 账号密码登录；或 `c.SetCookiesFromMap(map[string]string{...})` 注入已持久化 cookie 免登。`c.GetCookieString()` 导出会话。
- **错误处理**：所有错误用 `errors.Is` 判断哨兵：`lanzou.ErrNotLoggedIn / ErrPasswordWrong / ErrFileExpired / ErrFileSizeLimit / ErrInvalidURL / ErrExtractFailed / ErrUploadFailed / ErrDownloadFailed / ErrAPIError`。
- **业务入口**：`c.Resolve() / c.Account() / c.Files() / c.Folders() / c.Upload() / c.Download() / c.Recycle()`。
- 登录登出是会话生命周期方法，在 Client 上：`c.Login / c.Logout`。
- 所有业务方法首参为 `ctx context.Context`（与 pan-go 其他模块一致）。
- HTTP 执行走 `pan-go/core`（httpx/invoker），本模块只保留 cookie 会话与挑战页方言。

---

## 直链解析（c.Resolve()，无需登录）

### 通过分享链接取直链

**方法名**：`GetDurlByURL`

**方法签名**：`func (s *Service) GetDurlByURL(ctx context.Context, shareURL, pwd string) (string, error)`

**调用示例**：

```go
durl, err := c.Resolve().GetDurlByURL(ctx, "https://pan.lanzoul.com/xxxxx", "")
```

**参数说明**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| shareURL | string | 是 | 蓝奏云分享链接（各域名自适应） |
| pwd | string | 否 | 提取密码，无密码传 `""` |

**返回值**：`durl`（真实下载直链，GET 时需带同款 UA 与 Referer）。

**失败**：`ErrInvalidURL` 非蓝奏链接；`ErrPasswordWrong` 密码错误；`ErrFileExpired` 文件失效；`ErrExtractFailed` 页面解析失败（常见于换混淆）。

**注意**：无需登录；蓝奏云换 JS 混淆导致解析失败时，更新 `c.SetChallengeConfig(&lanzou.ChallengeConfig{...})` 即可，无需升级 SDK。

### 通过分享链接取文件详情

**方法名**：`GetFileInfo`

**方法签名**：`func (s *Service) GetFileInfo(ctx context.Context, shareURL, pwd string) (*FileDetail, error)`

**调用示例**：

```go
detail, err := c.Resolve().GetFileInfo(ctx, "https://pan.lanzoul.com/xxxxx", "pwd")
fmt.Println(detail.NameAll, detail.Size, detail.DURL)
```

**参数说明**：同 `GetDurlByURL`。

**返回值**：`detail.NameAll`（文件名）、`detail.Size`（大小文本）、`detail.DURL`（直链，可能为空）、`detail.DownloadURL`（原分享链接）、`detail.FileID`。

**失败**：同 `GetDurlByURL`。

**注意**：iframe 或直链步骤失败时不报错，返回已有部分信息（`DURL` 为空），调用方需按需检查。

### 兼容方法

| 方法 | 说明 |
|------|------|
| `GetDurlByFolderURL(folderURL, pwd string, subdir bool) ([]string, error)` | 文件夹分享解析，当前行为与 `GetDurlByURL` 相同 |
| `GetDurlByURLAndFolder(shareURL, pwd, folderID string) (string, error)` | folderID 参数未使用（历史兼容，勿依赖） |
| `FileInfoByURL(shareURL, pwd string) (*FileDetail, error)` | 兼容别名，等价 `GetFileInfo` |

---

## 会话（c 上）与账号（c.Account()）

### 登录

**方法名**：`Login`

**方法签名**：`func (c *Client) Login(user, pwd string) error`

**调用示例**：

```go
err := c.Login("user", "pass")
```

**参数说明**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| user | string | 是 | 账号 |
| pwd | string | 是 | 密码 |

**返回值**：无（成功即建立会话）。

**失败**：`ErrPasswordWrong` 密码错误；`ErrAPIError` 其他登录失败（含旧接口迁移后的提示语）。

**注意**：登录走 accounts.woozooo.com 新账号系统；成功后可用 `c.GetCookieString()` 持久化会话，下次 `SetCookiesFromMap` 免登。

### 登出

**方法名**：`Logout`

**方法签名**：`func (c *Client) Logout() error`

**调用示例**：`err := c.Logout()`（无参数）。

**返回值**：无。**注意**：登出后本地 cookies 清空。

### 用户信息 / 帐号详情

**方法名**：`Info` / `Detail`

**方法签名**：`func (s *Service) Info(ctx context.Context) (*UserInfo, error)`、`func (s *Service) Detail(ctx context.Context) (*AccountInfo, error)`

**调用示例**：

```go
u, _ := c.Account().Info(ctx)      // u.UserName
a, _ := c.Account().Detail(ctx)    // a.TotalSize / a.UsedSize
```

**参数说明**：无参数。

**返回值**：`UserName`（用户 ID）；`Detail` 另含 `TotalSize / UsedSize`（从个人中心页面提取的容量文本）。

**失败**：`ErrNotLoggedIn` 未登录。

**注意**：`Detail` 从 HTML 页面正则提取，格式随蓝奏云改版可能变化。

---

## 文件（c.Files()）

### 列出文件

**方法名**：`List`

**方法签名**：`func (s *Service) List(ctx context.Context, fid int) (*FileList, error)`

**调用示例**：

```go
files, err := c.Files().List(ctx, -1) // 根目录传 -1
```

**参数说明**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| fid | int | 是 | 文件夹 ID，根目录传 `-1` |

**返回值**：`files.Text[]`，每项含 `ID / NameAll / Size / Time / Icon / Downs / Onof / IsNewd / FID`。

**失败**：`ErrNotLoggedIn`；`ErrAPIError` 接口返回异常。

**注意**：自动翻页直至空页。

### 获取分享链接

**方法名**：`ShareURL`

**方法签名**：`func (s *Service) ShareURL(ctx context.Context, fileID string) (*FileShareInfo, error)`

**调用示例**：

```go
info, _ := c.Files().ShareURL(ctx, fileID)
share := info.IsNewd + "/" + info.FID // 完整分享链接
```

**参数说明**：`fileID`（string，必填）—— `List` 返回的 `FileInfo.ID`。

**返回值**：`IsNewd`（分享 URL 前缀）、`FID`（分享 ID）、`Pwd`（提取密码，空=无）、`Onof`（"1"=有密码）。

**失败**：`ErrNotLoggedIn`；`ErrAPIError`。

### 其他

| 方法 | 说明 |
|------|------|
| `Move(fids []string, fid int) error` | 批量移动文件到目标文件夹 |
| `Delete(fids []string) error` | 批量删除文件（进回收站） |
| `SetPassword(entityID int, pwd string) error` | 设置文件/文件夹密码 |

---

## 文件夹（c.Folders()）

| 方法 | 说明 |
|------|------|
| `List(fid int) (*FolderList, error)` | 列出子文件夹（根目录传 -1；返回 `Text[].FolID / Name`） |
| `Create(name string, parentID int) (*FolderInfo, error)` | 创建文件夹，返回含 `FolID` |
| `Delete(fids []string) error` | 批量删除 |
| `Move(fids []string, fid int) error` | 批量移动到目标文件夹 |

均要求登录；失败返回 `ErrNotLoggedIn` 或 `ErrAPIError`。

---

## 上传（c.Upload()）

### 流式上传（推荐）

**方法名**：`Stream`

**方法签名**：`func (s *Service) Stream(ctx context.Context, filePath string, fid int, onProgress func(uploaded, total int64), desc ...string (*UploadResult, error)`

**调用示例**：

```go
res, err := c.Upload().Stream("local.txt", -1, func(up, total int64) {
    fmt.Printf("%d/%d\n", up, total)
})
fmt.Println(res.FileID, res.FileName)
```

**参数说明**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| filePath | string | 是 | 本地文件路径 |
| fid | int | 是 | 目标文件夹 ID（根目录传 0） |
| onProgress | func(int64, int64) | 否 | 进度回调（uploaded, total 字节），可 nil |
| desc | ...string | 否 | 文件描述 |

**返回值**：`res.FileID / res.FileName / res.Info`。

**失败**：`ErrNotLoggedIn`；`ErrFileSizeLimit` 超限；`ErrUploadFailed` 上传被拒；`ErrAPIError` 响应异常。

**注意**：边读边发不占内存，`onProgress` 反映真实网络进度；蓝奏 html5up.php 支持 chunked。

### 其他

| 方法 | 说明 |
|------|------|
| `File(filePath string, fid int, desc ...string) (*UploadResult, error)` | 一次性载入内存的兼容版本 |
| `ByURL(fileURL string, fid int, desc ...string) (*UploadResult, error)` | 转存网盘已有文件（非本地文件） |

---

## 下载（c.Download()）

| 方法 | 说明 |
|------|------|
| `File(savePath, shareURL string, pwd ...string) error` | 按分享链接下载到指定路径 |
| `FileAuto(saveDir, shareURL, pwd string) error` | 自动按远端文件名保存到目录 |
| `Dir(saveDir string, fid int) error` | 递归下载整个文件夹（并发受 `WithMaxDownloadCount` 限制；汇总错误返回） |
| `ByURL(savePath, durl string) error` | 直接按直链下载 |

**注意**：直链 GET 需带与 SDK 相同的 UA；`Dir` 部分文件失败不中断整体，错误在返回值中汇总。

---

## 回收站（c.Recycle()）

| 方法 | 说明 |
|------|------|
| `List(page int) (*RecycleList, error)` | 分页列表（`Text[].FileID / FileName / FilePath / UploadTime`） |
| `MoveToTrash(fids []string) error` | 移入回收站 |
| `RestoreFiles(fids []string) error` | 恢复 |
| `CleanRecycle() error` | 清空回收站 |

**注意**：`CleanRecycle` 不可逆，调用前确认。
