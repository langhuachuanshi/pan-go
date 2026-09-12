# core

pan-go 的公共基础层：HTTP 执行器、统一错误语义、统一 Invoker 接口。
四个网盘模块（alipan/baidu/lanzou/quark）的实现底座；**业务方不需要直接使用本模块**
——引入任意网盘模块时 core 作为间接依赖自动拉取。

## 什么时候需要直接 import core

只有写**跨网盘通用代码**时（例如同时处理多个网盘的统一错误处理）：

```go
import coreerrors "github.com/langhuachuanshi/pan-go/core/errors"

if coreerrors.IsAuth(err) {
    // 任意网盘的登录态失效 → 引导重新登录/扫码，一份逻辑通吃
}
```

## 组成

| 包 | 职责 |
|---|---|
| `httpx` | HTTP 执行器：请求构建/编码/公共头/传输层重试/日志 |
| `errors` | 统一 `APIError` + Kind 语义 + `IsAuth` 等判定 |
| `invoker` | 业务子包依赖的统一调用接口（各模块主包实现） |
| `template` | 新网盘模块接入指南 |

## 给模块作者

实现一个新的网盘模块，或想把现有模块迁移到 core 之上：读
[template/GUIDE.md](./template/GUIDE.md)。模块约定见 [AGENTS.md](./AGENTS.md)，
版本历史见 [CHANGELOG.md](./CHANGELOG.md)。
