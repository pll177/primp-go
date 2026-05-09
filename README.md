# primp-go (Windows-only)

[primp](https://github.com/deedy5/primp) HTTP 客户端的 **Go 语言绑定** —— 核心是 Rust,通过 C ABI + cgo 暴露给 Go。

> ⚠️ **当前版本仅支持 Windows amd64**。在其它平台上构建会得到 `build constraints exclude all Go files` 的报错。

设计目标:在 Go 端获得与 [`primp-python`](https://github.com/deedy5/primp/tree/main/crates/primp-python) 类似的使用体验,同时遵循 Go 习惯:

- ✅ **真正的命名枚举**:`Impersonate` / `ImpersonateOS` / `Method` 都是 `int32` 命名常量(如 `primp.ImpersonateChromeV146`),**不是字符串**。编译期就能拼写检查、IDE 跳转。
- ✅ **同步阻塞 API**:`client.Get(url)` 直接返回 `*Response`;并发由调用方用 `go` 关键字驱动。
- ✅ **函数式选项**:用 `WithJSON` / `WithHeaders` / `WithTimeout` 等覆盖 Python 关键字参数风格。
- ✅ **完整中文注释**。
- ✅ **支持 Chrome / Edge / Safari / Firefox / Opera 多版本指纹模拟**。
- ✅ **预编译静态库由 GitHub Actions 提供,调用方无需安装 Rust**。

---

## 调用方:零 Rust 依赖,直接 `go get`

仓库内已经包含由 GitHub Actions 自动构建的 Windows 静态库(`lib/windows_amd64/libprimp_go.a`)和 C 头文件(`include/primp.h`),所以**调用方只需要 Go 工具链**(自带 MinGW gcc),不需要安装 Rust:

```powershell
# 1. 在你的项目里
go get github.com/pll177/primp-go

# 2. 直接 build / run / test
go run .
```

> Go for Windows 的官方安装包自带 MinGW gcc,cgo 开箱即用。
> 如果你的环境里 cgo 不可用,先 `go env -w CGO_ENABLED=1`,并确保 PATH 里有 gcc(如装了 [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) 或 MSYS2 的 mingw64 工具链)。

### 最小用法

```go
package main

import (
    "fmt"
    "log"
    "time"

    primp "github.com/pll177/primp-go"
)

func main() {
    client, err := primp.NewClient(primp.ClientOptions{
        Impersonate:   primp.ImpersonateChromeV146,
        ImpersonateOS: primp.ImpersonateOSWindows,
        Timeout:       10 * time.Second,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    resp, err := client.Get("https://httpbin.org/get?msg=hello")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Close()

    fmt.Println("状态:", resp.StatusCode())
    fmt.Println("URL:  ", resp.URL())
    fmt.Println("UA:   ", resp.Headers()["User-Agent"])
}
```

---

## 维护者:在本地从源码构建

如果你要修改 Rust 端代码,需要 Rust toolchain ≥ 1.84:

```powershell
rustup target add x86_64-pc-windows-gnu
cargo build --release --target x86_64-pc-windows-gnu

# 把产物拷到 cgo 期望的位置
mkdir lib\windows_amd64 -ErrorAction SilentlyContinue
copy target\x86_64-pc-windows-gnu\release\libprimp_go.a lib\windows_amd64\

# 然后跑 Go 端
go build .\...
go test .\...
go run .\examples\basic
```

或者直接 push 到 main —— GitHub Actions 会自动重新编译并把新静态库 commit 回仓库。

---

## 模块级便捷函数

无需创建 `Client`,适合一次性调用:

```go
resp, err := primp.Get("https://httpbin.org/get")
resp, err := primp.Post("https://httpbin.org/post", primp.WithJSON(payload))
resp, err := primp.RequestOnce(primp.MethodPATCH, url, primp.WithContent(body))
```

## 错误处理

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

## 并发

`Client` 是线程安全的(内部 RwLock 保护),可在多个 goroutine 间共享:

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
├── Cargo.toml                # Rust crate(staticlib)
├── build.rs, cbindgen.toml   # cbindgen 生成 include/primp.h
├── src/                      # Rust FFI 源码
├── include/primp.h           # ← 由 CI 自动生成并 commit
├── lib/windows_amd64/
│   └── libprimp_go.a         # ← 由 CI 自动构建并 commit
├── go.mod
├── enums.go errors.go ffi.go primp.go response.go
├── primp_test.go             # 集成测试(联网,httpbin)
├── examples/basic/main.go
└── .github/workflows/build-libs.yml   # 自动构建 + commit
```

## 当前不支持

- 🚫 **Linux / macOS**:计划但尚未提供预编译库
- 🚫 流式响应(`iter_bytes` / `iter_lines`)
- 🚫 multipart 文件上传(`files=`)
- 🚫 `AsyncClient`(异步在 Go 中不必要,直接 `go client.Get(...)` 即可)

## 协议

MIT。primp 上游版权归 [deedy5](https://github.com/deedy5/primp) 所有。
