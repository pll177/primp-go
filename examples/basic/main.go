// 示例:用 Chrome 146 指纹访问 httpbin,打印 UA 和返回的 JSON。
package main

import (
	"fmt"
	"log"

	primp "github.com/pll177/primp-go"
)

func main() {
	// 创建一个模拟 Chrome 146 / Windows 的客户端。
	client, err := primp.NewClient(primp.ClientOptions{
		Impersonate:   primp.ImpersonateChromeV146,
		ImpersonateOS: primp.ImpersonateOSWindows,
	})
	if err != nil {
		log.Fatalf("创建 client 失败: %v", err)
	}
	defer client.Close()

	// 发送 GET 请求,带查询参数。
	resp, err := client.Get(
		"https://httpbin.org/get",
		primp.WithParams(map[string]string{"city": "Beijing"}),
	)
	if err != nil {
		log.Fatalf("请求失败: %v", err)
	}
	defer resp.Close()

	fmt.Printf("状态码: %d\n", resp.StatusCode())
	fmt.Printf("URL:    %s\n", resp.URL())
	fmt.Printf("UA:     %s\n", resp.Headers()["User-Agent"])
	fmt.Printf("响应体长度: %d 字节\n", len(resp.Bytes()))

	// 也可以直接用模块级函数(每次创建临时 client)
	r2, err := primp.Post(
		"https://httpbin.org/post",
		primp.WithJSON(map[string]any{"msg": "hello primp-go"}),
	)
	if err != nil {
		log.Fatalf("Post 失败: %v", err)
	}
	defer r2.Close()
	fmt.Printf("\nPOST 状态码: %d\n", r2.StatusCode())
	fmt.Printf("POST 响应:   %s\n", r2.Text())
}
