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
- [x] 主包保留：Client（实现 invoker.Invoker）、Config/Option、errors.go 哨兵错误
- [x] 新增 `lanzou/invoker/`：Invoker 接口（Get/Post/PostMultipart/PostMultipartStream/
      FetchPageWithChallenge/TaskURL/UploadURL/AjaxmURL/Vei 等）
- [x] Service 子包：`account/`（Info/Detail）、`file/`（List/ShareURL/Move/Delete/SetPassword）、
      `folder/`、`upload/`（File/Stream/ByURL）、`download/`（File/FileAuto/Dir/ByURL）、
      `recycle/`、`resolve/`（直链解析）
- [x] 对外 API 对齐其他模块风格：`c.Files().List(...)` 等 Service 访问器；
      Login/Logout 为会话生命周期，保留在主包 Client
- [x] 现有单测迁移：account/stream 测试随包保留并通过
- [x] 验证：build / vet / test 全过
- 验收：lanzou 目录形态与其余三模块一致；无平铺业务文件残留 ✅（提交 d8916e9）

### 阶段 2：文档随模块走 + 三件套建档
- [x] `docs/lanzou/API.md、CHANGELOG.md → lanzou/`（API.md 由 api-docs 版重写）
- [x] `docs/quark/INTERFACE.md → quark/`
- [x] 四模块 `CHANGELOG.md`：lanzou 平移历史并补迁移条目；baidu/quark/alipan 首建迁移条目
      （版本号"待定"，随首次打 tag 定稿）
- [x] 根 README 瘦身为纯导航；根 docs/ 只剩 plans/、reviews/
- 验收：根 docs/ 无网盘子目录；每模块三件套齐全 ✅（提交 3b6d8de）

### 阶段 3：API.md ×4（api-docs 技能，分批）
- [x] lanzou/API.md（配合阶段 1 新 API，全量）
- [x] quark/API.md（全量）
- [x] baidu/API.md（核心方法 + 长尾表格）
- [x] alipan/API.md（核心方法 + 长尾表格，长尾细节后续续写）
- 验收：八段式结构（Go 适配：方法签名/调用示例/参数表/返回值/失败/注意）；
  全局约定一页 + 每方法一节 ✅

### 阶段 4：收尾
- [x] 全量验证（逐模块 build/vet/test + 旧路径 grep 清零）
- [x] 根 TODO.md 更新
- [ ] 提交按规范拆分（refactor / docs），push（tag 仍等用户明确指令）
- [ ] 全局 api-docs 技能移除（完成"移动"语义），或按用户指示保留
