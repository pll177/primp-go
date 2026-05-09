//! FFI 辅助:C 字符串 / 字节缓冲 / JSON map 的转换。

use std::collections::BTreeMap;
use std::ffi::{CStr, CString};
use std::os::raw::c_char;

use crate::error::{ErrorKind, PrimpFfiError, PrimpFfiResult};

/// 把可空的 C 字符串安全地转成 `Option<String>`。
///
/// # Safety
/// 调用方必须保证 `ptr` 要么为空指针,要么指向以 `\0` 结尾的合法 UTF-8 缓冲区。
pub unsafe fn cstr_to_option_string(ptr: *const c_char) -> Option<String> {
    if ptr.is_null() {
        return None;
    }
    CStr::from_ptr(ptr).to_str().ok().map(|s| s.to_string())
}

/// 把可空 C 字符串转成 `&str`(零拷贝);空指针返回 `None`。
///
/// # Safety
/// 同 [`cstr_to_option_string`]。
pub unsafe fn cstr_to_option_str<'a>(ptr: *const c_char) -> Option<&'a str> {
    if ptr.is_null() {
        return None;
    }
    CStr::from_ptr(ptr).to_str().ok()
}

/// 把字节缓冲拷贝成 `Vec<u8>`,空指针/零长度返回 `None`。
///
/// # Safety
/// `ptr` 必须指向至少 `len` 字节的有效内存,或者为 null 且 `len == 0`。
pub unsafe fn buf_to_option_vec(ptr: *const u8, len: usize) -> Option<Vec<u8>> {
    if ptr.is_null() || len == 0 {
        return None;
    }
    Some(std::slice::from_raw_parts(ptr, len).to_vec())
}

/// 把 Rust `String` 转成 `*mut c_char`(由调用方负责通过 [`primp_string_free`] 释放)。
pub fn string_to_c(s: String) -> *mut c_char {
    match CString::new(s) {
        Ok(cs) => cs.into_raw(),
        Err(_) => std::ptr::null_mut(),
    }
}

/// 把 JSON 字符串解析成 `BTreeMap<String, String>`。空字符串/`null` 视为空 map。
pub fn parse_json_map(json: Option<&str>) -> PrimpFfiResult<BTreeMap<String, String>> {
    let s = match json {
        Some(s) if !s.is_empty() => s,
        _ => return Ok(BTreeMap::new()),
    };
    serde_json::from_str(s).map_err(|e| {
        PrimpFfiError::new(ErrorKind::Generic, format!("解析 JSON map 失败: {}", e))
    })
}

/// 把 [`primp::header::HeaderMap`] 序列化为 JSON 字符串(去重保留首个值即可)。
pub fn headers_to_json(headers: &::primp::header::HeaderMap) -> String {
    let mut map = serde_json::Map::with_capacity(headers.len());
    for (k, v) in headers.iter() {
        if let Ok(vs) = v.to_str() {
            map.entry(k.as_str().to_string())
                .or_insert_with(|| serde_json::Value::String(vs.to_string()));
        }
    }
    serde_json::Value::Object(map).to_string()
}

/// 从响应头里提取所有 Set-Cookie,序列化为 JSON 字符串(name -> value)。
pub fn cookies_to_json(headers: &::primp::header::HeaderMap) -> String {
    let mut map = serde_json::Map::new();
    for cookie_header in headers.get_all(::primp::header::SET_COOKIE).iter() {
        if let Ok(s) = cookie_header.to_str() {
            if let Some((name, rest)) = s.split_once('=') {
                let value = rest.split(';').next().unwrap_or("").trim().to_string();
                map.insert(name.trim().to_string(), serde_json::Value::String(value));
            }
        }
    }
    serde_json::Value::Object(map).to_string()
}
