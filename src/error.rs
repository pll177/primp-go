//! 错误分类:把内部错误归类为整数 kind,Go 侧根据 kind 还原成对应类型。
//!
//! 错误层级(参考 primp-python):
//! ```text
//! Builder=1, Request=2, Connect=3, Timeout=4,
//! Status=5, Redirect=6, Body=7, Decode=8, Upgrade=9, Generic=99
//! ```

use std::fmt;

/// 错误 kind 整数编码,与 Go 端 `ErrorKind` 常量保持一致。
#[repr(i32)]
#[derive(Debug, Clone, Copy)]
pub enum ErrorKind {
    Generic = 99,
    Builder = 1,
    Request = 2,
    Connect = 3,
    Timeout = 4,
    Status = 5,
    Redirect = 6,
    Body = 7,
    Decode = 8,
    Upgrade = 9,
}

/// 内部错误结构,在 FFI 边界会被装箱并以不透明指针暴露给 Go 端。
#[derive(Debug)]
pub struct PrimpFfiError {
    pub kind: ErrorKind,
    pub message: String,
    pub status: u16,
}

impl PrimpFfiError {
    pub fn new(kind: ErrorKind, message: impl Into<String>) -> Self {
        Self {
            kind,
            message: message.into(),
            status: 0,
        }
    }

    pub fn with_status(mut self, status: u16) -> Self {
        self.status = status;
        self
    }
}

impl fmt::Display for PrimpFfiError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "[{:?}] {}", self.kind, self.message)
    }
}

impl std::error::Error for PrimpFfiError {}

/// 把 primp(reqwest) 的错误归类成 [`PrimpFfiError`]。
/// 与 primp-python 的 `convert_reqwest_error` 保持同样的判定顺序。
pub fn classify_primp_error(err: ::primp::Error) -> PrimpFfiError {
    let message = err.to_string();

    if err.is_builder() {
        return PrimpFfiError::new(ErrorKind::Builder, message);
    }
    if err.is_status() {
        let status = err.status().map(|s| s.as_u16()).unwrap_or(0);
        return PrimpFfiError::new(ErrorKind::Status, message).with_status(status);
    }
    if err.is_redirect() {
        return PrimpFfiError::new(ErrorKind::Redirect, message);
    }
    if err.is_timeout() {
        return PrimpFfiError::new(ErrorKind::Timeout, message);
    }
    if err.is_connect() {
        return PrimpFfiError::new(ErrorKind::Connect, message);
    }
    if err.is_request() {
        return PrimpFfiError::new(ErrorKind::Request, message);
    }
    if err.is_decode() {
        return PrimpFfiError::new(ErrorKind::Decode, message);
    }
    if err.is_body() {
        return PrimpFfiError::new(ErrorKind::Body, message);
    }
    if err.is_upgrade() {
        return PrimpFfiError::new(ErrorKind::Upgrade, message);
    }
    PrimpFfiError::new(ErrorKind::Generic, message)
}

impl From<::primp::Error> for PrimpFfiError {
    fn from(e: ::primp::Error) -> Self {
        classify_primp_error(e)
    }
}

impl From<String> for PrimpFfiError {
    fn from(e: String) -> Self {
        Self::new(ErrorKind::Generic, e)
    }
}

impl From<&str> for PrimpFfiError {
    fn from(e: &str) -> Self {
        Self::new(ErrorKind::Generic, e.to_string())
    }
}

impl From<serde_json::Error> for PrimpFfiError {
    fn from(e: serde_json::Error) -> Self {
        Self::new(ErrorKind::Generic, format!("JSON 错误: {}", e))
    }
}

impl From<url::ParseError> for PrimpFfiError {
    fn from(e: url::ParseError) -> Self {
        Self::new(ErrorKind::Builder, format!("URL 解析错误: {}", e))
    }
}

impl From<http::method::InvalidMethod> for PrimpFfiError {
    fn from(e: http::method::InvalidMethod) -> Self {
        Self::new(ErrorKind::Builder, format!("HTTP 方法非法: {}", e))
    }
}

impl From<http::header::InvalidHeaderValue> for PrimpFfiError {
    fn from(e: http::header::InvalidHeaderValue) -> Self {
        Self::new(ErrorKind::Builder, format!("HTTP 头部值非法: {}", e))
    }
}

impl From<http::header::InvalidHeaderName> for PrimpFfiError {
    fn from(e: http::header::InvalidHeaderName) -> Self {
        Self::new(ErrorKind::Builder, format!("HTTP 头部名非法: {}", e))
    }
}

pub type PrimpFfiResult<T> = std::result::Result<T, PrimpFfiError>;
