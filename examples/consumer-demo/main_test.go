//go:build windows && amd64

package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	primp "github.com/pll177/primp-go"
)

// TestGet 端到端验证 GET 200 + JSON 反序列化。
func TestGet(t *testing.T) {
	c, err := primp.NewClient(primp.ClientOptions{
		Impersonate: primp.ImpersonateChromeV146,
		Timeout:     15 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Close()

	r, err := c.Get("https://httpbin.org/get?k=v")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer r.Close()

	if r.StatusCode() != 200 {
		t.Fatalf("status=%d, 期望 200", r.StatusCode())
	}

	var body struct {
		Args map[string]string `json:"args"`
	}
	if err := r.JSON(&body); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if body.Args["k"] != "v" {
		t.Errorf("args=%v, 期望 k=v", body.Args)
	}
}

// TestImpersonate 验证 Chrome 指纹注入了对应 UA。
// OS 显式指定为 Windows,避免随机到 iOS 时 UA 变成 CriOS(虽然也是 Chrome 系列)。
func TestImpersonate(t *testing.T) {
	c, _ := primp.NewClient(primp.ClientOptions{
		Impersonate:   primp.ImpersonateChromeV146,
		ImpersonateOS: primp.ImpersonateOSWindows,
	})
	defer c.Close()

	r, err := c.Get("https://httpbin.org/user-agent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer r.Close()

	var body struct {
		UserAgent string `json:"user-agent"`
	}
	_ = r.JSON(&body)
	ua := strings.ToLower(body.UserAgent)
	if !strings.Contains(ua, "chrome") && !strings.Contains(ua, "crios") {
		t.Errorf("UA=%q,期望包含 chrome 或 crios", body.UserAgent)
	}
	t.Logf("Chrome UA: %s", body.UserAgent)
}

// TestTimeout 验证超时错误能被 errors.Is 识别。
func TestTimeout(t *testing.T) {
	c, _ := primp.NewClient(primp.ClientOptions{Timeout: 100 * time.Millisecond})
	defer c.Close()

	_, err := c.Get("https://httpbin.org/delay/5")
	if err == nil {
		t.Fatal("期望超时错误")
	}
	if !errors.Is(err, primp.ErrTimeout) {
		t.Errorf("err=%v(%T),期望 ErrTimeout", err, err)
	}
}

// TestPostJSON 验证 POST + WithJSON 回显。
func TestPostJSON(t *testing.T) {
	r, err := primp.Post("https://httpbin.org/post", primp.WithJSON(map[string]any{"n": 7}))
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	defer r.Close()

	var body struct {
		JSON map[string]any `json:"json"`
	}
	_ = r.JSON(&body)
	if body.JSON["n"] != float64(7) {
		t.Errorf("回显 json=%v,期望 n=7", body.JSON)
	}
}
