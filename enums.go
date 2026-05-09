//go:build windows && amd64

// Package primp 是 primp HTTP 客户端的 Go 绑定,核心由 Rust 实现。
//
// 本文件定义命名枚举(Method / Impersonate / ImpersonateOS),
// 数值必须与 Rust 端 `crates/primp-go/src/enums.rs` 严格一致。
package primp

// Method 表示 HTTP 请求方法的枚举。
//
// 这是真正的命名枚举:每个常量都有自己的标识符,
// 而不是字符串("GET"/"POST" 之类),从而获得编译期校验与跳转支持。
type Method int32

const (
	MethodGET     Method = 0 // GET 请求
	MethodHEAD    Method = 1 // HEAD 请求
	MethodOPTIONS Method = 2 // OPTIONS 请求
	MethodDELETE  Method = 3 // DELETE 请求
	MethodPOST    Method = 4 // POST 请求
	MethodPUT     Method = 5 // PUT 请求
	MethodPATCH   Method = 6 // PATCH 请求
)

// String 返回方法的文本形式。
func (m Method) String() string {
	switch m {
	case MethodGET:
		return "GET"
	case MethodHEAD:
		return "HEAD"
	case MethodOPTIONS:
		return "OPTIONS"
	case MethodDELETE:
		return "DELETE"
	case MethodPOST:
		return "POST"
	case MethodPUT:
		return "PUT"
	case MethodPATCH:
		return "PATCH"
	default:
		return "UNKNOWN"
	}
}

// Impersonate 表示需要模拟的浏览器指纹枚举。
//
// **零值 ImpersonateNone(=0)代表不启用浏览器指纹模拟**,
// 这样 ClientOptions 字面量中省略 Impersonate 字段也能得到合理默认。
//
// 数值是稀疏分布的(给浏览器家族留版本扩展空间),
// 不要依赖具体数字,始终用具名常量传参。
type Impersonate int32

const (
	// ImpersonateNone 表示不启用浏览器指纹模拟(默认零值)。
	ImpersonateNone Impersonate = 0

	// Chrome 系列
	ImpersonateChrome     Impersonate = 1 // 最新 Chrome
	ImpersonateChromeV144 Impersonate = 2
	ImpersonateChromeV145 Impersonate = 3
	ImpersonateChromeV146 Impersonate = 4

	// Edge 系列
	ImpersonateEdge     Impersonate = 11
	ImpersonateEdgeV144 Impersonate = 12
	ImpersonateEdgeV145 Impersonate = 13
	ImpersonateEdgeV146 Impersonate = 14

	// Safari 系列
	ImpersonateSafari     Impersonate = 21
	ImpersonateSafariV185 Impersonate = 22 // Safari 18.5
	ImpersonateSafariV26  Impersonate = 23
	ImpersonateSafariV263 Impersonate = 24 // Safari 26.3

	// Firefox 系列
	ImpersonateFirefox     Impersonate = 31
	ImpersonateFirefoxV140 Impersonate = 32
	ImpersonateFirefoxV146 Impersonate = 33
	ImpersonateFirefoxV147 Impersonate = 34
	ImpersonateFirefoxV148 Impersonate = 35

	// Opera 系列
	ImpersonateOpera     Impersonate = 41
	ImpersonateOperaV126 Impersonate = 42
	ImpersonateOperaV127 Impersonate = 43
	ImpersonateOperaV128 Impersonate = 44
	ImpersonateOperaV129 Impersonate = 45

	// ImpersonateRandom 表示从所有可用指纹中随机选择。
	ImpersonateRandom Impersonate = 99
)

// ImpersonateOS 表示需要模拟的操作系统指纹枚举。
//
// 零值 ImpersonateOSNone(=0)代表不指定 OS。
type ImpersonateOS int32

const (
	// ImpersonateOSNone 不指定 OS 指纹(默认零值,由 impersonate 决定)。
	ImpersonateOSNone ImpersonateOS = 0

	ImpersonateOSAndroid ImpersonateOS = 1
	ImpersonateOSIOS     ImpersonateOS = 2
	ImpersonateOSLinux   ImpersonateOS = 3
	ImpersonateOSMacOS   ImpersonateOS = 4
	ImpersonateOSWindows ImpersonateOS = 5

	// ImpersonateOSRandom 随机选择一个 OS。
	ImpersonateOSRandom ImpersonateOS = 99
)
