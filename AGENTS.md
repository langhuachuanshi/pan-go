# AGENTS.md

## 项目概览

百度网盘网页端（抓包）Go SDK。基于 BDUSS + STOKEN cookie 鉴权（网页登录态），
走 pan.baidu.com 网页端 REST API。与 openapi 分支（OAuth + /xpan/*）是两套
完全不同的鉴权模型，不要混用。模块名 `github.com/langhuachuanshi/baidupan-go`。

## 目录结构

```
baidupan-go/
├── baidu/             主包 Client（实现 invoker.Invoker）+ Config
├── baidu/auth/        BDUSS/STOKEN 凭证管理 + bdstoken 获取
├── baidu/file/        列表/搜索/元信息/递归列目录
├── baidu/management/  建目录/复制/移动/重命名/删除/回收站
├── baidu/upload/      分片上传（precreate→superfile2→create，含秒传）
├── baidu/download/    下载（dlink/流式）
├── baidu/share/       创建分享/管理 + 转存他人分享
├── baidu/clouddl/     离线下载
├── baidu/user/        容量/用户信息/登录态探活
├── baidu/qrcode/      扫码登录（换发 BDUSS/STOKEN）
├── baidu/panhome/     sign/bdstoken 缓存
├── baidu/sign/        签名算法（Sign2 / locate download 签名）
├── baidu/types/       数据模型
├── baidu/invoker/     共享调用接口
└── example/           实测脚本（test_*），需真实凭证
```

## 常用命令

```bash
go build ./...
go test ./...        # 单元测试（不依赖真实账号）
go vet ./...
PANBAIDU_BDUSS=xxx PANBAIDU_STOKEN=yyy go run ./example/test_list   # 联调实测
```

## 架构与约定

- **invoker 接口模式**（同 quark-go/alipan-go）：业务子包依赖 `invoker.Invoker`
  接口，主包 `Client` 实现，避免循环依赖。新增子包照此办理。
- **接口约定**（均实测确认，详见 `baidu/client.go` 包注释）：接口全走
  `pan.baidu.com`（pcs 域名仅用于分片上传 superfile2）；query 统一带
  `channel=chunlei&web=1&app_id=250528&clienttype=0`；写操作额外带 bdstoken
  （auth 包自动获取）和 `Referer: https://pan.baidu.com/disk/main`。
- **凭证**：BDUSS 所有接口必需，STOKEN 写操作必需；扫码登录
  （`baidu/qrcode`）可换发两者。
- **实测文化**：接口行为以真机实测为准，结论写进提交信息与代码注释；
  example/test_* 就是实测脚本，新功能先补实测。
- **文档形态**：当前无 README，接口说明以各包 doc comment 与 example/ 为准；
  两者须随实现同步更新。

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
