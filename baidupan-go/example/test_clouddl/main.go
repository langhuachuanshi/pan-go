package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/langhuachuanshi/baidupan-go/baidu"
)

func main() {
	bduss := os.Getenv("PANBAIDU_BDUSS")
	stoken := os.Getenv("PANBAIDU_STOKEN")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c, err := baidu.New(ctx, &baidu.Config{BDUSS: bduss, STOKEN: stoken})
	if err != nil {
		log.Fatal(err)
	}

	// 1. 列出已有任务
	fmt.Println("=== 1. ListTasks ===")
	tasks, err := c.CloudDL().ListTasks(ctx)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
	} else {
		fmt.Printf("✅ 共 %d 个任务\n", len(tasks))
		for _, t := range tasks {
			fmt.Printf("   [%s] id=%d %s -> %s\n", t.StatusText, t.TaskID, t.TaskName, t.SavePath)
		}
	}

	// 2. AddTask (用小文件测试)
	fmt.Println("\n=== 2. AddTask ===")
	taskID, err := c.CloudDL().AddTask(ctx,
		"https://www.baidu.com/img/PCtm_d9c8750bed0b3c7d089fa7d55720d6cf.png",
		"/temp",
	)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
	} else {
		fmt.Printf("✅ 任务已添加 taskID=%d\n", taskID)

		// 3. QueryTask
		fmt.Println("\n=== 3. QueryTask ===")
		qtasks, err := c.CloudDL().QueryTask(ctx, []int64{taskID})
		if err != nil {
			fmt.Printf("❌ %v\n", err)
		} else if len(qtasks) > 0 {
			fmt.Printf("✅ taskID=%d status=%s size=%d/%d\n",
				qtasks[0].TaskID, qtasks[0].StatusText, qtasks[0].FinishedSize, qtasks[0].FileSize)
		}

		// 4. CancelTask
		fmt.Println("\n=== 4. CancelTask ===")
		if err := c.CloudDL().CancelTask(ctx, taskID); err != nil {
			fmt.Printf("⚠️  %v\n", err)
		} else {
			fmt.Println("✅ 已取消")
		}
	}

	fmt.Println("\n🎉 完成")
}
