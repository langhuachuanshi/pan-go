// Package lanzou 提供蓝奏云网盘的 Go 语言 SDK。
//
// 本库基于 zaxtyson/LanZouCloud-API (Python) 移植，实现了蓝奏云网盘的完整功能：
//   - 登录/登出
//   - 文件列表、上传、下载、删除、移动
//   - 文件夹创建、删除、移动
//   - 回收站管理
//   - 分享链接直链解析
//   - 密码设置
//
// 使用示例：
//
//	client := lanzou.NewClient(
//	    lanzou.WithTimeout(30),
//	)
//
//	// 登录
//	err := client.Login("username", "password")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Logout()
//
//	// 获取文件列表
	//	files, err := client.GetFileList(-1)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, f := range files.Text {
//	    fmt.Printf("[%s] %s (%s)\n", f.ID, f.NameAll, f.Size)
//	}
//
//	// 解析直链（无需登录）
//	durl, err := client.GetDurlByURL("https://pan.lanzoul.com/xxxxx", "")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("直链:", durl)
//
package lanzou
