//! 整数枚举定义 + 与 primp 原生枚举的互转。
//!
//! Go 端对应的命名常量在 `go/enums.go`,数值必须严格保持一致。

use primp::imp::{Impersonate, ImpersonateOS};
use primp::Method;
use rand::seq::IndexedRandom;

/// HTTP 请求方法的整数编码(对应 Go 端 `Method` 枚举)。
#[repr(i32)]
#[derive(Debug, Clone, Copy)]
pub enum MethodC {
    Get = 0,
    Head = 1,
    Options = 2,
    Delete = 3,
    Post = 4,
    Put = 5,
    Patch = 6,
}

impl MethodC {
    /// 把 Go 端传过来的整数转成 primp 的 [`Method`]。
    pub fn from_i32(v: i32) -> Option<Method> {
        Some(match v {
            0 => Method::GET,
            1 => Method::HEAD,
            2 => Method::OPTIONS,
            3 => Method::DELETE,
            4 => Method::POST,
            5 => Method::PUT,
            6 => Method::PATCH,
            _ => return None,
        })
    }
}

/// 浏览器指纹的整数编码(对应 Go 端 `Impersonate` 枚举)。
///
/// **关键约束**:数值必须与 `go/enums.go` 中的常量一一对应。
/// 此处使用稀疏编号是为了给浏览器家族留出版本扩展空间。
/// **关键约定**:0 表示 None(Go 零值即"不启用 impersonate")。
#[repr(i32)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ImpersonateC {
    None = 0,
    Chrome = 1,
    ChromeV144 = 2,
    ChromeV145 = 3,
    ChromeV146 = 4,
    Edge = 11,
    EdgeV144 = 12,
    EdgeV145 = 13,
    EdgeV146 = 14,
    Safari = 21,
    SafariV18_5 = 22,
    SafariV26 = 23,
    SafariV26_3 = 24,
    Firefox = 31,
    FirefoxV140 = 32,
    FirefoxV146 = 33,
    FirefoxV147 = 34,
    FirefoxV148 = 35,
    Opera = 41,
    OperaV126 = 42,
    OperaV127 = 43,
    OperaV128 = 44,
    OperaV129 = 45,
    Random = 99,
}

impl ImpersonateC {
    /// 把 Go 端传过来的整数转换为 primp 的 [`Impersonate`] 值。
    /// 返回 `Ok(None)` 表示"不设置 impersonate";`Err` 表示数值无效。
    pub fn from_i32(v: i32) -> Result<Option<Impersonate>, String> {
        let imp = match v {
            0 => return Ok(None),
            1 => Impersonate::Chrome,
            2 => Impersonate::ChromeV144,
            3 => Impersonate::ChromeV145,
            4 => Impersonate::ChromeV146,
            11 => Impersonate::Edge,
            12 => Impersonate::EdgeV144,
            13 => Impersonate::EdgeV145,
            14 => Impersonate::EdgeV146,
            21 => Impersonate::Safari,
            22 => Impersonate::SafariV18_5,
            23 => Impersonate::SafariV26,
            24 => Impersonate::SafariV26_3,
            31 => Impersonate::Firefox,
            32 => Impersonate::FirefoxV140,
            33 => Impersonate::FirefoxV146,
            34 => Impersonate::FirefoxV147,
            35 => Impersonate::FirefoxV148,
            41 => Impersonate::Opera,
            42 => Impersonate::OperaV126,
            43 => Impersonate::OperaV127,
            44 => Impersonate::OperaV128,
            45 => Impersonate::OperaV129,
            99 => Impersonate::Random,
            _ => return Err(format!("未知的 Impersonate 编码: {}", v)),
        };
        Ok(Some(imp))
    }
}

/// 操作系统指纹的整数编码(对应 Go 端 `ImpersonateOS` 枚举)。
/// **关键约定**:0 表示 None(Go 零值即"不指定 OS 指纹")。
#[repr(i32)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ImpersonateOSC {
    None = 0,
    Android = 1,
    IOS = 2,
    Linux = 3,
    MacOS = 4,
    Windows = 5,
    Random = 99,
}

const OS_LIST: &[ImpersonateOS] = &[
    ImpersonateOS::Android,
    ImpersonateOS::IOS,
    ImpersonateOS::Linux,
    ImpersonateOS::MacOS,
    ImpersonateOS::Windows,
];

impl ImpersonateOSC {
    /// 把 Go 端传过来的整数转换为 primp 的 [`ImpersonateOS`] 值。
    /// `None` 表示不设置;`Random` 会从所有可用 OS 中挑选一个。
    pub fn from_i32(v: i32) -> Result<Option<ImpersonateOS>, String> {
        let os = match v {
            0 => return Ok(None),
            1 => ImpersonateOS::Android,
            2 => ImpersonateOS::IOS,
            3 => ImpersonateOS::Linux,
            4 => ImpersonateOS::MacOS,
            5 => ImpersonateOS::Windows,
            99 => *OS_LIST.choose(&mut rand::rng()).unwrap(),
            _ => return Err(format!("未知的 ImpersonateOS 编码: {}", v)),
        };
        Ok(Some(os))
    }
}
