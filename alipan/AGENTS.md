# alipan 模块约定

> 公共规范（文档与计划 / TODO / 更新日志 / Tag / 提交与发版 / 行为准则）见仓库根 `AGENTS.md`，本文件只写本模块特有内容。

## 模块概览

阿里云盘（alipan / aliyundrive）Go SDK，对应 Python 库 aligo 的核心子集 + 分享功能。
token 鉴权（扫码 / refresh_token 直登）+ secp256k1 设备签名。四模块中唯一有第三方依赖
（go-qrcode、golang.org/x/text、secp256k1）。

## 目录结构

```
alipan/            # 主包：Client（实现 invoker.Invoker）+ Option + APIError 别名
├── auth/          # 登录认证：扫码 / refresh_token / 持久化（兼容 ~/.aligo/<name>.json）
├── device/        # secp256k1 动态签名 + create_session 设备激活
├── drive/         # 网盘：详情 / 默认 / 列表 / 容量
├── file/          # 列表 / 搜索 / 复制 / 移动 / 上传（秒传+分片+URL 续期）/ 下载（断点续传）
├── share/         # 分享：创建 / 管理 / 保存他人分享 / QuickShare / CustomShare（aligo:// 协议）
├── user/          # 用户信息
├── types/         # 全部数据模型
└── invoker/       # 子包共享调用接口（打破循环依赖）
```

依赖单向：`types ← invoker ← 各 service 子包 ← 主包 alipan`，无循环。

## 接口约定

- access_token 自动刷新并持久化；过期由 SDK 内部处理，调用方不感知。
- 上传断点续传状态跨进程持久化（resume_store）。
- 2026-09-12 实测：正式分享 Create 的根因约束——**备份盘不能分享**。

## 常用命令

```bash
cd alipan && go build ./... && go vet ./... && go test ./...
```

## 模块特有规矩

- 设备签名参数（secp256k1）参与风控，签名逻辑改动必须真机验证。
- 代码审查报告待补（docs/reviews/2026-09-12-alipan.md，见根 TODO）。
