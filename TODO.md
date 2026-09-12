# TODO

## 进行中

- [ ] 旧仓库远端删库：token 缺 `delete_repo` scope，需先执行
      `gh auth refresh -h github.com -s delete_repo`（浏览器授权）后再删
      alipan-go / baidupan-go / lanzou-go / quark-go 四个远端仓库
- [ ] 本地 `/d/Project/lanzou-go` 目录被其他程序占用，关闭占用程序后手动删除

## 待办

- [ ] backend / workbench 等使用方统一切换到 pan-go 模块（替换旧 quark-go 依赖；
      import 路径 `.../quark-go/quark` → `.../pan-go/quark`，API 不变仅换路径，自动获得分页修复）
      ——**前置：打首个 tag**（外部项目 go get 嵌套模块需要 `quark/v26.x.y` 形式的 tag）
- [ ] 补 alipan 模块代码审查报告（docs/reviews/2026-09-12-alipan.md）
- [ ] 通用网盘基础模块（core）统一架构：计划已定稿（docs/plans/2026-09-12-core-architecture.md v2，5 个决策点已定），待批准后按 6 阶段执行
- [ ] 各模块审查报告 P1 / P2 修复项逐项落地（docs/reviews/2026-09-12-baidupan / lanzou / quark.md）
- [ ] API.md 分批补全：alipan（方法最多）与 baidu 的长尾方法待续写
- [ ] 首个 tag（pan-go/v26.37.x）：等用户明确指令，按发版流程执行

## 已完成

- [x] 四仓库盘点、统一方案确定、公共规范体系落地（2026-09-12）
- [x] monorepo 迁移：subtree 历史合并 + 多模块重排 + import 替换 + example 删除（2026-09-12）
- [x] lanzou 重构为 invoker + Service 子包结构，与另三模块统一（2026-09-12）
- [x] 文档随模块走：四模块 README/CHANGELOG/API.md 三件套 + 模块 AGENTS（2026-09-12）
- [x] 本地旧仓库清理：alipan-go / baidupan-go / quark-go 已删，lanzou-go 被占用待手动（2026-09-12）
- [x] api-docs 技能移入仓库（.agents/skills/api-docs/），全局份已移除（2026-09-12）
