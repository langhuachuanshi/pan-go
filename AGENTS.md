# AGENTS.md

## 项目概览

夸克网盘的 Go SDK。基于 cookie 鉴权（无 token、无签名），支持扫码登录
（`quark/qrcode`）、文件管理、上传（秒传+分片直传 OSS）、下载、创建分享、
转存他人分享。模块名 `github.com/langhuachuanshi/quark-go`。
接口真相的参考索引在 `INTERFACE.md`，查接口/补功能/查 bug 先看那里。

## 目录结构

```
quark-go/
├── quark/            主包 Client（实现 invoker.Invoker）
├── quark/auth/       cookie 管理 + 持久化（~/.quark/cookie.json）
├── quark/file/       文件列表/详情/管理
├── quark/share/      创建分享 + 转存 + 聚合发货
├── quark/upload/     上传（秒传 + 分片直传 OSS）
├── quark/download/   下载
├── quark/qrcode/     扫码登录（换发登录 cookie）
├── quark/types/      数据模型
├── quark/invoker/    共享调用接口与 APIError
└── example/          实测脚本（test_*），多数需要真实 cookie
```

## 常用命令

```bash
go build ./...
go vet ./...
go run ./example/quickstart          # 联调示例，需要 ~/.quark/cookie.json 或环境提供 cookie
go run ./example/test_qrlogin        # 扫码登录实测，无需已有 cookie
```

## 架构与约定

- **invoker 接口模式**（同 alipan-go）：业务子包依赖 `invoker.Invoker` 接口，
  主包 `Client` 实现，避免循环依赖。新增子包照此办理。
- **接口约定**：base `https://drive-pc.quark.cn/1/clouddrive`，公共 query
  `pr=ucpro&fr=pc`（`uqm` 会被当游客返回 31001）；响应 `{status, code, message, ...}`，
  `code==0 && status==200` 才算成功。详见 `INTERFACE.md`。
- **错误**：统一 `invoker.APIError`；登录态失效用 `invoker.IsAuthError`
  （31001 未登录 / 31003 cookie 失效）判断，引导重新扫码登录。
- **cookie 校验**：`__puus` 或 `__pus` 任一存在即有效（扫码换发的 cookie 只有
  `__pus`，drive 接口实测认）；新 cookie 的 `__puus` 回写要合并进本地 cookie。
- **实测文化**：接口行为以真机实测为准，结论写进提交信息和 `INTERFACE.md`；
  未实测的推断要标注"未验证"。
- **文档同步**：对外行为变更须同步 `README.md`（使用）与 `INTERFACE.md`
  （接口索引/已知差异）。

## 行为准则
以下准则偏向谨慎而非速度，目的是减少常见编码失误。

### 1. 先想清楚再动手
不要想当然。不要掩盖困惑。主动暴露取舍。
动手之前：
- 明确说出你的假设，不确定就问
- 如果有多种理解，列出来让用户选，不要自己悄悄选一个
- 如果存在更简单的方案，说出来。该反驳就反驳
- 遇到不清楚的地方，停下来。指出哪里不明白，然后问

### 2. 简单优先
用最少的代码解决问题。不要写猜测性的代码。
- 不加没要求的功能
- 一次性使用的代码不做抽象
- 没要求的"灵活性"或"可配置性"不加
- 不可能发生的场景不做错误处理
- 如果写了 200 行但 50 行就能搞定，重写
- 问自己："资深工程师会觉得这太复杂了吗？" 如果是，简化

### 3. 精准修改
只动该动的。只清理自己弄乱的。
编辑已有代码时：
- 不要"顺手改进"旁边的代码、注释或格式
- 不要重构没坏的东西
- 匹配已有风格，即使你习惯不同
- 发现无关的死代码，提一句就行，别删
你的改动产生的孤立代码：
- 删除你的改动导致不再使用的 import/变量/函数
- 不要删除之前就存在的死代码，除非被要求
检验标准：每一行改动都应该能追溯到用户的需求。

### 4. 目标驱动
定义成功标准，循环直到验证通过。
把任务转化为可验证的目标：
- "加校验" → "为无效输入写测试，然后让测试通过"
- "修 bug" → "写一个能复现的测试，然后让它通过"
- "重构 X" → "确保重构前后测试都能通过"
多步骤任务，简要列个计划：
1. [步骤] → 验证：[检查点]
2. [步骤] → 验证：[检查点]
3. [步骤] → 验证：[检查点]
