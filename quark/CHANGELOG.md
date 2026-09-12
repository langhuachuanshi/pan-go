# 更新日志

## [待定] - 2026-09-12

### 修复

- `Files().List` 自动分页在夸克返回 `total=0` 时第一页即截断（大目录只能取到第一页），
  补跑/重跑场景会定位失败或误建目录。改为仅 `total>0` 时按 total 提前停，
  其余靠短页/空页兜底；补三个分页单测（原 quark-go v0.2.1 修复未随移植带入，本次补回）

### 变更

- 仓库并入 pan-go 多模块统一仓库，import 路径变更为
  `github.com/langhuachuanshi/pan-go/quark`（破坏性）
