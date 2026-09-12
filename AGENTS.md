# AGENTS.md

## 项目概览

pan-go：四个网盘的 Go SDK 统一仓库（monorepo）——阿里云盘（alipan）、百度网盘（baidu）、蓝奏云（lanzou）、夸克网盘（quark）。各网盘目录级完全独立、互不依赖；鉴权模型各不相同（token+签名 / BDUSS cookie /cookie+反爬挑战 / cookie+扫码登录）。

## 目录结构

```
pan-go/                 # 多模块 monorepo：根目录无 go.mod，靠 go.work 聚合
├── go.work             # 本地开发工作区（core + 四个网盘模块）
├── core/               # 公共基础层 module：httpx 执行器 / errors 统一语义 / invoker 菜单 / template 接入指南
├── alipan/             # 阿里云盘 module：go.mod + 主包（client/option/error）
│                       #   + auth/device/drive/file/invoker/share/types/user
├── baidu/              # 百度网盘 module：go.mod + 主包
│                       #   + auth/clouddl/download/file/management/share/upload/user/qrcode/panhome/sign/types/invoker
├── lanzou/             # 蓝奏云 module：主包（client/http/challenge/stream/config/errors）
│                       #   + invoker/account/file/folder/upload/download/recycle/resolve
├── quark/              # 夸克 module：go.mod + 主包
│                       #   + auth/download/file/invoker/qrcode/share/types/upload
│                       #   + INTERFACE.md（接口参考索引）
├── .agents/skills/     # 仓库自带技能（api-docs：接口文档写作规范）
└── docs/
    ├── plans/          # 开发/迁移计划
    └── reviews/        # 各模块代码审查报告
```

每个模块：module 路径 `github.com/langhuachuanshi/pan-go/<网盘>`，模块根即主包；
三件套（README.md / API.md / CHANGELOG.md）与模块 AGENTS.md 都在模块根目录。

## 常用命令

```bash
go build all      # 仓库根：工作区全量构建（依赖 go.work）
# 质量门（逐模块；go vet/test all 会连带第三方依赖自己的测试，不作门禁）
for m in alipan baidu lanzou quark; do (cd $m && go vet ./... && go test ./...); done
```

## 项目约定

- **多模块隔离**：四个网盘各自独立 go.mod、独立发版（tag 加网盘前缀，如
  `quark/v26.37.1`），互不 import；跨模块共享的思路各自实现 invoker 模式，不建公共包。
- **实测文化**：接口行为以真机实测为准，结论写进提交信息、模块 AGENTS 与
  docs/；未实测的推断要标注"未验证"。
- **凭据红线**：各网盘 cookie/密钥只走环境变量或本地文件，绝不入库。
- 各网盘的协议细节、坑与红线见各模块 `AGENTS.md`（lanzou 的挑战参数、
  百度与 openapi 分支不混用、夸克先查 docs/quark/INTERFACE.md 等）。
- baidupan 历史原为私有：本仓库保持 private，公开前需重新评估。

## 注释

- **只写代码表达不了的信息**：为什么这么做、约束、坑、实测结论；不复述代码行为。
- **默认一行**：超过三行的注释，先考虑改代码（换名字、拆函数），而不是继续写注释。
- **导出符号必须有一行 godoc 注释**（以标识符名开头）；包注释讲清职责与用法即可。
- **同步红线**：改代码必须同步改注释；过时注释直接删，不留历史。
- 注释用中文，标识符与专有名词保持原文。

## 文档与计划

- `README.md`：每个仓库根一份，只放项目定位、安装、快速开始、文档导航；深入内容一律进 docs/。
- `docs/plans/`：开发 / 重构计划。**计划先行**——多步任务先落计划再动手，计划不落盘不动工；每项任务带验收标准与 checkbox，执行中更新勾选，完成后文首标注完成日期。
- `docs/reviews/`：代码审查报告，文件名 `YYYY-MM-DD-<范围>.md`。固定六节：范围 / 方法 / 结论摘要 / 发现 / 修复计划 / 勘误记录。发现按 P0（正确性·安全）/ P1（应尽快）/ P2（建议）分级，每条 = 位置 + 依据 + 处置；已修标 ☑；写错的发现进"勘误记录"撤销，不删除。
- 接口参考等深入文档放 `docs/<模块或主题>/`，不放根目录。

## TODO

`TODO.md` 是根目录任务索引。**想到就记**：开发中冒出的想法、还没做成计划的待办，直接按格式追加，不要求先写计划书。

```markdown
# TODO

## 待办
- [ ] 一句话说清做什么（背景 / 来源，可选）

## 进行中
- [ ] 任务（计划：docs/plans/<主题>.md）

## 已完成
- [x] 任务（2026-09-12 完成）
```

- 想法 → 待办：一句话即可，零门槛入账。
- 待办 → 开发：先把计划落到 `docs/plans/`，条目补上计划链接，移入"进行中"。
- 完成：打勾标日期移入"已完成"；有产出的（报告 / 计划）把链接写进条目。

## 更新日志

- 对外发布的仓库根放 `CHANGELOG.md`，格式统一中文：

  ```markdown
  # 更新日志

  ## [0.4.0] - 2026-09-12

  ### 新增
  - 条目

  ### 修复
  - 条目

  ### 变更 / 移除
  - 条目
  ```

- 版本条目从上到下按时间倒序，最新在最上。
- 条目与 tag 一一对应：**先补条目，后打 tag**。

## Tag 规范

- 版本号格式：`[前缀/]v<年>.<周>.<序>`——x=年份后两位（2026 → 26），y=当年第几周（ISO 周号），z=该周内第几个版本，从 1 顺加。示例：`v26.37.1`。
- 多模块仓库**只打模块 tag**（`quark/v26.37.1`），不打仓库级 tag——Go 工具链只认模块前缀，仓库级版本会制造"所有模块同步发版"的假耦合。
- 发 tag 前置：build / vet / test 全过；更新日志已有本次发版条目（先补条目，后打 tag）。

## 提交与发版

- **主动提交**：每完成一个计划项（或一个可独立验证的任务）就主动 commit，不等用户提醒；阶段性收尾主动 push（用户明确说"先不推"的除外）。
- **提交信息**：`<类型>(<范围>): 中文描述`，类型取 feat / fix / docs / refactor / test / chore；一次提交只说一件事。
- **提交质量线**：提交前 build / vet / test 全过，不提交半成品；收尾时工作区不留未提交的改动。
- **tag / 发版必须等明确指令**：只有用户说出"发布新版本""打 tag"之类的明确指示才进入发版流程，**绝不主动打 tag**。
- **发版流程**（用户下达指令后按序执行）：
  1. 确定版本号：`v<年2位>.<周>.<周内序号>`（多网盘 / 多模块仓库加类型前缀）。
  2. 文档同步：依据自上次 tag 以来的全部提交，写 CHANGELOG 条目（中文，标题带刚确定的版本号），同步 API 文档、README、docs/ 下 wiki 类文档——固定动作，不需要用户额外交代。
  3. 验证：build / vet / test 全过。
  4. 提交文档同步（`docs:` 提交）。
  5. 打 tag（指向该提交），push 提交与 tag 到远端，发布完成。

## 行为准则

以下准则偏向谨慎而非速度，目的是减少常见编码失误。

1. **先想清楚再动手**：动手前说出假设，不确定就问；有多种理解列出来让用户选；有更简单的方案直说；不清楚就停下来指出哪里不明白。
2. **简单优先**：用最少的代码解决问题；不加没要求的功能与"灵活性"；不为不可能的场景做错误处理；能用 50 行就不写 200 行。
3. **精准修改**：只动该动的——不顺手改进旁边的代码，不重构没坏的东西，匹配已有风格；只清理自己改动产生的孤立代码；每一行改动都应能追溯到需求。
4. **目标驱动**：把任务转成可验证目标（加校验 → 先写测试让它通过；修 bug → 先写复现测试；重构 → 前后测试都过）；多步任务先列"步骤 → 验证检查点"。
