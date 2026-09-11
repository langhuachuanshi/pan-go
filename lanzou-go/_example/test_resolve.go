package main

import (
	"fmt"
	"log"

	"github.com/langhuachuanshi/lanzou-go"
)

func main() {
	client := lanzou.NewClient(lanzou.WithTimeout(30))

	url := "https://36cq.lanzouo.com/i5qJz2z14zoh"
	fmt.Println("正在解析:", url)

	detail, err := client.GetFileInfo(url, "")
	if err != nil {
		log.Fatalf("解析失败: %v", err)
	}

	fmt.Println("=== 解析结果 ===")
	fmt.Printf("文件名:   %s\n", detail.NameAll)
	fmt.Printf("文件大小: %s\n", detail.Size)
	fmt.Printf("文件ID:   %s\n", detail.FileID)
	fmt.Printf("直链:     %s\n", detail.DURL)
	fmt.Printf("IsNewd:   %d\n", detail.IsNewd)
}
