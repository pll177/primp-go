//go:build windows && amd64

// 示例:用 Chrome 146 指纹抓取网页,演示 base_url、cookie 持久化与重定向。
//
// 运行:
//
//	cargo build --release         # 先在仓库根编译 Rust 库
//	go run ./examples/scraping
package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	primp "github.com/pll177/primp-go"
)

func main() {
	// 1. 构造一个带 Chrome 146 / Windows 指纹、10 秒超时的客户端。
	//    BaseURL 设置后,后续请求传相对路径即可拼接。
	client, err := primp.NewClient(primp.ClientOptions{
		Impersonate:   primp.ImpersonateChromeV146,
		ImpersonateOS: primp.ImpersonateOSWindows,
		Timeout:       10 * time.Second,
		BaseURL:       "https://httpbin.org",
	})
	if err != nil {
		log.Fatalf("创建 client 失败: %v", err)
	}
	defer client.Close()

	// 2. 第一次请求:写入一个 cookie。
	if _, err := client.Get("/cookies/set?session=abc&user=张三"); err != nil {
		log.Fatalf("写入 cookie 失败: %v", err)
	}

	// 3. 第二次请求:cookie 自动随请求附带(因为 CookieStore 默认开启)。
	resp, err := client.Get("/cookies")
	if err != nil {
		log.Fatalf("读取 cookie 失败: %v", err)
	}
	defer resp.Close()

	var body struct {
		Cookies map[string]string `json:"cookies"`
	}
	if err := resp.JSON(&body); err != nil {
		log.Fatalf("解析失败: %v", err)
	}
	fmt.Println("回显 cookie:")
	for k, v := range body.Cookies {
		fmt.Printf("  %s = %s\n", k, v)
	}

	// 4. 演示 RaiseForStatus:遇到 4xx/5xx 时把 status 转成 error。
	bad, _ := client.Get("/status/418")
	defer bad.Close()
	if err := bad.RaiseForStatus(); err != nil {
		fmt.Printf("\n服务器拒绝(预期):%v\n", err)
	}

	// 5. 演示 UA 检查:确认指纹生效。
	uaResp, _ := client.Get("/user-agent")
	defer uaResp.Close()
	var uaBody struct {
		UserAgent string `json:"user-agent"`
	}
	_ = uaResp.JSON(&uaBody)
	if strings.Contains(strings.ToLower(uaBody.UserAgent), "chrome") {
		fmt.Printf("\n指纹生效,UA = %s\n", uaBody.UserAgent)
	}
}
