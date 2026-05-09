# primp-go

[primp](https://github.com/deedy5/primp) HTTP 客户端的 **Go 语言绑定** —— 核心仍是 Rust,通过 C ABI + cgo 暴露给 Go。

设计目标:在 Go 端获得与 [`primp-python`](https://github.com/deedy5/primp/tree/main/crates/primp-python) 类似的使用体验,同时遵循 Go 习惯:

- ✅ **真正的命名枚举**:`Impersonate` / `ImpersonateOS` / `Method` 都是 `int32` 命名常量(如 `primp.ImpersonateChromeV146`),**不是字符串**。编译期就能拼写检查、IDE 跳转。
- ✅ **同步阻塞 API**:`client.Get(url)` 直接返回 `*Response`;并发由调用方用 `go` 关键字驱动。
- ✅ **函数式选项**:用 `WithJSON` / `WithHeaders` / `WithTimeout` 等覆盖 Python 关键字参数风格。
- ✅ **完整中文注释**。
- ✅ **支持 Chrome / Edge / Safari / Firefox / Opera 多版本指纹模拟,支持 Android / iOS / Linux / macOS / Windows 系统指纹**。

---

## 安装与构建

primp-go 是 cgo 包,需要先构建出 Rust 动态库。

### 1. 拉取代码

```bash
git clone https://github.com/pll177/primp-go.git
cd primp-go
```

或者通过 `go get`:

```bash
go get github.com/pll177/primp-go
```

(`go get` 只会下载源码,**仍需手动编译 Rust 库** —— 见下一步)

### 2. 编译 Rust 动态库

> 需要 Rust toolchain ≥ 1.84(`rustup toolchain install stable`)

```bash
cargo build --release
```

产物路径:
- Linux:   `target/release/libprimp_go.so`
- macOS:   `target/release/libprimp_go.dylib`
- Windows: `target\release\primp_go.dll`(同时生成 `primp_go.dll.lib` 给 cgo 链接)
- 头文件: `include/primp.h`(由 cbindgen 自动生成)

第一次编译会从 [deedy5/primp](https://github.com/deedy5/primp) 拉取上游 primp Rust crate,耗时较长(数分钟),之后增量构建很快。

### 3. 在 Go 代码中使用

```bash
go build ./...
go test ./...                   # 联网测试,会访问 httpbin.org
go run ./examples/basic
```

`ffi.go` 已通过 `#cgo LDFLAGS` 自动指向 `./target/release` 目录,**只要在仓库根目录跑 `go build` 即可**,无需手工设置 LD_LIBRARY_PATH。

如果要把 primp-go 作为依赖被别的 Go 项目使用,需要把 `target/release/libprimp_go.*` 与 `include/primp.h` 一起分发,或者在你的项目里再次执行 `cargo build --release`。

---

## 用法速览

```go
package main

import (
    "fmt"
    "log"
    "time"

    primp "github.com/pll177/primp-go"
)

func main() {
    // 1. 创建 Client(同步,支持浏览器指纹模拟)
    client, err := primp.NewClient(primp.ClientOptions{
        Impersonate:   primp.ImpersonateChromeV146,
        ImpersonateOS: primp.ImpersonateOSWindows,
        Timeout:       10 * time.Second,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // 2. 发送请求(函数式选项)
    resp, err := client.Post("https://httpbin.org/post",
        primp.WithJSON(map[string]any{"hello": "世界"}),
        primp.WithHeaders(map[string]string{"X-Trace": "abc"}),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Close()

    fmt.Println(resp.StatusCode(), resp.URL())
    var body map[string]any
    _ = resp.JSON(&body)
}
```

### 模块级便捷函数

无需创建 `Client`,适合一次性调用:

```go
resp, err := primp.Get("https://httpbin.org/get")
resp, err := primp.Post("https://httpbin.org/post", primp.WithJSON(payload))
resp, err := primp.RequestOnce(primp.MethodPATCH, url, primp.WithContent(body))
```

### 错误处理

```go
import "errors"

_, err := client.Get(url)
switch {
case errors.Is(err, primp.ErrTimeout):
    // 超时
case errors.Is(err, primp.ErrConnect):
    // 连接失败
case errors.Is(err, primp.ErrStatus):
    var pe *primp.PrimpError
    errors.As(err, &pe)
    fmt.Println("HTTP", pe.Status, pe.Message)
}
```

完整错误类型:`ErrBuilder` / `ErrRequest` / `ErrConnect` / `ErrTimeout` / `ErrStatus` / `ErrRedirect` / `ErrBody` / `ErrDecode` / `ErrUpgrade` / `ErrGeneric`。

### 并发

primp-go 是同步 API,要并发就用 goroutine + WaitGroup/channel:

```go
client, _ := primp.NewClient(primp.ClientOptions{Impersonate: primp.ImpersonateChrome})
defer client.Close()

var wg sync.WaitGroup
for _, u := range urls {
    wg.Add(1)
    go func(u string) {
        defer wg.Done()
        resp, err := client.Get(u)
        if err != nil { return }
        defer resp.Close()
        process(resp.Bytes())
    }(u)
}
wg.Wait()
```

`Client` 是线程安全的(内部 RwLock 保护),可在多个 goroutine 间共享。

---

## 浏览器指纹枚举速查

| 常量                          | 含义                              |
|-------------------------------|----------------------------------|
| `ImpersonateNone`             | 不启用模拟(默认零值)             |
| `ImpersonateChrome`           | 最新 Chrome                       |
| `ImpersonateChromeV144/145/146`| 指定 Chrome 版本                |
| `ImpersonateEdge` / `EdgeV144..146` | Edge                       |
| `ImpersonateSafari` / `SafariV185/V26/V263` | Safari            |
| `ImpersonateFirefox` / `FirefoxV140/V146/V147/V148` | Firefox    |
| `ImpersonateOpera` / `OperaV126..129` | Opera                    |
| `ImpersonateRandom`           | 随机一个                          |

OS 指纹:`ImpersonateOSAndroid` / `IOS` / `Linux` / `MacOS` / `Windows` / `Random` / `None`。

---

## 与 primp-python 的对照

| Python                                | Go                                                          |
|---------------------------------------|-------------------------------------------------------------|
| `Client(impersonate="chrome_146")`    | `NewClient(ClientOptions{Impersonate: ImpersonateChromeV146})` |
| `client.get(url, params=...)`         | `client.Get(url, WithParams(...))`                          |
| `client.post(url, json=payload)`      | `client.Post(url, WithJSON(payload))`                       |
| `r.json()` / `r.text` / `r.content`   | `r.JSON(&v)` / `r.Text()` / `r.Bytes()`                     |
| `r.raise_for_status()`                | `r.RaiseForStatus()`                                        |
| `TimeoutError`                        | `errors.Is(err, primp.ErrTimeout)`                          |
| `with Client() as c: ...`             | `defer c.Close()`                                           |

---

## 项目结构

```
primp-go/
├── Cargo.toml          # Rust crate (cdylib + staticlib)
├── build.rs            # cbindgen 生成 include/primp.h
├── cbindgen.toml
├── src/                # Rust FFI 源码
│   ├── lib.rs          # extern "C" 导出
│   ├── enums.rs        # i32 ↔ primp 原生枚举映射
│   ├── error.rs        # 错误分类(对应 Go ErrorKind)
│   ├── ffi.rs          # 字符串/JSON 辅助
│   └── runtime.rs      # 全局 tokio runtime
├── include/primp.h     # cbindgen 自动生成(.gitignore)
├── go.mod              # Go module: github.com/pll177/primp-go
├── enums.go            # Method / Impersonate / ImpersonateOS 命名枚举
├── errors.go           # PrimpError / ErrKindXxx
├── ffi.go              # cgo 包装层(内部)
├── primp.go            # 公开 API:Client / NewClient / Get / Post / ...
├── response.go         # Response 类型
├── primp_test.go       # 集成测试(联网,httpbin)
└── examples/basic/main.go
```

---

## 当前不支持(后续版本)

- 流式响应(`iter_bytes` / `iter_lines`)
- multipart 文件上传(`files=`)
- `AsyncClient`(异步语义在 Go 中不必要 —— 直接 `go client.Get(...)` 即可)

## 协议

MIT。primp 上游版权归 [deedy5](https://github.com/deedy5/primp) 所有。
