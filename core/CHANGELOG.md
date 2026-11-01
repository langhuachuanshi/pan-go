# 更新日志

## [0.2645.1] - 2026-11-02

### 变更

- `go` 指令由 1.26.4 降到 1.20：使用方可用 Go 1.20 工具链编译（Win7 客户端硬约束）。
  代码未使用 1.21+ 语言特性与标准库 API。

## [0.2637.1] - 2026-09-12

### 新增

- 首建 core 模块：`httpx`（HTTP 执行器：JSONBody/FormBody/RawBody、传输层重试、请求日志）、
  `errors`（统一 APIError + Kind 语义 + IsAuth/IsRateLimited/IsNotFound/IsDenied）、
  `invoker`（业务子包统一调用接口）、`template`（新网盘接入指南）
- 单测：httpx 七场景（query/JSON/Form/重试/重试耗尽/日志/UA）+ errors 四场景
