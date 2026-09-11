package share

// ShareFile 分享内的文件项（/share/sharepage/detail 返回）。
//
// 转存需要同时带 FID 和 ShareFIDToken（per-file 令牌，不同于 stoken）。
type ShareFile struct {
	FID          string // 文件/文件夹 fid
	ShareFIDToken string // 每文件令牌（share_fid_token），转存 save 时必需
	FileName     string // 文件名
	Size         int64  // 字节数（文件夹为 0）
	IsFolder     bool   // 是否文件夹
}
