//! primp-go: 把 primp HTTP 客户端通过 C ABI 暴露,供 Go 等语言通过 cgo 调用。
//!
//! 设计要点:
//! - 边界全部用整数 / 不透明指针 / C 字符串,不穿过任何 Rust 复合类型;
//! - headers / cookies / params 这类 map 用 JSON 字符串传输,简化 marshal;
//! - 所有阻塞调用通过 [`runtime::rt`] 全局 tokio 运行时 `block_on` 驱动;
//! - 错误统一装箱为 `PrimpFfiError`,Go 端通过 `kind` 整数还原成对应类型。

#![allow(clippy::missing_safety_doc)]

use std::os::raw::c_char;
use std::sync::{Arc, RwLock};
use std::time::Duration;

use primp::header::{HeaderMap, HeaderName, HeaderValue};
use primp::redirect::Policy;
use primp::{Body, Client as PrimpClient, Proxy, Response as PrimpResponse, Url};

mod enums;
mod error;
mod ffi;
mod runtime;

use enums::{ImpersonateC, ImpersonateOSC, MethodC};
use error::{ErrorKind, PrimpFfiError, PrimpFfiResult};
use ffi::{
    buf_to_option_vec, cookies_to_json, cstr_to_option_str, cstr_to_option_string,
    headers_to_json, parse_json_map, string_to_c,
};
use runtime::rt;

// =============================================================================
// 不透明句柄
// =============================================================================

/// 客户端句柄(对 Go 端不透明)。
pub struct PrimpClientHandle {
    inner: Arc<RwLock<PrimpClient>>,
    /// 可选的 base_url,在 request 时与相对路径拼接。
    base_url: Option<String>,
    /// 可选的全局参数(JSON map),用于无 per-request params 时的回退。
    default_params: Option<String>,
}

/// 响应句柄(对 Go 端不透明)。
pub struct PrimpResponseHandle {
    /// 已读完的响应体字节。首版直接全部读入内存。
    body: Vec<u8>,
    url: String,
    status: u16,
    headers_json: String,
    cookies_json: String,
    encoding: String,
}

/// 错误句柄(对 Go 端不透明)。
pub struct PrimpErrorHandle {
    pub kind: ErrorKind,
    pub message: String,
    pub status: u16,
}

// =============================================================================
// C 结构体
// =============================================================================

/// 客户端创建参数(由 Go 侧填充,字段对应 Python `Client.__init__`)。
///
/// 字符串字段为空指针表示"未设置";map 类字段以 JSON 字符串传入。
/// `i32` 枚举字段:`-1` 表示未设置(对应 [`ImpersonateC::None`])。
#[repr(C)]
pub struct PrimpClientOptionsC {
    pub auth_username: *const c_char,
    pub auth_password: *const c_char,
    pub auth_bearer: *const c_char,
    pub params_json: *const c_char,
    pub headers_json: *const c_char,
    pub cookies_json: *const c_char,
    pub cookie_store: bool,
    pub referer: bool,
    pub proxy: *const c_char,
    /// 总超时,秒;0 表示未设置。
    pub timeout_secs: f64,
    pub connect_timeout_secs: f64,
    pub read_timeout_secs: f64,
    pub impersonate: i32,
    pub impersonate_os: i32,
    pub follow_redirects: bool,
    pub max_redirects: u32,
    pub verify: bool,
    pub ca_cert_file: *const c_char,
    pub https_only: bool,
    pub http2_only: bool,
    pub base_url: *const c_char,
}

/// 单次请求参数。
#[repr(C)]
pub struct PrimpRequestParamsC {
    pub params_json: *const c_char,
    pub headers_json: *const c_char,
    pub cookies_json: *const c_char,
    /// 二进制请求体(content)。
    pub body_ptr: *const u8,
    pub body_len: usize,
    /// JSON 字符串(json= 参数)。
    pub json_body: *const c_char,
    /// form-data(JSON 对象,会被 form-encode)。
    pub form_data_json: *const c_char,
    pub auth_username: *const c_char,
    pub auth_password: *const c_char,
    pub auth_bearer: *const c_char,
    pub timeout_secs: f64,
    pub read_timeout_secs: f64,
    /// 重定向策略:0=继承,1=强制开启,2=强制关闭。
    pub follow_redirects_override: i32,
}

// =============================================================================
// 客户端生命周期
// =============================================================================

/// 创建一个新的 primp 客户端。
///
/// 返回非空指针即成功;失败时返回 null,并通过 `out_err` 返回错误对象。
///
/// # Safety
/// `opts` 必须指向有效的 [`PrimpClientOptionsC`];`out_err` 必须指向可写指针位置。
#[no_mangle]
pub unsafe extern "C" fn primp_client_new(
    opts: *const PrimpClientOptionsC,
    out_err: *mut *mut PrimpErrorHandle,
) -> *mut PrimpClientHandle {
    if opts.is_null() {
        write_err(out_err, PrimpFfiError::new(ErrorKind::Builder, "opts 为 null"));
        return std::ptr::null_mut();
    }
    let opts = &*opts;

    match build_client(opts) {
        Ok(handle) => Box::into_raw(Box::new(handle)),
        Err(e) => {
            write_err(out_err, e);
            std::ptr::null_mut()
        }
    }
}

/// 释放客户端句柄。可对 null 安全调用。
#[no_mangle]
pub unsafe extern "C" fn primp_client_free(c: *mut PrimpClientHandle) {
    if !c.is_null() {
        drop(Box::from_raw(c));
    }
}

unsafe fn build_client(opts: &PrimpClientOptionsC) -> PrimpFfiResult<PrimpClientHandle> {
    let mut builder = PrimpClient::builder();

    // -------- impersonate --------
    let imp = ImpersonateC::from_i32(opts.impersonate)
        .map_err(|m| PrimpFfiError::new(ErrorKind::Builder, m))?;
    let imp_os = ImpersonateOSC::from_i32(opts.impersonate_os)
        .map_err(|m| PrimpFfiError::new(ErrorKind::Builder, m))?;

    if let Some(imp_val) = imp {
        // 必须先 set os,再 set impersonate(参考 primp-python 注释)
        if let Some(os) = imp_os {
            builder = builder.impersonate_os(os);
        }
        builder = builder.impersonate(imp_val);
    } else if let Some(os) = imp_os {
        builder = builder.impersonate_os(os);
    }

    // -------- headers --------
    if let Some(s) = cstr_to_option_str(opts.headers_json) {
        let map = parse_json_map(Some(s))?;
        let mut hm = HeaderMap::new();
        for (k, v) in map {
            let name = HeaderName::from_bytes(k.as_bytes())?;
            let value = HeaderValue::from_str(&v)?;
            hm.insert(name, value);
        }
        builder = builder.default_headers(hm);
    }

    // -------- cookie store / referer --------
    if opts.cookie_store {
        builder = builder.cookie_store(true);
    }
    if opts.referer {
        builder = builder.referer(true);
    }

    // -------- proxy --------
    let proxy = cstr_to_option_string(opts.proxy)
        .or_else(|| std::env::var("PRIMP_PROXY").ok());
    if let Some(p) = &proxy {
        builder = builder.proxy(Proxy::all(p)?);
    }

    // -------- timeouts --------
    if opts.timeout_secs > 0.0 {
        builder = builder.timeout(Duration::from_secs_f64(opts.timeout_secs));
    }
    if opts.connect_timeout_secs > 0.0 {
        builder = builder.connect_timeout(Duration::from_secs_f64(opts.connect_timeout_secs));
    }
    if opts.read_timeout_secs > 0.0 {
        builder = builder.read_timeout(Duration::from_secs_f64(opts.read_timeout_secs));
    }

    // -------- redirects --------
    if opts.follow_redirects {
        let max = if opts.max_redirects > 0 { opts.max_redirects as usize } else { 20 };
        builder = builder.redirect(Policy::limited(max));
    } else {
        builder = builder.redirect(Policy::none());
    }

    // -------- TLS --------
    if !opts.verify {
        builder = builder.danger_accept_invalid_certs(true);
    } else if let Some(ca_path) = cstr_to_option_string(opts.ca_cert_file) {
        match std::fs::read(&ca_path) {
            Ok(bytes) => match ::primp::Certificate::from_pem_bundle(&bytes) {
                Ok(certs) => {
                    for cert in certs {
                        builder = builder.add_root_certificate(cert);
                    }
                }
                Err(e) => {
                    return Err(PrimpFfiError::new(
                        ErrorKind::Builder,
                        format!("CA 证书解析失败: {}", e),
                    ))
                }
            },
            Err(e) => {
                return Err(PrimpFfiError::new(
                    ErrorKind::Builder,
                    format!("CA 证书读取失败 ({}): {}", ca_path, e),
                ))
            }
        }
    }

    if opts.https_only {
        builder = builder.https_only(true);
    }
    if opts.http2_only {
        builder = builder.http2_prior_knowledge();
    }

    let client = builder.build()?;
    let inner = Arc::new(RwLock::new(client));

    // 初始 cookies(若提供)
    if let Some(cookies_json) = cstr_to_option_str(opts.cookies_json) {
        let map = parse_json_map(Some(cookies_json))?;
        if !map.is_empty() {
            // 需要一个 URL 才能 set cookies;此处先暂存,留给 base_url 或第一次请求时使用。
            // primp 的 cookie store 要求按 URL 设置;我们简化处理:若有 base_url 就立刻设置。
            if let Some(base) = cstr_to_option_str(opts.base_url) {
                if let Ok(url) = Url::parse(base) {
                    let values: Vec<HeaderValue> = map
                        .iter()
                        .filter_map(|(k, v)| HeaderValue::from_str(&format!("{}={}", k, v)).ok())
                        .collect();
                    let guard = inner.read().expect("client lock poisoned");
                    guard.set_cookies(&url, values);
                }
            }
        }
    }

    Ok(PrimpClientHandle {
        inner,
        base_url: cstr_to_option_string(opts.base_url),
        default_params: cstr_to_option_string(opts.params_json),
    })
}

// =============================================================================
// 请求
// =============================================================================

/// 同步执行一次 HTTP 请求。
///
/// 成功时返回 0 并通过 `out_resp` 输出响应句柄;失败时返回非 0 并通过 `out_err` 输出错误。
///
/// # Safety
/// `c` 必须是 [`primp_client_new`] 返回的有效指针;
/// `url` 必须是 NUL 结尾的 UTF-8 字符串;
/// `req` 必须指向有效的 [`PrimpRequestParamsC`];
/// `out_resp` / `out_err` 必须指向可写位置。
#[no_mangle]
pub unsafe extern "C" fn primp_request(
    c: *mut PrimpClientHandle,
    method: i32,
    url: *const c_char,
    req: *const PrimpRequestParamsC,
    out_resp: *mut *mut PrimpResponseHandle,
    out_err: *mut *mut PrimpErrorHandle,
) -> i32 {
    if c.is_null() || req.is_null() || url.is_null() {
        write_err(out_err, PrimpFfiError::new(ErrorKind::Builder, "参数包含 null"));
        return 1;
    }
    let handle = &*c;
    let req = &*req;
    let url_str = match cstr_to_option_str(url) {
        Some(s) => s,
        None => {
            write_err(out_err, PrimpFfiError::new(ErrorKind::Builder, "url 不是合法 UTF-8"));
            return 1;
        }
    };

    match do_request(handle, method, url_str, req) {
        Ok(resp) => {
            *out_resp = Box::into_raw(Box::new(resp));
            0
        }
        Err(e) => {
            write_err(out_err, e);
            2
        }
    }
}

unsafe fn do_request(
    handle: &PrimpClientHandle,
    method_int: i32,
    url: &str,
    req: &PrimpRequestParamsC,
) -> PrimpFfiResult<PrimpResponseHandle> {
    let method = MethodC::from_i32(method_int).ok_or_else(|| {
        PrimpFfiError::new(ErrorKind::Builder, format!("未知 HTTP method 编码: {}", method_int))
    })?;

    // base_url 拼接
    let resolved_url = match &handle.base_url {
        Some(base) if !url.starts_with("http://") && !url.starts_with("https://") => {
            let base = base.trim_end_matches('/');
            let path = url.trim_start_matches('/');
            format!("{}/{}", base, path)
        }
        _ => url.to_string(),
    };

    // headers
    let header_map_opt = match cstr_to_option_str(req.headers_json) {
        Some(s) if !s.is_empty() => {
            let m = parse_json_map(Some(s))?;
            let mut hm = HeaderMap::new();
            for (k, v) in m {
                let name = HeaderName::from_bytes(k.as_bytes())?;
                let value = HeaderValue::from_str(&v)?;
                hm.insert(name, value);
            }
            Some(hm)
        }
        _ => None,
    };

    // cookies(若提供,即时写入 cookie store)
    if let Some(s) = cstr_to_option_str(req.cookies_json) {
        let m = parse_json_map(Some(s))?;
        if !m.is_empty() {
            let url_parsed = Url::parse(&resolved_url)?;
            let values: Vec<HeaderValue> = m
                .iter()
                .filter_map(|(k, v)| HeaderValue::from_str(&format!("{}={}", k, v)).ok())
                .collect();
            let guard = handle.inner.read().expect("client lock poisoned");
            guard.set_cookies(&url_parsed, values);
        }
    }

    // follow_redirects 临时覆盖
    let original_override = req.follow_redirects_override;
    if original_override != 0 {
        let mut guard = handle.inner.write().expect("client lock poisoned");
        if original_override == 1 {
            guard.set_redirect_policy(Policy::limited(20));
        } else {
            guard.set_redirect_policy(Policy::none());
        }
    }

    // 克隆 client 以释放锁
    let client = handle.inner.read().expect("client lock poisoned").clone();

    // params
    let params_json = cstr_to_option_str(req.params_json)
        .filter(|s| !s.is_empty())
        .or_else(|| handle.default_params.as_deref());
    let params_map = parse_json_map(params_json)?;

    // body
    let body_bytes = buf_to_option_vec(req.body_ptr, req.body_len);
    let json_body = cstr_to_option_string(req.json_body);
    let form_data_json = cstr_to_option_string(req.form_data_json);

    let auth_user = cstr_to_option_string(req.auth_username);
    let auth_pass = cstr_to_option_string(req.auth_password);
    let auth_bearer = cstr_to_option_string(req.auth_bearer);

    let timeout_secs = req.timeout_secs;
    let read_timeout_secs = req.read_timeout_secs;

    let result: PrimpFfiResult<(PrimpResponse, String, u16)> = rt().block_on(async move {
        let mut rb = client.request(method, &resolved_url);

        if !params_map.is_empty() {
            rb = rb.query(&params_map);
        }
        if let Some(hm) = header_map_opt {
            rb = rb.headers(hm);
        }
        if let Some(b) = body_bytes {
            rb = rb.body(Body::from(b));
        }
        if let Some(s) = json_body {
            let v: serde_json::Value = serde_json::from_str(&s)?;
            rb = rb.json(&v);
        }
        if let Some(s) = form_data_json {
            let v: serde_json::Value = serde_json::from_str(&s)?;
            rb = rb.form(&v);
        }
        if let Some(u) = auth_user {
            rb = rb.basic_auth(u, auth_pass.as_deref());
        } else if let Some(t) = auth_bearer {
            rb = rb.bearer_auth(t);
        }
        if timeout_secs > 0.0 {
            rb = rb.timeout(Duration::from_secs_f64(timeout_secs));
        }
        if read_timeout_secs > 0.0 {
            rb = rb.read_timeout(Duration::from_secs_f64(read_timeout_secs));
        }

        let resp = rb.send().await?;
        let final_url = resp.url().to_string();
        let status = resp.status().as_u16();
        Ok((resp, final_url, status))
    });

    // 恢复重定向策略(无论成功失败)
    if original_override != 0 {
        let mut guard = handle.inner.write().expect("client lock poisoned");
        guard.set_redirect_policy(Policy::limited(20));
    }

    let (resp, final_url, status) = result?;

    let headers_json = headers_to_json(resp.headers());
    let cookies_json = cookies_to_json(resp.headers());
    let encoding = extract_encoding_name(resp.headers());

    // 读取响应体(首版同步全部读入)
    let body = rt().block_on(async move {
        resp.bytes().await.map_err(PrimpFfiError::from)
    })?;

    Ok(PrimpResponseHandle {
        body: body.to_vec(),
        url: final_url,
        status,
        headers_json,
        cookies_json,
        encoding,
    })
}

fn extract_encoding_name(headers: &HeaderMap) -> String {
    use mime::Mime;
    headers
        .get(::primp::header::CONTENT_TYPE)
        .and_then(|v| v.to_str().ok())
        .and_then(|s| {
            s.parse::<Mime>().ok().and_then(|mime| {
                mime.get_param("charset")
                    .and_then(|c| encoding_rs::Encoding::for_label(c.as_str().as_bytes()))
            })
        })
        .unwrap_or(encoding_rs::UTF_8)
        .name()
        .to_string()
}

// =============================================================================
// Response 访问器
// =============================================================================

/// 获取响应的状态码。
#[no_mangle]
pub unsafe extern "C" fn primp_response_status(r: *const PrimpResponseHandle) -> u16 {
    if r.is_null() {
        return 0;
    }
    (*r).status
}

/// 获取最终响应 URL(C 字符串,**调用方必须用 [`primp_string_free`] 释放**)。
#[no_mangle]
pub unsafe extern "C" fn primp_response_url(r: *const PrimpResponseHandle) -> *mut c_char {
    if r.is_null() {
        return std::ptr::null_mut();
    }
    string_to_c((*r).url.clone())
}

/// 获取响应头 JSON 字符串(name -> value),**调用方必须用 [`primp_string_free`] 释放**。
#[no_mangle]
pub unsafe extern "C" fn primp_response_headers_json(r: *const PrimpResponseHandle) -> *mut c_char {
    if r.is_null() {
        return std::ptr::null_mut();
    }
    string_to_c((*r).headers_json.clone())
}

/// 获取响应 cookies JSON 字符串。**调用方必须用 [`primp_string_free`] 释放**。
#[no_mangle]
pub unsafe extern "C" fn primp_response_cookies_json(r: *const PrimpResponseHandle) -> *mut c_char {
    if r.is_null() {
        return std::ptr::null_mut();
    }
    string_to_c((*r).cookies_json.clone())
}

/// 获取响应 body 的字节指针与长度(指针有效期与 PrimpResponseHandle 相同)。
///
/// **不要释放返回的指针**;通过 [`primp_response_free`] 释放整个 PrimpResponseHandle 即可。
#[no_mangle]
pub unsafe extern "C" fn primp_response_body(
    r: *const PrimpResponseHandle,
    out_len: *mut usize,
) -> *const u8 {
    if r.is_null() || out_len.is_null() {
        if !out_len.is_null() {
            *out_len = 0;
        }
        return std::ptr::null();
    }
    *out_len = (*r).body.len();
    (*r).body.as_ptr()
}

/// 获取响应编码名(如 "UTF-8")。**调用方必须用 [`primp_string_free`] 释放**。
#[no_mangle]
pub unsafe extern "C" fn primp_response_encoding(r: *const PrimpResponseHandle) -> *mut c_char {
    if r.is_null() {
        return std::ptr::null_mut();
    }
    string_to_c((*r).encoding.clone())
}

/// 释放响应句柄。
#[no_mangle]
pub unsafe extern "C" fn primp_response_free(r: *mut PrimpResponseHandle) {
    if !r.is_null() {
        drop(Box::from_raw(r));
    }
}

// =============================================================================
// 错误访问器
// =============================================================================

/// 获取错误 kind(对应 [`ErrorKind`])。
#[no_mangle]
pub unsafe extern "C" fn primp_error_kind(e: *const PrimpErrorHandle) -> i32 {
    if e.is_null() {
        return 0;
    }
    (*e).kind as i32
}

/// 获取错误关联的 HTTP 状态码(仅 Status 类型有意义,其它返回 0)。
#[no_mangle]
pub unsafe extern "C" fn primp_error_status(e: *const PrimpErrorHandle) -> u16 {
    if e.is_null() {
        return 0;
    }
    (*e).status
}

/// 获取错误信息字符串。**调用方必须用 [`primp_string_free`] 释放**。
#[no_mangle]
pub unsafe extern "C" fn primp_error_message(e: *const PrimpErrorHandle) -> *mut c_char {
    if e.is_null() {
        return std::ptr::null_mut();
    }
    string_to_c((*e).message.clone())
}

/// 释放错误句柄。
#[no_mangle]
pub unsafe extern "C" fn primp_error_free(e: *mut PrimpErrorHandle) {
    if !e.is_null() {
        drop(Box::from_raw(e));
    }
}

/// 释放由 primp_go 分配的 C 字符串。
#[no_mangle]
pub unsafe extern "C" fn primp_string_free(s: *mut c_char) {
    if !s.is_null() {
        drop(std::ffi::CString::from_raw(s));
    }
}

// =============================================================================
// 客户端运行时操作
// =============================================================================

/// 获取客户端默认 headers JSON 字符串。**调用方必须用 [`primp_string_free`] 释放**。
#[no_mangle]
pub unsafe extern "C" fn primp_client_headers_json(c: *const PrimpClientHandle) -> *mut c_char {
    if c.is_null() {
        return std::ptr::null_mut();
    }
    let guard = (*c).inner.read().expect("client lock poisoned");
    string_to_c(headers_to_json(guard.headers()))
}

/// 设置或更新客户端默认 headers(JSON 对象)。
/// `replace=true` 时清空已有头部,否则追加/覆盖。
#[no_mangle]
pub unsafe extern "C" fn primp_client_set_headers(
    c: *mut PrimpClientHandle,
    headers_json: *const c_char,
    replace: bool,
    out_err: *mut *mut PrimpErrorHandle,
) -> i32 {
    if c.is_null() {
        write_err(out_err, PrimpFfiError::new(ErrorKind::Builder, "client 为 null"));
        return 1;
    }
    let s = cstr_to_option_str(headers_json).unwrap_or("");
    let m = match parse_json_map(if s.is_empty() { None } else { Some(s) }) {
        Ok(m) => m,
        Err(e) => {
            write_err(out_err, e);
            return 1;
        }
    };
    let mut guard = (*c).inner.write().expect("client lock poisoned");
    let headers = guard.headers_mut();
    if replace {
        headers.clear();
    }
    for (k, v) in m {
        let name = match HeaderName::from_bytes(k.as_bytes()) {
            Ok(n) => n,
            Err(e) => {
                write_err(out_err, PrimpFfiError::from(e));
                return 1;
            }
        };
        let val = match HeaderValue::from_str(&v) {
            Ok(v) => v,
            Err(e) => {
                write_err(out_err, PrimpFfiError::from(e));
                return 1;
            }
        };
        headers.insert(name, val);
    }
    0
}

// =============================================================================
// 内部辅助
// =============================================================================

unsafe fn write_err(out_err: *mut *mut PrimpErrorHandle, err: PrimpFfiError) {
    if out_err.is_null() {
        return;
    }
    let handle = PrimpErrorHandle {
        kind: err.kind,
        message: err.message,
        status: err.status,
    };
    *out_err = Box::into_raw(Box::new(handle));
}
