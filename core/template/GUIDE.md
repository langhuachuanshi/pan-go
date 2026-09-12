# 新网盘模块接入指南（template）

> 目标：新增一个网盘模块（或把现有模块迁到 core 之上）时，只写"方言"，不写基建。
> 三层各司其职：httpx 管执行、errors 管语义、invoker 管菜单。

## 1. 目录骨架（照抄这个形状）

```
<网盘>/
├── go.mod              # module github.com/langhuachuanshi/pan-go/<网盘>
│                       # require github.com/langhuachuanshi/pan-go/core
├── client.go           # 主包：Config/New + Service 访问器 + 实现 core/invoker
├── <业务子包>/          # file/share/upload/download/...（依赖 invoker 接口，不 import 主包）
├── AGENTS.md / README.md / API.md / CHANGELOG.md   # 四件套
```

## 2. 主包 Client 要做的四件事

```go
type Client struct {
    exec *httpx.Executor   // core 执行器
    base string             // 模块 API base（如 https://drive-pc.quark.cn/1/clouddrive）
    // ...模块自有状态：cookie/token/签名缓存
}

func New(ctx context.Context, ...) (*Client, error) {
    return &Client{exec: httpx.New(httpx.Config{
        UserAgent: "<该网盘的 UA>",   // 方言：各网盘 UA 不同
        Timeout:   30 * time.Second,
    }), ...}, nil
}
```

1. **拼请求**：base + path + 公共 query（如夸克 `pr=ucpro&fr=pc`）——方言
2. **注鉴权**：cookie 拼头 / Authorization / 签名参数——方言，放进 `Request.Headers`
3. **执行**：`exec.Do(ctx, &httpx.Request{...})`——基建，core 已管
4. **判错**：HTTP ≥400 → `coreerrors.New("<网盘>", 0, status, msg, KindOther)`；
   业务码（code/errno/zt）按下方映射表转成带 Kind 的统一错误——方言中的"译码"环节

## 3. 业务码 → Kind 映射表（接入时登记，写进本模块 AGENTS）

| 语义 | 本网盘码 | 说明 |
|---|---|---|
| KindAuth（登录态失效） | 例：夸克 31001/31003 | 引导重新登录 |
| KindRateLimited | 例：夸克 41013 | 限频 |
| KindNotFound | 例：夸克 31005 | 资源不存在 |
| KindDenied | 实测补充 | 风控/无权限 |
| KindOther | 兜底 | 未分类 |

## 4. 业务子包写法（统一菜单）

```go
type Service struct{ inv coreinvoker.Invoker }   // 只认 core 标准菜单
func New(inv coreinvoker.Invoker) *Service { return &Service{inv: inv} }

func (s *Service) List(ctx context.Context, ...) ([]*MyFile, error) {
    var resp listResponse
    // path 相对 base；判错/解码由主包实现兜底
    if err := s.inv.Get(ctx, "/file/sort", map[string]string{"pdir_fid": "0"}, &resp); err != nil {
        return nil, err
    }
    if resp.Code != 0 {   // 业务码判定=方言，构造带 Kind 的统一错误
        return nil, coreerrors.New("<网盘>", resp.Code, 200, resp.Msg, kindOf(resp.Code))
    }
    ...
}
```

模块特有能力（如蓝奏的挑战页、alipan 的额外 header）：
在模块内定义 `type localInvoker interface { coreinvoker.Invoker; ExtraMethod(...) }`，
业务子包需要加菜时依赖 localInvoker，标准部分永远走 core 菜单。

## 5. 四件套与收尾 checklist

- [ ] `AGENTS.md`：概览/目录/接口约定（含判错映射表）/常用命令/特有规矩
- [ ] `README.md`：安装 + 获取凭证 + 最小示例
- [ ] `API.md`：api-docs 八段式（用 `.agents/skills/api-docs` 技能写）
- [ ] `CHANGELOG.md`：首条 [待定] 记录
- [ ] `go build ./... && go vet ./... && go test ./...` 全过
- [ ] **每个接口真机实测**后，实测结论写进 AGENTS（pan-go 的实测文化）
- [ ] 根 README 模块表加一行
