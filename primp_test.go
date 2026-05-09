//go:build windows && amd64

package primp

// 端到端测试。需要联网,默认通过 httpbin.org 进行验证。
// 跑测试前需要先编译 Rust 库:`cargo build --release -p primp-go`。

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const httpbin = "https://httpbin.org"

// TestGetSimple 验证基础 GET 请求与 JSON 反序列化。
func TestGetSimple(t *testing.T) {
	c, err := NewClient(ClientOptions{Impersonate: ImpersonateChromeV146})
	if err != nil {
		t.Fatalf("NewClient 失败: %v", err)
	}
	defer c.Close()

	resp, err := c.Get(httpbin + "/get?foo=bar")
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	defer resp.Close()

	if resp.StatusCode() != 200 {
		t.Fatalf("期望状态 200,得到 %d", resp.StatusCode())
	}

	var body struct {
		Args map[string]string `json:"args"`
	}
	if err := resp.JSON(&body); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}
	if body.Args["foo"] != "bar" {
		t.Errorf("期望 foo=bar,得到 %v", body.Args)
	}
}

// TestPostJSON 验证 POST + WithJSON。
func TestPostJSON(t *testing.T) {
	c, err := NewClient(ClientOptions{})
	if err != nil {
		t.Fatalf("NewClient 失败: %v", err)
	}
	defer c.Close()

	payload := map[string]any{"hello": "世界", "n": 42}
	resp, err := c.Post(httpbin+"/post", WithJSON(payload))
	if err != nil {
		t.Fatalf("Post 失败: %v", err)
	}
	defer resp.Close()

	var body struct {
		JSON map[string]any `json:"json"`
	}
	if err := resp.JSON(&body); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}
	if body.JSON["hello"] != "世界" {
		t.Errorf("回显字段不一致: %v", body.JSON)
	}
}

// TestTimeout 验证短超时触发 Timeout 错误,并能用 errors.Is 匹配。
func TestTimeout(t *testing.T) {
	c, err := NewClient(ClientOptions{Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("NewClient 失败: %v", err)
	}
	defer c.Close()

	_, err = c.Get(httpbin + "/delay/3")
	if err == nil {
		t.Fatal("期望超时错误,实际成功")
	}
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("期望 ErrTimeout,得到 %v (%T)", err, err)
	}
}

// TestRaiseForStatus 验证 4xx 状态码可被显式抛出。
func TestRaiseForStatus(t *testing.T) {
	resp, err := Get(httpbin + "/status/404")
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	defer resp.Close()

	if err := resp.RaiseForStatus(); err == nil {
		t.Fatal("404 应触发 RaiseForStatus 错误")
	} else if !errors.Is(err, ErrStatus) {
		t.Errorf("期望 ErrStatus,得到 %v", err)
	}
}

// TestRedirectDisabled 验证关闭重定向后能拿到 3xx 状态码。
func TestRedirectDisabled(t *testing.T) {
	c, err := NewClient(ClientOptions{})
	if err != nil {
		t.Fatalf("NewClient 失败: %v", err)
	}
	defer c.Close()

	resp, err := c.Get(httpbin+"/redirect/1", WithFollowRedirects(false))
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	defer resp.Close()
	if resp.StatusCode() < 300 || resp.StatusCode() >= 400 {
		t.Errorf("期望 3xx,得到 %d", resp.StatusCode())
	}
}

// TestHeadersUserAgent 验证 impersonate 注入了浏览器 UA。
func TestHeadersUserAgent(t *testing.T) {
	c, err := NewClient(ClientOptions{Impersonate: ImpersonateChromeV146})
	if err != nil {
		t.Fatalf("NewClient 失败: %v", err)
	}
	defer c.Close()

	resp, err := c.Get(httpbin + "/headers")
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	defer resp.Close()

	var body struct {
		Headers map[string]string `json:"headers"`
	}
	if err := resp.JSON(&body); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}
	ua := body.Headers["User-Agent"]
	if !strings.Contains(strings.ToLower(ua), "chrome") {
		t.Errorf("期望 UA 含 chrome,实际 %q", ua)
	}
}

// TestPostWithContent 回归测试:用 WithContent 传 []byte 不应触发
// "argument of cgo function has Go pointer to unpinned Go pointer" panic。
//
// 跑这个用例时建议带上 GODEBUG=cgocheck=2 一起跑,可以最严格地复现/验证。
func TestPostWithContent(t *testing.T) {
	c, err := NewClient(ClientOptions{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Close()

	payload := []byte("hello world from WithContent —— cgo pointer regression test")
	resp, err := c.Post(httpbin+"/post",
		WithContent(payload),
		WithHeaders(map[string]string{"content-type": "text/plain"}),
	)
	if err != nil {
		t.Fatalf("Post WithContent 失败: %v", err)
	}
	defer resp.Close()

	if resp.StatusCode() != 200 {
		t.Fatalf("status=%d, 期望 200", resp.StatusCode())
	}
	var body struct {
		Data string `json:"data"`
	}
	if err := resp.JSON(&body); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if body.Data != string(payload) {
		t.Errorf("回显 data=%q, 期望 %q", body.Data, string(payload))
	}
}

// TestPostWithContentLarge 用一个 64KB 的随机 payload 走一遍 WithContent,
// 确保大体积也不会触发 cgocheck panic 或越界。
func TestPostWithContentLarge(t *testing.T) {
	c, _ := NewClient(ClientOptions{Timeout: 15 * time.Second})
	defer c.Close()

	payload := make([]byte, 64*1024)
	for i := range payload {
		payload[i] = byte(i & 0xff)
	}

	resp, err := c.Post(httpbin+"/post", WithContent(payload))
	if err != nil {
		t.Fatalf("Post 大 body 失败: %v", err)
	}
	defer resp.Close()
	if resp.StatusCode() != 200 {
		t.Fatalf("status=%d, 期望 200", resp.StatusCode())
	}
}

// TestCookies 验证 cookie 写入能被服务器回显。
func TestCookies(t *testing.T) {
	c, err := NewClient(ClientOptions{})
	if err != nil {
		t.Fatalf("NewClient 失败: %v", err)
	}
	defer c.Close()

	resp, err := c.Get(httpbin+"/cookies", WithCookies(map[string]string{"k1": "v1"}))
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	defer resp.Close()

	var body struct {
		Cookies map[string]string `json:"cookies"`
	}
	if err := resp.JSON(&body); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}
	if body.Cookies["k1"] != "v1" {
		t.Errorf("cookie 未生效: %v", body.Cookies)
	}
}
