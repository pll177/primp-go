//go:build windows && amd64

package primp

import "fmt"

// ErrorKind 错误分类枚举,与 Rust 端 `error::ErrorKind` 数值一致。
//
// 用法:
//
//	if errors.Is(err, ErrTimeout) { ... }
//	或
//	var pe *PrimpError
//	if errors.As(err, &pe) && pe.Kind == ErrKindTimeout { ... }
type ErrorKind int32

const (
	ErrKindGeneric  ErrorKind = 99 // 兜底错误
	ErrKindBuilder  ErrorKind = 1  // 客户端构造或参数解析阶段错误
	ErrKindRequest  ErrorKind = 2  // 通用请求错误
	ErrKindConnect  ErrorKind = 3  // 建立连接失败
	ErrKindTimeout  ErrorKind = 4  // 超时
	ErrKindStatus   ErrorKind = 5  // HTTP 4xx/5xx 状态错误
	ErrKindRedirect ErrorKind = 6  // 重定向处理错误
	ErrKindBody     ErrorKind = 7  // 响应体读取错误
	ErrKindDecode   ErrorKind = 8  // 解码/解压错误
	ErrKindUpgrade  ErrorKind = 9  // 协议升级错误
)

// String 返回 kind 的文字形式,便于日志输出。
func (k ErrorKind) String() string {
	switch k {
	case ErrKindBuilder:
		return "Builder"
	case ErrKindRequest:
		return "Request"
	case ErrKindConnect:
		return "Connect"
	case ErrKindTimeout:
		return "Timeout"
	case ErrKindStatus:
		return "Status"
	case ErrKindRedirect:
		return "Redirect"
	case ErrKindBody:
		return "Body"
	case ErrKindDecode:
		return "Decode"
	case ErrKindUpgrade:
		return "Upgrade"
	case ErrKindGeneric:
		return "Generic"
	default:
		return fmt.Sprintf("Unknown(%d)", int32(k))
	}
}

// PrimpError 是所有 primp-go 错误的统一表达。
//
// 通过 errors.Is 与下方的哨兵实例比较即可判定类型:
//
//	if errors.Is(err, ErrTimeout) { ... }
type PrimpError struct {
	Kind    ErrorKind
	Message string
	// Status 仅当 Kind == ErrKindStatus 时有意义。
	Status uint16
}

// Error 实现 error 接口。
func (e *PrimpError) Error() string {
	if e.Kind == ErrKindStatus && e.Status != 0 {
		return fmt.Sprintf("primp[%s]: HTTP %d - %s", e.Kind, e.Status, e.Message)
	}
	return fmt.Sprintf("primp[%s]: %s", e.Kind, e.Message)
}

// Is 让 errors.Is 能根据 Kind 匹配哨兵错误。
func (e *PrimpError) Is(target error) bool {
	t, ok := target.(*PrimpError)
	if !ok {
		return false
	}
	return e.Kind == t.Kind
}

// 哨兵错误:用于 errors.Is 判定 kind。
//
// 注意 Message 为空,仅用作 kind 比较载体。
var (
	ErrBuilder  = &PrimpError{Kind: ErrKindBuilder}
	ErrRequest  = &PrimpError{Kind: ErrKindRequest}
	ErrConnect  = &PrimpError{Kind: ErrKindConnect}
	ErrTimeout  = &PrimpError{Kind: ErrKindTimeout}
	ErrStatus   = &PrimpError{Kind: ErrKindStatus}
	ErrRedirect = &PrimpError{Kind: ErrKindRedirect}
	ErrBody     = &PrimpError{Kind: ErrKindBody}
	ErrDecode   = &PrimpError{Kind: ErrKindDecode}
	ErrUpgrade  = &PrimpError{Kind: ErrKindUpgrade}
	ErrGeneric  = &PrimpError{Kind: ErrKindGeneric}
)
