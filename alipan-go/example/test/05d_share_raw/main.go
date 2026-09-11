// 极简诊断：对照测 api.aliyundrive.com vs api.alipan.com 的分享列表。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/langhuachuanshi/alipan-go/alipan"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c, _ := alipan.New(ctx, alipan.WithTokenFile("test-account"))

	body := map[string]any{
		"limit":           10,
		"order_by":        "created_at",
		"order_direction": "DESC",
	}
	for _, host := range []string{"https://api.aliyundrive.com", "https://api.alipan.com"} {
		fmt.Printf("=== %s ===\n", host)
		data, status, err := c.PostRaw(ctx, host+"/adrive/v3/share_link/list", body, true, nil, []int{200})
		fmt.Printf("status=%d err=%v\n", status, err)
		var pretty map[string]any
		if json.Unmarshal(data, &pretty) == nil {
			b, _ := json.MarshalIndent(pretty, "", "  ")
			fmt.Println(string(b))
		} else if len(data) > 0 {
			fmt.Println(string(data))
		}
		fmt.Println()
	}
}
