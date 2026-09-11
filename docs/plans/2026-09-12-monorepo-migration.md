# pan-go 统一仓库迁移计划

- 状态：**待用户批准**（批准前不动任何代码）
- 日期：2026-09-12
- 相关仓库：alipan-go、baidupan-go、lanzou-go、quark-go → 合并为 `D:\Project\pan-go`

## 已确认的决定

1. 仓库名 `pan-go`，本地位置 `D:\Project\pan-go`。
2. 纳入 4 个网盘模块（GitHub 全量盘点过，无遗漏）：alipan / baidu / lanzou / quark。
   `090cq` 工作台是使用方，不纳入。
3. 顶层统一：`docs/`、`example/`、`AGENTS.md`、`README.md`、`TODO.md`、`.gitignore`。
4. 任务清单命名 `TODO.md`；接口参考文档（API.md / CHANGELOG.md / INTERFACE.md）
   统一挪进 `docs/`。
5. lanzou 的 `_example/` 改名 `example/`，接受编译检查。
6. **计划先行**：本计划批准后才动手；执行中在本文件勾选进度。

## 【待确认 ①】module 方案

背景修正：原 M1 设想"保留四个旧 module 名"在 GitHub 上不可行——`go get`
按 module 名同名仓库取代码，不会感知代码搬进了 pan-go。合并仓库必然
更换一次 import 门牌号。

| | 方案 S：单 module（推荐） | 方案 M：多 module 嵌套 |
|---|---|---|
| go.mod | 1 份：`module github.com/langhuachuanshi/pan-go` | 4 份：`.../pan-go/alipan-go` 等 |
| import 示例 | `.../pan-go/quark/file` | `.../pan-go/quark-go/quark/file`（三段冗余） |
| 发版 | 一套 tag `vX.Y.Z` 管全部 | 每模块带前缀 tag（`quark-go/v0.2.0`） |
| 本地开发 | 直接 `go build ./...` | 需 go.work 或逐模块构建 |
| 依赖 | alipan 的 3 个第三方依赖成为模块公共依赖 | 各模块自持 |
| 适用 | 独立开发者、模块同步演进（= 现状） | 需要严格按模块独立发版 |

推荐 **方案 S**：四个包名（alipan/baidu/lanzou/quark）互不冲突、目录级完全
独立，"分别管理"体现在目录与文档上；心智和维护成本最低。

## 【待确认 ②】远端可见性

baidupan-go 当前是**私有**仓库，其提交历史并入后：
- pan-go 建为**私有**（默认，推荐）：无泄露风险，想开源时再评估。
- pan-go 建为公开：会连带公开 baidupan 全部历史，需明确接受。

## 目标结构（按方案 S）

```
pan-go/
├── go.mod                  # module github.com/langhuachuanshi/pan-go（go 1.26.4）
├── alipan/                 # 主包 + auth/device/drive/file/invoker/share/types/user
├── baidu/                  # 主包 + auth/clouddl/download/file/management/...
├── lanzou/                 # 14 个 .go + 2 个 _test 自根平铺迁入
├── quark/                  # 主包 + auth/download/file/invoker/qrcode/share/types/upload
├── example/
│   ├── alipan/  baidu/  lanzou/  quark/    # 各模块示例（lanzou 自 _example 迁入）
├── docs/
│   ├── plans/              # 本计划 + 后续开发计划
│   ├── reviews/            # 四模块代码审查报告（alipan 待补）
│   ├── lanzou/             # API.md、CHANGELOG.md
│   └── quark/              # INTERFACE.md
├── AGENTS.md               # 公共准则（吸收四模块现有 AGENTS 的模块约定）
├── README.md               # 总览：四模块导航 + 各自安装/快速开始链接
├── TODO.md                 # 任务索引（勾选清单，详情指向 docs/plans）
└── .gitignore              # 四模块并集
```

## 迁移步骤

1. [ ] `git subtree add` 依次并入四仓库 main 分支（保留完整提交历史，
       先落位到 `alipan-go/`、`baidupan-go/`、`lanzou-go/`、`quark-go/` 子目录）。
2. [ ] 结构重排（单独一次提交，纯 `git mv`，历史可追溯）：
   - `alipan-go/alipan → alipan`，`alipan-go/example → example/alipan`
   - `baidupan-go/baidu → baidu`，`baidupan-go/example → example/baidu`
   - `quark-go/quark → quark`，`quark-go/example → example/quark`
   - `lanzou-go/*.go + *_test.go → lanzou/`，`lanzou-go/_example → example/lanzou`
   - 文档归位：`lanzou-go/API.md、CHANGELOG.md → docs/lanzou/`；
     `quark-go/INTERFACE.md → docs/quark/`；README 全部并入根 README 后删除
3. [ ] 新建 `go.mod`（go 1.26.4）并全量替换 import（4 条映射）：
   - `github.com/langhuachuanzhi/alipan-go/alipan → .../pan-go/alipan`
   - `github.com/langhuachuanshi/baidupan-go/baidu → .../pan-go/baidu`
   - `github.com/langhuachuanshi/lanzou-go → .../pan-go/lanzou`
   - `github.com/langhuachuanshi/quark-go/quark → .../pan-go/quark`
   - 覆盖范围：所有示例源码、包注释示例、README/docs 中的代码块
4. [ ] 根文件落位：`AGENTS.md`（☑ 已按 vibe-coding 公共模板 v1.0 实例化并提交，
   迁移完成后原样生效）；`README.md`（总览：四模块导航 +
   安装示例）；`TODO.md`；`.gitignore`（四份并集）；三份审查报告收编至
   `docs/reviews/`（baidupan / lanzou / quark，alipan 待补）。
   各模块目录另放模块级 `AGENTS.md`（内容取自四份旧模块 AGENTS 拆分归位，
   只写"换个模块就会变"的内容）。
5. [ ] 验证：`go build ./... && go vet ./... && go test ./...` 全过；
   `grep` 确认无旧 import 残留；四个 example 子目录均可编译。
6. [ ] 补 alipan 模块审查报告（沿用 P0-P2 模板），修复计划继承各模块
   未完成项并汇入 TODO.md。
7. [ ] 远端：GitHub 创建 `langhuachuanshi/pan-go`（可见性见待确认②），
   push main。首个 tag（按日期版本号规则 `pan-go/v26.37.x`）**待用户明确
   指示后再打**。
8. [ ] 旧仓库收尾：四个旧仓库 README 顶部加"已迁移至 pan-go"说明后归档
   （archive；旧地址仍可访问，lanzou 旧 tag 历史在那里可查）。

## 风险与对策

| 风险 | 对策 |
|---|---|
| import 门牌号一次性变更 | 现阶段外部用户≈0，是最便宜的时点；旧仓库归档前一直可访问 |
| baidupan 私有历史并入 | 仓库默认私有（待确认②） |
| 多仓库 .gitignore 差异漏盖 | 取四份并集，覆盖 cookie/凭据/构建产物/IDE/OS |
| 历史丢失 | subtree add 完整保留；lanzou 旧 tag（v0.2.0~v0.3.2）不迁移，原仓库可查，CHANGELOG.md 已有完整记录 |

## 验收清单

- [ ] build / vet / test 全绿（含全部 example）
- [ ] 全仓 grep 无旧 module 路径残留
- [ ] 根四件套 + docs/{plans,reviews,lanzou,quark} 到位
- [ ] 远端 push 成功且 tag `pan-go/v26.37.1` 存在
- [ ] 四个旧仓库已加迁移说明并归档
