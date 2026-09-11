// 示例：alipan-go 快速入门，对应该 aligo 的 quick start。
//
// 运行前请确保已安装阿里云盘 App 用于扫码登录。
//   go run ./example/quickstart
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/langhuachuanshi/alipan-go/alipan"
	"github.com/langhuachuanshi/alipan-go/alipan/file"
)

func main() {
	ctx := context.Background()

	// 创建客户端：默认读取 ~/.aligo/aligo.json 复用登录；不存在则终端扫码。
	// 其它登录方式：
	//   alipan.WithRefreshToken("你的refresh_token") // 直接用 token 登录
	//   alipan.WithQRCodeWeb(8080)                   // 网页扫码
	//   alipan.WithTokenFile("work")                 // 复用 ~/.aligo/work.json
	c, err := alipan.New(ctx)
	if err != nil {
		log.Fatalf("创建客户端失败: %v", err)
	}

	// 1. 用户信息。
	user, err := c.Users().Get(ctx)
	if err != nil {
		log.Fatalf("获取用户信息失败: %v", err)
	}
	fmt.Printf("用户: %s (%s)  电话: %s\n", user.UserName, user.NickName, user.Phone)

	// 2. 列出根目录文件。
	files, err := c.Files().List(ctx, &file.ListRequest{
		ParentFileID: "root",
		DriveID:      c.DefaultDriveID(),
		Limit:        50,
	})
	if err != nil {
		log.Fatalf("获取文件列表失败: %v", err)
	}
	fmt.Printf("\n根目录共 %d 个文件/文件夹:\n", len(files))
	for _, f := range files {
		if f.IsFolder() {
			fmt.Printf("  [文件夹] %s\n", f.Name)
		} else {
			fmt.Printf("  [文件]   %-30s %s\n", f.Name, humanSize(f.Size))
		}
	}

	// —— 以下为示例代码，按需取消注释 ——

	// 3. 搜索文件。
	// results, _ := c.Files().SearchByName(ctx, "报告", c.DefaultDriveID())

	// 4. 创建分享链接（分享根目录第一个文件）。
	// if len(files) > 0 {
	// 	resp, _ := c.Share().Create(ctx, &share.CreateShareLinkRequest{
	// 		FileIDList: []string{files[0].FileID},
	// 		SharePwd:   "1234",
	// 	})
	// 	fmt.Printf("分享链接: %s 提取码: %s\n", resp.ShareURL, resp.SharePwd)
	// }

	// 5. 保存他人分享到自己网盘。
	// token, _ := c.Share().GetShareToken(ctx, "<share_id>", "") // 无密码传空串
	// resp, _ := c.Share().SaveToDrive(ctx, &share.SaveToDriveRequest{
	// 	ShareToken: token,
	// 	FileID:     "<file_id>",
	// })

	// 6. 上传 / 下载。
	// c.Files().Upload(ctx, &file.UploadRequest{
	// 	FilePath: "C:/path/to/file.zip", ParentFileID: "root",
	// 	OnProgress: func(sent, total int64) { fmt.Printf("\r上传: %d/%d", sent, total) },
	// })
	// c.Files().Download(ctx, &file.DownloadRequest{
	// 	FileID: "<file_id>", DriveID: c.DefaultDriveID(), LocalFolder: "./downloads",
	// })
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n := n / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
