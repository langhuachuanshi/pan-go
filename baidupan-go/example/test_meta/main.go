package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/langhuachuanshi/baidupan-go/baidu"
	"github.com/langhuachuanshi/baidupan-go/baidu/file"
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

	// 先列出文件获取 fs_id
	files, err := c.Files().List(ctx, &file.ListRequest{Dir: "/temp", Num: 3})
	if err != nil {
		log.Fatal(err)
	}
	var fsid int64
	for _, f := range files {
		if !f.IsFolder() {
			fsid = f.FSID
			fmt.Printf("文件: %s fs_id=%d\n", f.Path, fsid)
			break
		}
	}

	// Meta (by fs_id)
	fmt.Println("\n=== Meta ===")
	meta, err := c.Files().Meta(ctx, fsid)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
	} else {
		fmt.Printf("✅ fs_id=%d size=%d md5=%s ctime=%d\n",
			meta.FSID, meta.Size, meta.MD5, meta.ServerCTime)
	}

	// BatchMeta
	fmt.Println("\n=== BatchMeta ===")
	list, err := c.Files().BatchMeta(ctx, []int64{fsid})
	if err != nil {
		fmt.Printf("❌ %v\n", err)
	} else {
		fmt.Printf("✅ %d 项:\n", len(list))
		for _, f := range list {
			fmt.Printf("  fs_id=%d size=%d md5=%s path=%s\n",
				f.FSID, f.Size, f.MD5, f.Path)
		}
	}
}
