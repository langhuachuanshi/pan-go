# baidu 模块约定

> 公共规范（文档与计划 / TODO / 更新日志 / Tag / 提交与发版 / 行为准则）见仓库根 `AGENTS.md`，本文件只写本模块特有内容。

## 模块概览

百度网盘网页端（抓包）Go SDK。BDUSS + STOKEN cookie 鉴权，走 pan.baidu.com 网页端
REST API。与 openapi 分支（OAuth + /xpan/*）是两套完全不同的鉴权模型，不要混用。

## 目录结构

```
baidu/            # 主包：Client（实现 invoker.Invoker）+ Config
├── auth/         # BDUSS/STOKEN 凭证管理 + bdstoken 获取
├── file/         # 列表/搜索/元信息/递归列目录
├── management/   # 建目录/复制/移动/重命名/删除/回收站
├── upload/       # 分片上传（precreate→superfile2→create，含秒传、进度回调）
├── download/     # 下载（dlink / 流式）
├── share/        # 创建分享/管理 + 转存他人分享
├── clouddl/      # 离线下载
├── user/         # 容量/用户信息/登录态探活 CheckLogin
├── qrcode/       # 扫码登录（getqrcode→unicast 轮询→qrbdusslogin 换发 BDUSS/STOKEN）
├── panhome/      # sign/bdstoken 缓存
├── sign/         # 签名算法（Sign2 / locate download 签名）
├── types/        # 数据模型
└── invoker/      # 共享调用接口
```

## 接口约定（均实测确认）

- 接口全走 `pan.baidu.com`，pcs 域名仅用于分片上传 superfile2（d.pcs.baidu.com）。
- query 统一带 `channel=chunlei&web=1&app_id=250528&clienttype=0`；写操作额外带
  bdstoken（auth 包自动获取）+ `Referer: https://pan.baidu.com/disk/main`。
- BDUSS 所有接口必需；STOKEN 写操作必需（list 可空）。
- 扫码登录（baidu/qrcode）可换发 BDUSS/STOKEN。

## 常用命令

```bash
cd baidu && go build ./... && go vet ./... && go test ./...
```

## 模块特有规矩

- 凭据只走环境变量 `PANBAIDU_BDUSS` / `PANBAIDU_STOKEN`（或本地 cookie 文件），绝不入库。
- 已知问题与修复计划见 `docs/reviews/2026-09-12-baidupan.md`（下载 API 的
  httpClient 依赖外置、GetPCSLocateURL 等疑似闲置 API）。
