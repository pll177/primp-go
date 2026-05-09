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
	"time"
	"unsafe"
)

// =============================================================================
// 客户端
// =============================================================================

// BasicAuth 基本认证(用户名 + 可选密码)。
type BasicAuth struct {
	Username string
	Password string // 留空表示仅传用户名
}

// ClientOptions 创建 [Client] 的配置项,字段含义参考 primp-python 的 `Client.__init__`。
//
// 零值即合理默认:
//   - CookieStore / Referer / FollowRedirects / Verify 默认关闭(false),
//     使用 NewClient 时会按 Python 习惯自动启用,通过 Set*Disabled 字段显式关掉。
//
// 注意:为了让 Go 端零值即可用,这里把"关掉某项"用单独 *Disabled 字段表达,
// 避免出现"明明没设但被默认值覆盖"的反直觉行为。
type ClientOptions struct {
	Auth       *BasicAuth        // 基本认证
	AuthBearer string            // Bearer Token
	Params     map[string]string // 默认查询参数
	Headers    map[string]string // 默认请求头(若设置了 Impersonate 会被覆盖)
	Cookies    map[string]string // 初始 cookie

	CookieStoreDisabled bool // true 表示禁用 cookie 持久化(默认启用)
	RefererDisabled     bool // true 表示禁用自动 Referer(默认启用)
	VerifyDisabled      bool // true 表示禁用 TLS 证书校验(默认开启)
	RedirectsDisabled   bool // true 表示禁用自动跟随重定向(默认启用)

	Proxy          string        // 代理 URL
	Timeout        time.Duration // 总超时
	ConnectTimeout time.Duration // 连接超时
	ReadTimeout    time.Duration // 读超时

	Impersonate   Impersonate   // 浏览器指纹枚举(默认 ImpersonateNone)
	ImpersonateOS ImpersonateOS // 操作系统指纹枚举(默认 ImpersonateOSNone)

	MaxRedirects int    // 最大重定向次数(默认 20)
	CACertFile   string // 自定义 CA 证书路径
	HTTPSOnly    bool   // 仅允许 HTTPS
	HTTP2Only    bool   // 仅使用 HTTP/2
	BaseURL      string // 相对 URL 拼接基址
}

// Client HTTP 客户端,可模拟主流浏览器指纹。
//
// 必须用 [NewClient] 创建,使用完毕后调用 [Client.Close] 释放底层 Rust 句柄。
type Client struct {
	mu     sync.Mutex
	handle *C.PrimpClientHandle
}

// NewClient 用给定配置创建客户端。
func NewClient(opts ClientOptions) (*Client, error) {
	// 把字段编码成 C 结构体所需的 JSON 字符串与原生标量。
	headersJSON, err := encodeMap(opts.Headers)
	if err != nil {
		return nil, &PrimpError{Kind: ErrKindBuilder, Message: "headers 序列化失败: " + err.Error()}
	}
	paramsJSON, err := encodeMap(opts.Params)
	if err != nil {
		return nil, &PrimpError{Kind: ErrKindBuilder, Message: "params 序列化失败: " + err.Error()}
	}
	cookiesJSON, err := encodeMap(opts.Cookies)
	if err != nil {
		return nil, &PrimpError{Kind: ErrKindBuilder, Message: "cookies 序列化失败: " + err.Error()}
	}

	// 用 C.malloc 分配一组 C 字符串,函数末尾统一释放。
	cstrs := newCStrPool()
	defer cstrs.freeAll()

	var authUser, authPass string
	if opts.Auth != nil {
		authUser = opts.Auth.Username
		authPass = opts.Auth.Password
	}

	maxRedirects := opts.MaxRedirects
	if maxRedirects <= 0 {
		maxRedirects = 20
	}

	cOpts := C.PrimpClientOptionsC{
		auth_username:        cstrs.add(authUser),
		auth_password:        cstrs.add(authPass),
		auth_bearer:          cstrs.add(opts.AuthBearer),
		params_json:          cstrs.add(paramsJSON),
		headers_json:         cstrs.add(headersJSON),
		cookies_json:         cstrs.add(cookiesJSON),
		cookie_store:         C.bool(!opts.CookieStoreDisabled),
		referer:              C.bool(!opts.RefererDisabled),
		proxy:                cstrs.add(opts.Proxy),
		timeout_secs:         C.double(opts.Timeout.Seconds()),
		connect_timeout_secs: C.double(opts.ConnectTimeout.Seconds()),
		read_timeout_secs:    C.double(opts.ReadTimeout.Seconds()),
		impersonate:          C.int32_t(opts.Impersonate),
		impersonate_os:       C.int32_t(opts.ImpersonateOS),
		follow_redirects:     C.bool(!opts.RedirectsDisabled),
		max_redirects:        C.uint32_t(maxRedirects),
		verify:               C.bool(!opts.VerifyDisabled),
		ca_cert_file:         cstrs.add(opts.CACertFile),
		https_only:           C.bool(opts.HTTPSOnly),
		http2_only:           C.bool(opts.HTTP2Only),
		base_url:             cstrs.add(opts.BaseURL),
	}

	var errH *C.PrimpErrorHandle
	h := C.primp_client_new(&cOpts, &errH)
	if h == nil {
		return nil, errFromHandle(errH)
	}

	c := &Client{handle: h}
	// 设置 finalizer 兜底,避免用户忘记 Close 时泄漏。
	runtime.SetFinalizer(c, func(c *Client) { _ = c.Close() })
	return c, nil
}

// Close 释放客户端句柄,可重入(再次调用是空操作)。
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handle != nil {
		C.primp_client_free(c.handle)
		c.handle = nil
		runtime.SetFinalizer(c, nil)
	}
	return nil
}

// Headers 返回客户端默认请求头(快照)。
func (c *Client) Headers() (map[string]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handle == nil {
		return nil, &PrimpError{Kind: ErrKindBuilder, Message: "client 已关闭"}
	}
	js := goString(C.primp_client_headers_json(c.handle))
	if js == "" {
		return map[string]string{}, nil
	}
	m := map[string]string{}
	if err := json.Unmarshal([]byte(js), &m); err != nil {
		return nil, &PrimpError{Kind: ErrKindGeneric, Message: "headers JSON 解析失败: " + err.Error()}
	}
	return m, nil
}

// SetHeaders 替换或合并客户端默认请求头。
//
// replace=true 时清空原有头部;false 时仅追加/覆盖同名键。
func (c *Client) SetHeaders(h map[string]string, replace bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handle == nil {
		return &PrimpError{Kind: ErrKindBuilder, Message: "client 已关闭"}
	}
	js, err := encodeMap(h)
	if err != nil {
		return &PrimpError{Kind: ErrKindBuilder, Message: err.Error()}
	}
	cs := cstr(js)
	defer freeCStr(cs)
	var errH *C.PrimpErrorHandle
	rc := C.primp_client_set_headers(c.handle, cs, C.bool(replace), &errH)
	if rc != 0 {
		return errFromHandle(errH)
	}
	return nil
}

// =============================================================================
// 请求选项
// =============================================================================

// requestConfig 是 RequestOption 内部使用的可变配置。
type requestConfig struct {
	params      map[string]string
	headers     map[string]string
	cookies     map[string]string
	body        []byte
	jsonBody    any
	formData    any
	auth        *BasicAuth
	authBearer  string
	timeout     time.Duration
	readTimeout time.Duration

	// 重定向覆盖:0=不覆盖,1=强制开启,2=强制关闭
	followOverride int32
}

// RequestOption 函数式选项,在 Client.Request 等方法中使用。
type RequestOption func(*requestConfig)

// WithParams 设置查询参数,会与 URL 自带的参数合并。
func WithParams(m map[string]string) RequestOption {
	return func(c *requestConfig) { c.params = m }
}

// WithHeaders 设置本次请求的额外请求头。
func WithHeaders(m map[string]string) RequestOption {
	return func(c *requestConfig) { c.headers = m }
}

// WithCookies 设置本次请求附带的 cookies(会写入 cookie store)。
func WithCookies(m map[string]string) RequestOption {
	return func(c *requestConfig) { c.cookies = m }
}

// WithContent 用原始字节作为请求体。
func WithContent(b []byte) RequestOption {
	return func(c *requestConfig) { c.body = b }
}

// WithJSON 用任意 Go 值作为 JSON 请求体(自动设置 Content-Type)。
func WithJSON(v any) RequestOption {
	return func(c *requestConfig) { c.jsonBody = v }
}

// WithFormData 用 map / struct 作为 application/x-www-form-urlencoded 请求体。
func WithFormData(v any) RequestOption {
	return func(c *requestConfig) { c.formData = v }
}

// WithBasicAuth 添加 HTTP 基本认证。
func WithBasicAuth(user, pass string) RequestOption {
	return func(c *requestConfig) { c.auth = &BasicAuth{Username: user, Password: pass} }
}

// WithBearerAuth 添加 Bearer Token 认证。
func WithBearerAuth(token string) RequestOption {
	return func(c *requestConfig) { c.authBearer = token }
}

// WithTimeout 覆盖本次请求的总超时。
func WithTimeout(d time.Duration) RequestOption {
	return func(c *requestConfig) { c.timeout = d }
}

// WithReadTimeout 覆盖本次请求的读超时。
func WithReadTimeout(d time.Duration) RequestOption {
	return func(c *requestConfig) { c.readTimeout = d }
}

// WithFollowRedirects 覆盖客户端层面的重定向策略。
func WithFollowRedirects(follow bool) RequestOption {
	return func(c *requestConfig) {
		if follow {
			c.followOverride = 1
		} else {
			c.followOverride = 2
		}
	}
}

// =============================================================================
// 请求方法
// =============================================================================

// Request 用任意 HTTP method 发起请求。
func (c *Client) Request(method Method, url string, opts ...RequestOption) (*Response, error) {
	cfg := &requestConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	c.mu.Lock()
	if c.handle == nil {
		c.mu.Unlock()
		return nil, &PrimpError{Kind: ErrKindBuilder, Message: "client 已关闭"}
	}
	handle := c.handle
	c.mu.Unlock()

	paramsJSON, err := encodeMap(cfg.params)
	if err != nil {
		return nil, &PrimpError{Kind: ErrKindBuilder, Message: "params 序列化失败: " + err.Error()}
	}
	headersJSON, err := encodeMap(cfg.headers)
	if err != nil {
		return nil, &PrimpError{Kind: ErrKindBuilder, Message: "headers 序列化失败: " + err.Error()}
	}
	cookiesJSON, err := encodeMap(cfg.cookies)
	if err != nil {
		return nil, &PrimpError{Kind: ErrKindBuilder, Message: "cookies 序列化失败: " + err.Error()}
	}

	var jsonBody string
	if cfg.jsonBody != nil {
		b, err := json.Marshal(cfg.jsonBody)
		if err != nil {
			return nil, &PrimpError{Kind: ErrKindBuilder, Message: "JSON body 序列化失败: " + err.Error()}
		}
		jsonBody = string(b)
	}
	var formJSON string
	if cfg.formData != nil {
		b, err := json.Marshal(cfg.formData)
		if err != nil {
			return nil, &PrimpError{Kind: ErrKindBuilder, Message: "form 序列化失败: " + err.Error()}
		}
		formJSON = string(b)
	}

	var authUser, authPass string
	if cfg.auth != nil {
		authUser = cfg.auth.Username
		authPass = cfg.auth.Password
	}

	cstrs := newCStrPool()
	defer cstrs.freeAll()

	// 把请求体拷到 C 堆,避免把 Go 指针放进 cgo struct 触发
	// "argument of cgo function has Go pointer to unpinned Go pointer" panic。
	// cgo 规则:传给 C 的 Go 指针(&cReq)里不能再嵌套指向 Go 内存的指针,
	// 所以这里用 C.CBytes 复制一份到 C 堆,函数返回前 C.free 掉。
	var bodyCMem unsafe.Pointer
	var bodyLen C.size_t
	if len(cfg.body) > 0 {
		bodyCMem = C.CBytes(cfg.body) // C.malloc + memcpy
		defer C.free(bodyCMem)
		bodyLen = C.size_t(len(cfg.body))
	}

	cReq := C.PrimpRequestParamsC{
		params_json:               cstrs.add(paramsJSON),
		headers_json:              cstrs.add(headersJSON),
		cookies_json:              cstrs.add(cookiesJSON),
		body_ptr:                  (*C.uint8_t)(bodyCMem),
		body_len:                  bodyLen,
		json_body:                 cstrs.add(jsonBody),
		form_data_json:            cstrs.add(formJSON),
		auth_username:             cstrs.add(authUser),
		auth_password:             cstrs.add(authPass),
		auth_bearer:               cstrs.add(cfg.authBearer),
		timeout_secs:              C.double(cfg.timeout.Seconds()),
		read_timeout_secs:         C.double(cfg.readTimeout.Seconds()),
		follow_redirects_override: C.int32_t(cfg.followOverride),
	}

	cURL := cstr(url)
	defer freeCStr(cURL)

	var respH *C.PrimpResponseHandle
	var errH *C.PrimpErrorHandle
	rc := C.primp_request(handle, C.int32_t(method), cURL, &cReq, &respH, &errH)
	if rc != 0 {
		return nil, errFromHandle(errH)
	}
	return newResponseFromHandle(respH), nil
}

// Get 发送 GET 请求。
func (c *Client) Get(url string, opts ...RequestOption) (*Response, error) {
	return c.Request(MethodGET, url, opts...)
}

// Head 发送 HEAD 请求。
func (c *Client) Head(url string, opts ...RequestOption) (*Response, error) {
	return c.Request(MethodHEAD, url, opts...)
}

// Options 发送 OPTIONS 请求。
func (c *Client) Options(url string, opts ...RequestOption) (*Response, error) {
	return c.Request(MethodOPTIONS, url, opts...)
}

// Delete 发送 DELETE 请求。
func (c *Client) Delete(url string, opts ...RequestOption) (*Response, error) {
	return c.Request(MethodDELETE, url, opts...)
}

// Post 发送 POST 请求。
func (c *Client) Post(url string, opts ...RequestOption) (*Response, error) {
	return c.Request(MethodPOST, url, opts...)
}

// Put 发送 PUT 请求。
func (c *Client) Put(url string, opts ...RequestOption) (*Response, error) {
	return c.Request(MethodPUT, url, opts...)
}

// Patch 发送 PATCH 请求。
func (c *Client) Patch(url string, opts ...RequestOption) (*Response, error) {
	return c.Request(MethodPATCH, url, opts...)
}

// =============================================================================
// 模块级便捷函数(每次创建临时 client)
// =============================================================================

// Get 用临时客户端发送 GET 请求。
func Get(url string, opts ...RequestOption) (*Response, error) {
	return doOnce(MethodGET, url, opts...)
}

// Post 用临时客户端发送 POST 请求。
func Post(url string, opts ...RequestOption) (*Response, error) {
	return doOnce(MethodPOST, url, opts...)
}

// Head 用临时客户端发送 HEAD 请求。
func Head(url string, opts ...RequestOption) (*Response, error) {
	return doOnce(MethodHEAD, url, opts...)
}

// Options 用临时客户端发送 OPTIONS 请求。
func OptionsRequest(url string, opts ...RequestOption) (*Response, error) {
	return doOnce(MethodOPTIONS, url, opts...)
}

// Delete 用临时客户端发送 DELETE 请求。
func Delete(url string, opts ...RequestOption) (*Response, error) {
	return doOnce(MethodDELETE, url, opts...)
}

// Put 用临时客户端发送 PUT 请求。
func Put(url string, opts ...RequestOption) (*Response, error) {
	return doOnce(MethodPUT, url, opts...)
}

// Patch 用临时客户端发送 PATCH 请求。
func Patch(url string, opts ...RequestOption) (*Response, error) {
	return doOnce(MethodPATCH, url, opts...)
}

// RequestOnce 用临时客户端发送任意方法的请求。
func RequestOnce(method Method, url string, opts ...RequestOption) (*Response, error) {
	return doOnce(method, url, opts...)
}

func doOnce(method Method, url string, opts ...RequestOption) (*Response, error) {
	c, err := NewClient(ClientOptions{})
	if err != nil {
		return nil, err
	}
	defer c.Close()
	return c.Request(method, url, opts...)
}

// =============================================================================
// 辅助
// =============================================================================

// encodeMap 把 map[string]string 序列化为 JSON 字符串;空 map 返回空串(表示未设置)。
func encodeMap(m map[string]string) (string, error) {
	if len(m) == 0 {
		return "", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// cstrPool 管理一组通过 C.CString 分配的指针,统一释放。
type cstrPool struct {
	ptrs []*C.char
}

func newCStrPool() *cstrPool {
	return &cstrPool{ptrs: make([]*C.char, 0, 16)}
}

func (p *cstrPool) add(s string) *C.char {
	if s == "" {
		return nil
	}
	c := C.CString(s)
	p.ptrs = append(p.ptrs, c)
	return c
}

func (p *cstrPool) freeAll() {
	for _, c := range p.ptrs {
		C.free(unsafe.Pointer(c))
	}
	p.ptrs = nil
}
