# TODO

## 进行中

- [ ] backend / workbench 切换 pan-go@v0.2637.1（import 路径替换即可，分页修复自动到手）

## 待办

- [ ] backend / workbench 等使用方统一切换到 pan-go 模块（替换旧 quark-go 依赖；
      import 路径 `.../quark-go/quark` → `.../pan-go/quark`，API 不变仅换路径，自动获得分页修复）
      ——**前置：打首个 tag**（外部项目 go get 嵌套模块需要 `quark/v26.x.y` 形式的 tag）
- [ ] 补 alipan 模块代码审查报告（docs/reviews/2026-09-12-alipan.md）
- [ ] core 统一架构收尾二期：alipan 错误语义映射（字符串码表）、分片上传骨架（触发条件见计划决议 3）
- [ ] 各模块审查报告 P1 / P2 修复项逐项落地（docs/reviews/2026-09-12-baidupan / lanzou / quark.md）
- [ ] API.md 分批补全：alipan（方法最多）与 baidu 的长尾方法待续写


## 已完成

- [x] 远端旧仓库删除：alipan-go / baidupan-go / lanzou-go / quark-go 已从 GitHub 移除（2026-09-12）

- [x] 首个 tag：core/alipan/baidu/lanzou/quark 各 v0.2637.1（2026-09-12，用户确认）

- [x] core 统一架构：core 模块落地 + 四模块迁移完成（lanzou/quark/baidu 全嵌菜单，alipan 仅执行层，2026-09-12）

- [x] 四仓库盘点、统一方案确定、公共规范体系落地（2026-09-12）
- [x] monorepo 迁移：subtree 历史合并 + 多模块重排 + import 替换 + example 删除（2026-09-12）
- [x] lanzou 重构为 invoker + Service 子包结构，与另三模块统一（2026-09-12）
- [x] 文档随模块走：四模块 README/CHANGELOG/API.md 三件套 + 模块 AGENTS（2026-09-12）
- [x] 仓库转公开（用户批准开源，公开前全历史凭据扫描 0 泄露）（2026-09-12）
- [x] 本地旧仓库清理：alipan-go / baidupan-go / quark-go 已删；lanzou-go 内容已清空，
      仅剩空壳目录被某程序工作目录占用，程序退出后可删（2026-09-12）
- [x] api-docs 技能移入仓库（.agents/skills/api-docs/），全局份已移除（2026-09-12）
