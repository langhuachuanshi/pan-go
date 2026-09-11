# 文档统一 + lanzou 结构改造 计划

- 状态：执行中（2026-09-12 用户确认）
- 前置决议：① lanzou 引入 invoker + Service 模式对齐其他三模块（破坏性 API 变更，已确认做）；
  ② 根 README 瘦身为纯导航，模块用法进各模块 README；③ quark/INTERFACE.md 保留在 quark 根，
  另用 api-docs 技能新写面向使用者的 quark/API.md；④ API.md 分批写（先核心后全量）；
  ⑤ baidu/quark/alipan 的 CHANGELOG 首建只写迁移条目；⑥ api-docs 技能移入
  `.agents/skills/api-docs/` 随仓库打包（已复制，全局份在文档写完后移除）。

## 目标结构（每模块统一）

```
<模块>/
├── go.mod
├── <主包 + 子包>          # lanzou 从平铺改造为 invoker + Service 子包
├── AGENTS.md              # 模块规范（已有）
├── README.md              # 模块用法入口（新增）
├── API.md                 # api-docs 八段式接口文档（新增）
└── CHANGELOG.md           # 模块更新日志（lanzou 平移现有，其余首建迁移条目）
```

根目录保留：README.md（纯导航）、AGENTS.md、TODO.md、go.work、.agents/skills/、
docs/plans/、docs/reviews/。docs/<网盘>/ 取消，内容随模块走。

## 阶段

### 阶段 1：lanzou 结构改造（破坏性，单独提交）
- [ ] 主包保留：Client（实现 invoker.Invoker）、Config/Option、errors.go 哨兵错误
- [ ] 新增 `lanzou/invoker/`：Invoker 接口（Get/Post/PostMultipart/fetchPageWithChallenge 能力）
- [ ] Service 子包：`account/`（Login/Logout/User/AccountInfo）、`file/`（列表/分享链接/
      移动/删除/设密码）、`folder/`、`upload/`（含流式）、`download/`、`recycle/`、`resolve/`（直链解析）
- [ ] 对外 API 对齐其他模块风格：`c.Files().List(...)` 等 Service 访问器
- [ ] 现有单测迁移到对应子包并保持通过
- [ ] 验证：build / vet / test 全过
- 验收：lanzou 目录形态与其余三模块一致；无平铺业务文件残留

### 阶段 2：文档随模块走 + 三件套建档
- [ ] `docs/lanzou/API.md、CHANGELOG.md → lanzou/`（API.md 后续被 api-docs 版取代/合并）
- [ ] `docs/quark/INTERFACE.md → quark/`
- [ ] 四模块 `CHANGELOG.md`：lanzou 平移历史；baidu/quark/alipan 首建写迁移条目
- [ ] 根 README 瘦身为纯导航；根 docs/ 只剩 plans/、reviews/
- 验收：根 docs/ 无网盘子目录；每模块三件套齐全

### 阶段 3：API.md ×4（api-docs 技能，分批）
- [ ] lanzou/API.md（配合阶段 1 新 API）
- [ ] quark/API.md
- [ ] baidu/API.md
- [ ] alipan/API.md（方法最多，放最后）
- 验收：每份都是八段式结构；全局约定一页 + 每接口一节；不与 README 重复

### 阶段 4：收尾
- [ ] 全量验证（逐模块 build/vet/test）
- [ ] 根 TODO.md 更新
- [ ] 提交按规范拆分（refactor / docs），push（tag 仍等用户明确指令）
- [ ] 全局 api-docs 技能移除（完成"移动"语义），或按用户指示保留
