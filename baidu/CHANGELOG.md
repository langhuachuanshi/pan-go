# 更新日志

## [待定] - 2026-09-12

### 变更

- 仓库并入 pan-go 多模块统一仓库，import 路径变更为
  `github.com/langhuachuanshi/pan-go/baidu`（破坏性）
- HTTP 执行迁移到 `pan-go/core`（httpx/invoker 标准菜单 + baidu 方言扩展：
  GetRaw/PostFormRaw/PostMultipartForm/PostMultipart/PostFormQuery）；
  errno 错误统一 coreerrors 语义（-6→Auth），`GetAndDecode` 等助手签名不变
