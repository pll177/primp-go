/* primp-go FFI 头文件 - 由 cbindgen 自动生成,请勿手工编辑 */

#ifndef PRIMP_GO_H
#define PRIMP_GO_H

#pragma once

/* 警告:本文件由构建脚本自动生成 */

#include <stdarg.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>

/**
 * 客户端句柄(对 Go 端不透明)。
 */
typedef struct PrimpClientHandle PrimpClientHandle;

/**
 * 错误句柄(对 Go 端不透明)。
 */
typedef struct PrimpErrorHandle PrimpErrorHandle;

/**
 * 响应句柄(对 Go 端不透明)。
 */
typedef struct PrimpResponseHandle PrimpResponseHandle;

/**
 * 客户端创建参数(由 Go 侧填充,字段对应 Python `Client.__init__`)。
 *
 * 字符串字段为空指针表示"未设置";map 类字段以 JSON 字符串传入。
 * `i32` 枚举字段:`-1` 表示未设置(对应 [`ImpersonateC::None`])。
 */
typedef struct {
  const char *auth_username;
  const char *auth_password;
  const char *auth_bearer;
  const char *params_json;
  const char *headers_json;
  const char *cookies_json;
  bool cookie_store;
  bool referer;
  const char *proxy;
  /**
   * 总超时,秒;0 表示未设置。
   */
  double timeout_secs;
  double connect_timeout_secs;
  double read_timeout_secs;
  int32_t impersonate;
  int32_t impersonate_os;
  bool follow_redirects;
  uint32_t max_redirects;
  bool verify;
  const char *ca_cert_file;
  bool https_only;
  bool http2_only;
  const char *base_url;
} PrimpClientOptionsC;

/**
 * 单次请求参数。
 */
typedef struct {
  const char *params_json;
  const char *headers_json;
  const char *cookies_json;
  /**
   * 二进制请求体(content)。
   */
  const uint8_t *body_ptr;
  size_t body_len;
  /**
   * JSON 字符串(json= 参数)。
   */
  const char *json_body;
  /**
   * form-data(JSON 对象,会被 form-encode)。
   */
  const char *form_data_json;
  const char *auth_username;
  const char *auth_password;
  const char *auth_bearer;
  double timeout_secs;
  double read_timeout_secs;
  /**
   * 重定向策略:0=继承,1=强制开启,2=强制关闭。
   */
  int32_t follow_redirects_override;
} PrimpRequestParamsC;

#ifdef __cplusplus
extern "C" {
#endif // __cplusplus

/**
 * 创建一个新的 primp 客户端。
 *
 * 返回非空指针即成功;失败时返回 null,并通过 `out_err` 返回错误对象。
 *
 * # Safety
 * `opts` 必须指向有效的 [`PrimpClientOptionsC`];`out_err` 必须指向可写指针位置。
 */
PrimpClientHandle *primp_client_new(const PrimpClientOptionsC *opts, PrimpErrorHandle **out_err);

/**
 * 释放客户端句柄。可对 null 安全调用。
 */
void primp_client_free(PrimpClientHandle *c);

/**
 * 同步执行一次 HTTP 请求。
 *
 * 成功时返回 0 并通过 `out_resp` 输出响应句柄;失败时返回非 0 并通过 `out_err` 输出错误。
 *
 * # Safety
 * `c` 必须是 [`primp_client_new`] 返回的有效指针;
 * `url` 必须是 NUL 结尾的 UTF-8 字符串;
 * `req` 必须指向有效的 [`PrimpRequestParamsC`];
 * `out_resp` / `out_err` 必须指向可写位置。
 */
int32_t primp_request(PrimpClientHandle *c,
                      int32_t method,
                      const char *url,
                      const PrimpRequestParamsC *req,
                      PrimpResponseHandle **out_resp,
                      PrimpErrorHandle **out_err);

/**
 * 获取响应的状态码。
 */
uint16_t primp_response_status(const PrimpResponseHandle *r);

/**
 * 获取最终响应 URL(C 字符串,**调用方必须用 [`primp_string_free`] 释放**)。
 */
char *primp_response_url(const PrimpResponseHandle *r);

/**
 * 获取响应头 JSON 字符串(name -> value),**调用方必须用 [`primp_string_free`] 释放**。
 */
char *primp_response_headers_json(const PrimpResponseHandle *r);

/**
 * 获取响应 cookies JSON 字符串。**调用方必须用 [`primp_string_free`] 释放**。
 */
char *primp_response_cookies_json(const PrimpResponseHandle *r);

/**
 * 获取响应 body 的字节指针与长度(指针有效期与 PrimpResponseHandle 相同)。
 *
 * **不要释放返回的指针**;通过 [`primp_response_free`] 释放整个 PrimpResponseHandle 即可。
 */
const uint8_t *primp_response_body(const PrimpResponseHandle *r,
                                   size_t *out_len);

/**
 * 获取响应编码名(如 "UTF-8")。**调用方必须用 [`primp_string_free`] 释放**。
 */
char *primp_response_encoding(const PrimpResponseHandle *r);

/**
 * 释放响应句柄。
 */
void primp_response_free(PrimpResponseHandle *r);

/**
 * 获取错误 kind(对应 [`ErrorKind`])。
 */
int32_t primp_error_kind(const PrimpErrorHandle *e);

/**
 * 获取错误关联的 HTTP 状态码(仅 Status 类型有意义,其它返回 0)。
 */
uint16_t primp_error_status(const PrimpErrorHandle *e);

/**
 * 获取错误信息字符串。**调用方必须用 [`primp_string_free`] 释放**。
 */
char *primp_error_message(const PrimpErrorHandle *e);

/**
 * 释放错误句柄。
 */
void primp_error_free(PrimpErrorHandle *e);

/**
 * 释放由 primp_go 分配的 C 字符串。
 */
void primp_string_free(char *s);

/**
 * 获取客户端默认 headers JSON 字符串。**调用方必须用 [`primp_string_free`] 释放**。
 */
char *primp_client_headers_json(const PrimpClientHandle *c);

/**
 * 设置或更新客户端默认 headers(JSON 对象)。
 * `replace=true` 时清空已有头部,否则追加/覆盖。
 */
int32_t primp_client_set_headers(PrimpClientHandle *c,
                                 const char *headers_json,
                                 bool replace,
                                 PrimpErrorHandle **out_err);

#ifdef __cplusplus
}  // extern "C"
#endif  // __cplusplus

#endif  /* PRIMP_GO_H */
