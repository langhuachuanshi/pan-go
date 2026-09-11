// Package lanzou 提供蓝奏云网盘的 Go 语言 SDK（基于逆向协议，零第三方依赖）。
//
// 架构同 pan-go 其他模块：主包 Client 实现 invoker.Invoker，业务能力经
// Account / Files / Folders / Upload / Download / Recycle / Resolve 访问器提供。
// 会话生命周期（Login/Logout/cookie 注入与导出）在主包 Client 上。
//
// 使用示例：
//
//	c := lanzou.NewClient(lanzou.WithTimeout(30))
//	defer c.Logout()
//
//	// 登录（或 c.SetCookiesFromMap 用已持久化的 cookie 免登）
//	if err := c.Login("user", "pass"); err != nil {
//	    log.Fatal(err)
//	}
//
//	// 文件列表
//	files, err := c.Files().List(-1)
//	for _, f := range files.Text {
//	    fmt.Printf("[%s] %s (%s)\n", f.ID, f.NameAll, f.Size)
//	}
//
//	// 上传 / 下载
//	c.Upload().Stream("local.txt", -1, nil)
//	c.Download().File("local.bin", "https://pan.lanzoul.com/xxxxx")
//
//	// 解析直链（无需登录）
//	c.Resolve().GetDurlByURL("https://pan.lanzoul.com/xxxxx", "")
package lanzou
