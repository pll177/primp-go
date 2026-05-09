# consumer-demo

> ⚠️ Windows amd64 only

完整端到端示例:**模拟一个外部项目**用 `go get` 拉 primp-go,然后跑业务代码 + 单元测试。
设计目的是验证「调用方零 Rust 依赖,只装 Go 即可使用」流程。

## 在仓库内直接跑

```powershell
cd primp-go
go run .\examples\consumer-demo\
go test .\examples\consumer-demo\... -v
```

## 在仓库外跑(模拟真实下游)

```powershell
mkdir mytest
cd mytest
go mod init example.com/mytest

# 把 main.go 与 main_test.go 拷到这里
go get github.com/pll177/primp-go@latest

go run .            # 跑业务代码
go test -v ./...    # 跑测试
```

预期结果:

```
=== RUN   TestGet           --- PASS
=== RUN   TestImpersonate   --- PASS  (UA: Mozilla/5.0 (Windows NT 10.0...) Chrome/146.0.0.0)
=== RUN   TestTimeout       --- PASS
=== RUN   TestPostJSON      --- PASS
PASS
```

## 测试覆盖范围

| 用例 | 内容 |
|---|---|
| `TestGet` | GET 200 + JSON 反序列化 + 查询参数回显 |
| `TestImpersonate` | Chrome 146 / Windows 指纹注入到 UA |
| `TestTimeout` | 短超时触发 `errors.Is(err, primp.ErrTimeout)` |
| `TestPostJSON` | POST + `WithJSON` + 服务端回显 |

测试需要联网(httpbin.org)。
