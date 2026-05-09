package primp

/*
#cgo CFLAGS: -I${SRCDIR}/include
#cgo linux LDFLAGS: -L${SRCDIR}/target/release -lprimp_go -lm -ldl -lpthread
#cgo darwin LDFLAGS: -L${SRCDIR}/target/release -lprimp_go -framework Security -framework CoreFoundation
#cgo windows LDFLAGS: -L${SRCDIR}/target/release -lprimp_go -lws2_32 -luserenv -lntdll -lbcrypt

#include <stdlib.h>
#include <string.h>
#include "primp.h"
*/
import "C"

import (
	"unsafe"
)

// 本文件是 cgo 包装层:仅做 Go 类型 ↔ C 类型转换,不暴露给用户。

// cstr 把 Go string 转成 *C.char(用 C.CString 分配,需配套 C.free 释放)。
// 空字符串返回 nil 指针,语义为"未设置"。
func cstr(s string) *C.char {
	if s == "" {
		return nil
	}
	return C.CString(s)
}

// freeCStr 释放 cstr 分配的指针(对 nil 安全)。
func freeCStr(p *C.char) {
	if p != nil {
		C.free(unsafe.Pointer(p))
	}
}

// goString 把由 Rust 端通过 primp_string_free 配套分配的 *C.char 转成 Go string,
// 并在转换后调用 primp_string_free 释放。
func goString(p *C.char) string {
	if p == nil {
		return ""
	}
	s := C.GoString(p)
	C.primp_string_free(p)
	return s
}

// goBytes 拷贝 [ptr, ptr+len) 字节为 Go []byte。
func goBytes(ptr *C.uint8_t, length C.size_t) []byte {
	if ptr == nil || length == 0 {
		return nil
	}
	return C.GoBytes(unsafe.Pointer(ptr), C.int(length))
}

// errFromHandle 把 Rust 端返回的 *ErrorHandle 转成 Go *PrimpError 并释放原句柄。
// 入参为 nil 时返回 nil。
func errFromHandle(eh *C.PrimpErrorHandle) error {
	if eh == nil {
		return nil
	}
	defer C.primp_error_free(eh)
	return &PrimpError{
		Kind:    ErrorKind(C.primp_error_kind(eh)),
		Status:  uint16(C.primp_error_status(eh)),
		Message: goString(C.primp_error_message(eh)),
	}
}
