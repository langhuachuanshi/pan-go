# 更新日志

## [26.37.1] - 2026-09-12

### 变更

- 仓库并入 pan-go 多模块统一仓库，import 路径变更为
  `github.com/langhuachuanshi/pan-go/alipan`（破坏性）
- HTTP 执行层迁移到 `pan-go/core`（httpx）；因 POST-only 协议与 core 菜单 Post
  签名冲突，本模块不嵌入标准菜单（Post* 方言保留），错误语义映射列二期
