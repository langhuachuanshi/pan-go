// 完整诊断：复刻浏览器3步创建分享，dump每步响应。
// 1. POST /share → task_id
// 2. GET /task?task_id=xx → 轮询拿 share_id
// 3. POST /share/password → 拿最终短链
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/langhuachuanshi/quark-go/quark"
	"github.com/langhuachuanshi/quark-go/quark/file"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	c, err := quark.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fid := findFile(ctx, c, "0")
	if fid == "" {
		log.Fatal("没找到文件")
	}
	fmt.Printf("文件 fid=%s\n\n", fid)

	// 步骤1：POST /share 拿 task_id。
	body1 := map[string]any{
		"fid_list":     []string{fid},
		"title":        "diag-3step",
		"url_type":     1,
		"expired_type": 1,
	}
	d1, _, err := c.Post(ctx, "/share", body1, nil, nil)
	if err != nil {
		log.Fatalf("步骤1失败: %v", err)
	}
	fmt.Printf("[步骤1 /share]\n%s\n\n", string(d1))
	var r1 struct {
		Code int `json:"code"`
		Data struct {
			TaskID   string `json:"task_id"`
			TaskResp struct {
				Data struct {
					ShareID string `json:"share_id"`
				} `json:"data"`
			} `json:"task_resp"`
		} `json:"data"`
	}
	json.Unmarshal(d1, &r1)
	taskID := r1.Data.TaskID
	shareID := r1.Data.TaskResp.Data.ShareID
	fmt.Printf("→ task_id=%s\n→ share_id=%s\n\n", taskID, shareID)

	if taskID == "" {
		log.Fatal("没拿到 task_id")
	}

	// 步骤2：GET /task?task_id=xx 轮询。
	d2, _, err := c.Get(ctx, "/task", map[string]string{"task_id": taskID, "retry_index": "0"}, nil)
	if err != nil {
		log.Fatalf("步骤2失败: %v", err)
	}
	fmt.Printf("[步骤2 /task]\n%s\n\n", string(d2))

	// 步骤3：POST /share/password（即使无密码也要调，浏览器就是这么做的）。
	// 用步骤1/2拿到的 share_id。
	if shareID == "" {
		fmt.Println("[步骤3] 跳过：没拿到 share_id")
		return
	}
	body3 := map[string]string{"share_id": shareID}
	d3, status3, err := c.Post(ctx, "/share/password", body3, nil, nil)
	if err != nil {
		fmt.Printf("[步骤3] 请求失败(可能是此接口需密码): %v\n", err)
	} else {
		fmt.Printf("[步骤3 /share/password] status=%d\n%s\n", status3, string(d3))
	}
}

func findFile(ctx context.Context, c *quark.Client, pdir string) string {
	files, _ := c.Files().List(ctx, &file.ListRequest{PDirFID: pdir, Size: 50})
	for _, f := range files {
		if f.IsFile() {
			return f.FID
		}
	}
	for _, f := range files {
		if f.IsFolder() {
			if fid := findFile(ctx, c, f.FID); fid != "" {
				return fid
			}
		}
	}
	return ""
}
