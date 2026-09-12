# 更新日志

## [待定] - 2026-09-12

### 新增

- 首建 core 模块：`httpx`（HTTP 执行器：JSONBody/FormBody/RawBody、传输层重试、请求日志）、
  `errors`（统一 APIError + Kind 语义 + IsAuth/IsRateLimited/IsNotFound/IsDenied）、
  `invoker`（业务子包统一调用接口）、`template`（新网盘接入指南）
- 单测：httpx 七场景（query/JSON/Form/重试/重试耗尽/日志/UA）+ errors 四场景
