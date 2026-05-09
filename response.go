//go:build windows && amd64

package primp

/*
#include <stdlib.h>
#include "primp.h"
*/
import "C"

import (
	"encoding/json"
	"runtime"
	"sync"
	"unsafe"
)

// Response 表示一次 HTTP 响应,封装 Rust 端的 PrimpResponseHandle。
//
// 必须用 [Response.Close] 释放底层资源(或依赖 GC finalizer)。
type Response struct {
	mu     sync.Mutex
	handle *C.PrimpResponseHandle

	// 缓存,首次访问时填充。
	statusCode int
	url        string
	encoding   string
	body       []byte
	headers    map[string]string
	cookies    map[string]string

	loaded bool
}

func newResponseFromHandle(h *C.PrimpResponseHandle) *Response {
	r := &Response{handle: h}
	runtime.SetFinalizer(r, func(r *Response) { _ = r.Close() })
	return r
}

func (r *Response) load() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.loaded || r.handle == nil {
		return
	}
	r.statusCode = int(C.primp_response_status(r.handle))
	r.url = goString(C.primp_response_url(r.handle))
	r.encoding = goString(C.primp_response_encoding(r.handle))

	// body
	var bodyLen C.size_t
	bodyPtr := C.primp_response_body(r.handle, &bodyLen)
	if bodyPtr != nil && bodyLen > 0 {
		r.body = C.GoBytes(unsafe.Pointer(bodyPtr), C.int(bodyLen))
	} else {
		r.body = []byte{}
	}

	// headers / cookies
	if hs := goString(C.primp_response_headers_json(r.handle)); hs != "" {
		_ = json.Unmarshal([]byte(hs), &r.headers)
	}
	if cs := goString(C.primp_response_cookies_json(r.handle)); cs != "" {
		_ = json.Unmarshal([]byte(cs), &r.cookies)
	}
	if r.headers == nil {
		r.headers = map[string]string{}
	}
	if r.cookies == nil {
		r.cookies = map[string]string{}
	}

	r.loaded = true
}

// StatusCode 返回 HTTP 状态码。
func (r *Response) StatusCode() int { r.load(); return r.statusCode }

// URL 返回最终响应 URL(经历重定向后的)。
func (r *Response) URL() string { r.load(); return r.url }

// Encoding 返回响应解码所用的字符集名,如 "UTF-8"。
func (r *Response) Encoding() string { r.load(); return r.encoding }

// Bytes 返回响应体原始字节。
func (r *Response) Bytes() []byte { r.load(); return r.body }

// Text 把响应体按 [Response.Encoding] 解码为字符串。
//
// 当前实现仅支持 UTF-8(将来可接入 golang.org/x/text/encoding 做完整解码)。
func (r *Response) Text() string {
	r.load()
	return string(r.body)
}

// Headers 返回响应头快照(map 拷贝)。
func (r *Response) Headers() map[string]string {
	r.load()
	out := make(map[string]string, len(r.headers))
	for k, v := range r.headers {
		out[k] = v
	}
	return out
}

// Cookies 返回响应中提取的 cookies 快照。
func (r *Response) Cookies() map[string]string {
	r.load()
	out := make(map[string]string, len(r.cookies))
	for k, v := range r.cookies {
		out[k] = v
	}
	return out
}

// JSON 把响应体反序列化到 v(v 应为指针)。
func (r *Response) JSON(v any) error {
	r.load()
	if err := json.Unmarshal(r.body, v); err != nil {
		return &PrimpError{Kind: ErrKindDecode, Message: "JSON 反序列化失败: " + err.Error()}
	}
	return nil
}

// RaiseForStatus 当状态码为 4xx/5xx 时返回 ErrKindStatus 错误。
func (r *Response) RaiseForStatus() error {
	r.load()
	if r.statusCode >= 400 {
		return &PrimpError{
			Kind:    ErrKindStatus,
			Status:  uint16(r.statusCode),
			Message: "HTTP " + http3DigitString(r.statusCode) + " for URL " + r.url,
		}
	}
	return nil
}

// Close 释放底层句柄,可重入。
func (r *Response) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.handle != nil {
		C.primp_response_free(r.handle)
		r.handle = nil
		runtime.SetFinalizer(r, nil)
	}
	return nil
}

// http3DigitString 把状态码格式化为三位字符串(避免引入 fmt 的额外开销)。
func http3DigitString(code int) string {
	if code < 0 || code > 999 {
		return "???"
	}
	buf := []byte{
		byte('0' + code/100),
		byte('0' + (code/10)%10),
		byte('0' + code%10),
	}
	return string(buf)
}
