//go:build windows && amd64

// 下游消费者 demo:演示用 go get 拉取 primp-go,直接用,无需 Rust toolchain。
package main

import (
	"fmt"
	"log"
	"time"

	primp "github.com/pll177/primp-go"
)

func main() {
	client, err := primp.NewClient(primp.ClientOptions{
		Impersonate:   primp.ImpersonateChromeV146,
		ImpersonateOS: primp.ImpersonateOSWindows,
		Timeout:       15 * time.Second,
	})
	if err != nil {
		log.Fatalf("NewClient 失败: %v", err)
	}
	defer client.Close()

	fmt.Println("=== GET https://httpbin.org/get ===")
	r, err := client.Get("https://httpbin.org/get?from=primp-go")
	if err != nil {
		log.Fatalf("GET 失败: %v", err)
	}
	defer r.Close()
	fmt.Println("状态码:", r.StatusCode())
	fmt.Println("最终 URL:", r.URL())
	fmt.Println("UA 头:", r.Headers()["User-Agent"])
	fmt.Println()

	var body struct {
		Args    map[string]string `json:"args"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}
	if err := r.JSON(&body); err != nil {
		log.Fatalf("JSON 解析失败: %v", err)
	}
	fmt.Println("回显 args:", body.Args)
	fmt.Println()

	fmt.Println("=== POST https://httpbin.org/post (json) ===")
	r2, err := client.Post(
		"https://httpbin.org/post",
		primp.WithJSON(map[string]any{"hello": "世界", "n": 42}),
	)
	if err != nil {
		log.Fatalf("POST 失败: %v", err)
	}
	defer r2.Close()
	fmt.Println("状态码:", r2.StatusCode())

	var post struct {
		JSON map[string]any `json:"json"`
	}
	_ = r2.JSON(&post)
	fmt.Println("服务器回显 json:", post.JSON)
}
