# quark 模块 API 参考

> 面向使用者的方法文档。协议细节、参考源与已知差异见 `INTERFACE.md`。

## 全局约定

- **Client 构造**：`c, err := quark.New(ctx)`，默认从 `~/.quark/cookie.json` 读 cookie；可选 `quark.WithCookie(cookie)`（直接传字符串）或 `quark.WithCookieFile(path)`。
- **鉴权**：cookie 鉴权（无 token、无签名）。cookie 须含 `__puus` 或 `__pus` 任一（扫码换发的只有 `__pus`）。
- **获取 cookie**：浏览器复制，或扫码登录 `qrcode.Create(ctx)` + `login.Wait(ctx, 2*time.Minute)`（详见 README）。
- **错误处理**：统一 `*quark.APIError`（`code / status / message`），`code==0 && status==200` 才是成功；登录态失效用 `invoker.IsAuthError(err)` 判断（31001 未登录 / 31003 cookie 失效），命中后引导重新扫码登录。
- **业务入口**：`c.Files() / c.Share() / c.Upload() / c.Download()`；扫码登录在 `quark/qrcode` 包。

---

## 文件（c.Files()）

### 列出文件

**方法名**：`List`

**方法签名**：`func (s *Service) List(ctx context.Context, req *ListRequest) ([]*types.File, error)`

**调用示例**：

```go
files, err := c.Files().List(ctx, &file.ListRequest{PDirFID: "0"})
```

**参数说明**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| PDirFID | string | 是 | 父目录 fid，根目录用 `"0"` |
| Page | int | 否 | 页码，从 1 开始（0 视为 1） |
| Size | int | 否 | 每页数量，默认 50 |
| Sort | string | 否 | 排序，如 `"file_type:asc,updated_at:desc"` |

**返回值**：`[]*types.File`（自动翻页取全量）；字段含 `FID / FileName / Size / Format / ItemType` 等，`f.IsFolder()` 判目录。

**失败**：`ErrAPIError`（31003 cookie 失效等）。

**注意**：已带 `fetch_all_file` / `fetch_risk_file_name`，违规词文件名返回原名。

### 其他文件方法

| 方法 | 说明 |
|------|------|
| `ListPage(ctx, req) ([]*types.File, int, error)` | 手动分页版，返回总数 |
| `Get(ctx, fid) (*types.File, error)` | 文件详情 |
| `MakeDir(ctx, pdirFID, name) (string, error)` | 建目录，返回新 fid（连续建多级目录内部自动重试） |
| `Rename(ctx, fid, newName) error` | 重命名 |
| `Move(ctx, fids, toDirFID) error` | 批量移动 |
| `Delete(ctx, fids) error` | 批量删除（进回收站） |

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
    PDirFID:  "0",
})
```

**参数说明**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| ReaderAt | io.ReaderAt | 是 | 可定位读取源（os.File / bytes.Reader 均可） |
| FileName | string | 是 | 文件名 |
| Size | int64 | 是 | 文件大小 |
| PDirFID | string | 是 | 目标目录 fid，根目录 `"0"` |
| OnProgress | func(uploaded, total int64) | 否 | 进度回调 |

**返回值**：`*types.File`（含 FID）。

**失败**：`ErrAPIError`；ReaderAt 为空直接报参数错。

**注意**：流式处理边算哈希边读，超大文件安全；先秒传探测，命中直接建文件，未命中走分片直传 OSS（分片大小由服务端下发）。

---

## 下载（c.Download()）

| 方法 | 说明 |
|------|------|
| `GetDownloadURL(ctx, fid) (string, error)` | 取临时下载直链 |
| `Download(ctx, req *DownloadRequest) error` | 流式下载到 `req.Writer`（io.Writer） |

**注意**：直链 GET 需带 cookie/Referer/UA 鉴权头（`c.DownloadHeaders()` 可取），否则 403。

---

## 分享与转存（c.Share()）

### 创建分享

**方法名**：`Create`

**方法签名**：`func (s *Service) Create(ctx context.Context, req *CreateRequest) (*types.CreateShareResponse, error)`

**调用示例**：

```go
resp, err := c.Share().Create(ctx, &share.CreateRequest{
    FIDs:        []string{fid},
    Title:       "资源分享",
    Forever:     true,
    WithPasscode: true,
})
fmt.Println(resp.ShareURL, resp.Passcode)
```

**参数说明**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| FIDs | []string | 是 | 文件/文件夹 fid 列表 |
| Title | string | 否 | 分享标题 |
| Forever | bool | 否 | true=永久；false 用 ExpiredDays（1/7/30，其他值按 30 处理） |
| ExpiredDays | int | 否 | 有效天数（Forever=false 时生效） |
| WithPasscode | bool | 否 | true=私密分享（带提取码） |
| Passcode | string | 否 | 提取码；留空自动生成 4 位（夸克拒绝纯数字） |

**返回值**：`resp.ShareURL`（https://pan.quark.cn/s/xxx 短链）、`resp.ShareID`、`resp.Passcode`。

**失败**：`ErrAPIError`；FIDs 为空直接报参数错。

**注意**：无密码也必须走完 `/share/password` 步骤（SDK 内部已处理），否则链接"已删除"；文件夹分享内部自动轮询 `/task` 取 share_id。

### 转存他人分享

**方法名**：`Transfer`

**方法签名**：`func (s *Service) Transfer(ctx context.Context, shareURL, passcode, toDirFID string) ([]string, error)`

**调用示例**：

```go
newFIDs, err := c.Share().Transfer(ctx, "https://pan.quark.cn/s/xxx#/list/share", "ab3x", "0")
```

**参数说明**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| shareURL | string | 是 | 分享链接 |
| passcode | string | 否 | 提取码，无密码传 `""` |
| toDirFID | string | 是 | 保存目标目录 fid |

**返回值**：`newFIDs`（转存后的新 fid 列表）。

**失败**：`ErrAPIError`（41013 转存限频等）。

**注意**：单次 save 超 100 个文件自动分批；缺反风控参数 `__dt`/`__t`，高频转存可能被拦（见 INTERFACE.md 潜在问题 #1）。

### 高级子步骤与其他

| 方法 | 说明 |
|------|------|
| `GetShareToken(ctx, pwdID, passcode) (string, error)` | 取 stoken（兼验分享是否失效） |
| `ListShareFiles(ctx, pwdID, stoken) ([]*ShareFile, error)` | 列分享内文件 |
| `SaveShare(ctx, pwdID, stoken, toDirFID, files) (string, error)` | 保存指定文件，返回 task_id |
| `WaitTask(ctx, taskID) ([]string, error)` | 轮询任务至完成，返回 save_as_top_fids |
| `DeliverByShareURLs(ctx, req) (*types.CreateShareResponse, error)` | 聚合发货：多个商品分享 → 一个限时带码新分享 |
| `ResolveFIDs(ctx, shareURL, passcode) ([]string, error)` | 从自己的分享反查 FID |

---

## 扫码登录（quark/qrcode 包）

### 创建扫码会话

**方法名**：`Create`

**方法签名**：`func Create(ctx context.Context) (*Login, error)`

**调用示例**：

```go
login, err := qrcode.Create(ctx)
fmt.Println(login.QRURL) // 贴到任意二维码生成器出图
```

**参数说明**：无参数。

**返回值**：`login.QRURL`（二维码内容短链）、`login.Token`、`login.RequestID`。

**失败**：网络错误 / 响应异常。

**注意**：token 有效期约 2 分钟，过期重新 `Create`。

### 等待扫码并换发 cookie

**方法名**：`Wait`

**方法签名**：`func (l *Login) Wait(ctx context.Context, timeout time.Duration, interval ...time.Duration) (string, error)`

**调用示例**：

```go
cookie, err := login.Wait(ctx, 2*time.Minute)
// 成功后：quark.New(ctx, quark.WithCookie(cookie)) 或 auth.SaveCookie(cookie) 落盘
```

**参数说明**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| timeout | time.Duration | 是 | 总等待上限 |
| interval | ...time.Duration | 否 | 轮询间隔，默认 2s |

**返回值**：完整 cookie 字符串（可直接构造 Client 或落盘）。

**失败**：超时；二维码过期（50004002，需重新 Create）；ctx 取消。

**注意**：换发的 cookie 只有 `__pus` 不带 `__puus`，drive 接口实测可用；2026-09-12 真机验证。
