//! 全局 tokio 运行时:FFI 是同步阻塞调用,内部用 `block_on` 驱动 primp 的 async API。

use once_cell::sync::Lazy;
use tokio::runtime::{Builder, Runtime};

static RUNTIME: Lazy<Runtime> = Lazy::new(|| {
    Builder::new_multi_thread()
        .enable_all()
        .thread_name("primp-go-rt")
        .build()
        .expect("无法创建 tokio 运行时")
});

/// 取得全局 tokio 运行时引用。
pub fn rt() -> &'static Runtime {
    &RUNTIME
}
