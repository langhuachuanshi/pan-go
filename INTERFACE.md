# 夸克网盘接口参考索引

> **本文件用途**:quark-go 的接口参考索引。需要查接口、补功能、查 bug 时,先看这里。
> 你不需要会抓包——接口的真相都在下面两个开源项目里,交叉核对即可。

## 参考源(两大金矿)

| 项目 | 语言 | 强项 | 仓库位置 |
|---|---|---|---|
| **AList** | Go | 文件/上传/下载最权威,签名逻辑最可信 | `github.com/alist-org/alist` → `drivers/quark_uc/`(夸克与 UC 共用驱动) |
| **quark-auto-save** | Python | 转存/回收站/账号/容量最权威 | `github.com/Cp0204/quark-auto-save` → `quark_auto_save.py` 的 `Quark` 类 |

> AList 的 `quark_uc` 驱动**不含**创建分享和转存;quark-auto-save **不含**上传和创建分享。两者刚好互补,我们的"创建分享"参考的是其他来源。

## 基础约定

- **base 域名(等价,任选)**:
  - `https://drive-pc.quark.cn/1/clouddrive` ← 本项目用(`client.go:34`)
  - `https://drive.quark.cn/1/clouddrive` ← AList 用
  - 一个被风控时可切另一个。
- **公共 query**:`pr=ucpro`、`fr=pc`(**注意不是 `uqm`**,会被当游客返回 31001)
- **公共 header**:`Cookie`、伪装 Electron 客户端的 `User-Agent`、`Referer: https://pan.quark.cn`、`Origin: https://pan.quark.cn`
- **错误约定**:响应 `{status, code, message, data, metadata}`,`status >= 400` 或 `code != 0` 即失败
  - 常见码:`31003` cookie 失效 · `31005` 文件不存在 · `41013` 转存限频
- **`__puus` 回写**:响应里若带 `Set-Cookie: __puus=...`,要合并回本地 cookie(它会刷新,见下方"潜在问题 #2")

---

## 总览矩阵

图例:✅ 已实现 · ⚠️ 实现但有差异 · ❌ 未实现

| 接口 | AList | quark-auto-save | 本项目 | 备注 |
|---|:---:|:---:|:---:|---|
| `GET /file/sort` 列目录 | ✅ | ✅ | ✅ | 已对齐:带 fetch_all_file / fetch_risk_file_name |
| `POST /file` 建目录 | ✅ | ✅ | ⚠️ | 我们靠 List 匹配取 fid;quark-auto-save 用 `dir_path` 直接返回 fid |
| `POST /file/rename` | ✅ | ✅ | ✅ | 一致 |
| `POST /file/move` | ✅ | ✅ | ✅ | 一致(`action_type=1`) |
| `POST /file/delete` | ✅ | ✅ | ✅ | AList/我们用 `action_type=1`,quark-auto-save 用 `2`,均可用 |
| `GET /file/info` 详情 | ❌ | ❌ | ✅ | 我们独有 |
| `POST /file/info/path_list` 路径转 fid | ❌ | ✅ | ❌ | 缺 |
| `POST /file/download` 取直链 | ✅ | ✅ | ✅ | AList 做 cookie 快照(见下) |
| `POST /file/v2/play/project` 视频转码 | ✅ | ❌ | ❌ | 缺(可选) |
| 上传全链路(pre/hash/auth/OSS×2/finish) | ✅ | ❌ | ✅ | **1:1 吻合,已验证正确** |
| 回收站 `recycle/list`·`recycle/remove` | ❌ | ✅ | ❌ | 缺 |
| `archive/unarchive` 解压 | ❌ | ✅ | ❌ | 缺 |
| 创建分享(`/share`→`/task`→`/share/password`) | ❌ | ❌ | ✅ | 我们独有 |
| 转存(token→detail→save→task) | ❌ | ✅ | ⚠️ | 缺反风控参数 `__dt`/`__t`,缺移动端域名切换 |
| `GET /task` 异步轮询 | ❌ | ✅ | ✅ | 一致 |
| `GET /config` 探活 + `__puus` 保活 | ✅ | ❌ | ❌ | 缺,长时间运行会 403 |
| `GET /account/info` cookie 校验 | ❌ | ✅ | ⚠️ | 我们只检查 `__puus` 是否存在,不够准 |
| 容量/签到(移动端,需 `kps/sign/vcode`) | ❌ | ✅ | ❌ | 缺(可选) |

---

## 模块详述

### 文件管理(`quark/file/`)

| 接口 | 用途 | 关键参数 | 参考源 |
|---|---|---|---|
| `GET /file/sort` | 列目录 | `pdir_fid`/`_page`/`_size`/`_fetch_total=1`/`_sort` + `fetch_all_file=1`/`fetch_risk_file_name=1` | AList `util.go:117` · qsas `:562` |
| `POST /file` | 建目录 | `dir_init_lock`/`dir_path`/`file_name`/`pdir_fid`;成功后 `sleep 1s` | AList `driver.go:110` · qsas `:660` |
| `POST /file/rename` | 重命名 | `fid`/`file_name` | AList `driver.go:137` · qsas `:674` |
| `POST /file/move` | 移动 | `action_type=1`/`filelist`/`to_pdir_fid`/`exclude_fids` | AList `driver.go:126` · qsas `:732` |
| `POST /file/delete` | 删除(进回收站) | `action_type=1`/`filelist`/`exclude_fids` | AList `driver.go:153` · qsas `:683` |
| `POST /file/info/path_list` | **路径→fid**(批量,≤50) | body `{"file_path":[...], "namespace":"0"}` | qsas `:543` |

### 上传(`quark/upload/`)—— ✅ 已与 AList 完全对齐

六步全流程,每一步的请求体/签名格式都与 AList 一致:

| 步骤 | 接口 | 用途 |
|---|---|---|
| 1 | 本地算 md5+sha1 | 流式,32KB 块 |
| 2 | `POST /file/upload/pre` | 拿 task_id/auth_info/bucket/obj_key/upload_url/upload_id/part_size/callback |
| 3 | `POST /file/update/hash` | 秒传:`data.finish=true` 直接跳到步骤 6 |
| 4 | `POST /file/upload/auth` → `PUT <OSS>` | 每片先换 auth_meta 签名再直传 OSS,收 ETag |
| 5 | `POST /file/upload/auth` → `POST <OSS>` | commit:XML body + Content-MD5 + x-oss-callback |
| 6 | `POST /file/upload/finish` | 提交 obj_key/task_id,成功后 `sleep 1s` |

- **OSS 签名的关键**:`auth_meta` 是 OSS V1 CanonicalRequest 格式,客户端不算密钥,而是把它发给夸克 `/file/upload/auth` 代签,拿回 `Authorization`(`oss.go`)。
- **固定魔术串**:`x-oss-user-agent: aliyun-sdk-js/6.6.1 Chrome 98.0.4758.80 on Windows 10 64-bit`(参与签名,必须照抄)
- **分片大小**取自服务端 `metadata.part_size`,客户端不硬编码(有 4MB 兜底)
- 参考源:AList `util.go:276-446`

### 下载(`quark/download/`)

- `POST /file/download` body `{"fids":[fid]}` → `data[0].download_url`(临时直链)
- GET 直链需带 `Cookie/Referer/UA`,否则 403 `RequestDeniedByCallback`
- 参考源:AList `util.go:153` · qsas `:651`

### 创建分享(`quark/share/create.go`)—— 我们独有

3 步(抓包确认,参考源未覆盖):
1. `POST /share` body `{fid_list, title, url_type, expired_type, passcode?}` → `data.task_resp.data.share_id`
2. `GET /task`(可选,步骤 1 一般 `task_sync=true` 已完成)
3. `POST /share/password` body `{share_id}` → `data.share_url`(短链,**无密码也必须调**,否则链接"已删除")

### 转存他人分享(`quark/share/transfer.go`)

4 步(参考 quark-auto-save):

| 步骤 | 接口 | 取值 |
|---|---|---|
| ① | `POST /share/sharepage/token` body `{pwd_id, passcode}` | `data.stoken`(兼验分享是否失效) |
| ② | `GET /share/sharepage/detail` query `{pwd_id, stoken, pdir_fid, _page, _size, ...}` | `data.list[].{fid, share_fid_token}` |
| ③ | `POST /share/sharepage/save` body `{fid_list, fid_token_list, to_pdir_fid, pwd_id, stoken, ...}` | `data.task_id` |
| ④ | `GET /task` 轮询至 `data.status==2` | `data.save_as.save_as_top_fids[]` |

- **stoken vs share_fid_token**:`stoken` 是按分享(`pwd_id`)维度的会话 token;`share_fid_token` 是 detail 里**每个文件**各自的令牌。两者层级不同,不能混用。
- 单次 save 建议 ≤100 个文件(`save_as_top_fids` 上限 100),本项目已自动分批。

---

## ⚠️ 关键差异与潜在问题(按优先级)

### 1. 转存缺反风控参数,收紧时可能被拦
quark-auto-save 的 `save` 还带了 `__dt`(1~5分钟随机毫秒)+ `__t`(时间戳)两个 query;而且当 cookie 含 `kps/sign/vcode` 时,会把整个 `share` 系列接口切到**移动端域名** `drive-m.quark.cn` 并改用签名鉴权(去 cookie,加 `fr=android`/`pf`/`bi`/`ve`/`kps`/`sign`/`vcode` 等)。
- **我们的现状**(`transfer.go`):PC 域名 + 无 `__dt`/`__t`。目前实测能成,但风控收紧(尤其高频转存)时可能失败。
- 参考源:qsas `quark_auto_save.py:384-434`(域名切换)、`:594-616`(save 参数)

### 2. 缺 `__puus` cookie 保活,长时间运行会 403
`__puus` 会过期,AList 的做法:每 100 分钟(±5min 抖动)主动发起一次**剥离了 `__puus`** 的 `GET /config`,让服务端重新下发新 `__puus`,合并回 cookie。否则下载/请求会逐渐 403。
- **我们的现状**:`auth.IsValid` 只检查 `__puus` 是否存在,不刷新。短脚本没事,长期运行(后台服务/定时转存)会踩坑。
- 参考源:AList `util.go:198-244`(`refreshPuus`)

### 3. `file/sort` 缺 `fetch_risk_file_name=1` ✅ 已修复
已在 List/ListPage 补上 `fetch_all_file=1` 和 `fetch_risk_file_name=1`(与 AList、quark-auto-save 对齐),含违规词的文件名返回原名,不再变 `***`。

### 4. `MakeDir` 取 fid 的方式可优化
我们建目录后 `sleep 1s` 再 `List` 父目录按名匹配取 fid(`file.go:178-189`),重名会出错。quark-auto-save 用 `dir_path` 创建时响应直接含 `data.fid`。
- 建议核实 `POST /file` 响应是否直接返回新 fid,若是则去掉 List 匹配。

### 5. 下载的 cookie 快照(低优先级,目前无影响)
AList 强调下载直链的签名绑定**发起 `/file/download` 那一刻的 cookie**。我们 cookie 是静态的(没有保活机制),天然一致,**目前无影响**——只有在引入问题 #2 的保活机制后才需要处理。

---

## 可补充功能清单

| 功能 | 接口 | 参考源 | 价值 |
|---|---|---|---|
| 回收站列表/彻底删 | `GET /file/recycle/list`、`POST /file/recycle/remove` | qsas `:692-714` | 中 |
| 解压缩 | `POST /archive/unarchive` | qsas `:716-730` | 中 |
| 路径批量转 fid | `POST /file/info/path_list` | qsas `:543-560` | 中(转存目标目录定位更方便) |
| 视频转码地址 | `POST /file/v2/play/project` | AList `util.go:255` | 低(播放场景) |
| cookie 有效性精确校验 | `GET /account/info`(域名 `pan.quark.cn`) | qsas `:445` | 中(替代 `__puus` 存在性检查) |
| 容量/每日签到 | `GET /capacity/growth/info`、`POST /capacity/growth/sign`(移动端,需签名) | qsas `:454-495` | 低 |

---

## 参考:本项目端点速查

| 模块 | 方法 | 端点(去掉 base) |
|---|---|---|
| file | List/ListPage | `GET /file/sort` |
| file | Get | `GET /file/info` |
| file | MakeDir | `POST /file` |
| file | Rename | `POST /file/rename` |
| file | Move | `POST /file/move` |
| file | Delete | `POST /file/delete` |
| upload | Upload | `pre`→`hash`→`auth`×2→OSS×2→`finish` |
| download | GetDownloadURL/Download | `POST /file/download` → `GET 直链` |
| share | Create | `POST /share` → `GET /task` → `POST /share/password` |
| share | Transfer 及子步骤 | `sharepage/token` → `sharepage/detail` → `sharepage/save` → `GET /task` |
