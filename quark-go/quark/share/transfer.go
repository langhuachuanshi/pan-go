// 本文件实现夸克网盘"转存他人分享"功能。
//
// 转存四步（全部走夸克 API，参考 quark-auto-save 权威实现）：
//  1. POST /share/sharepage/token   {pwd_id, passcode}              → data.stoken
//  2. GET  /share/sharepage/detail  {pwd_id, stoken, pdir_fid, ...} → data.list[]（每个含 fid + share_fid_token）
//  3. POST /share/sharepage/save    {fid_list[], fid_token_list[], to_pdir_fid, pwd_id, stoken, ...} → data.task_id
//  4. GET  /task                    {task_id}                       → 轮询到 status==2，新 fid 在 data.save_as.save_as_top_fids[]
//
// 关键：/s/xxxxx 的 xxxx 是 pwd_id（无 share_id 概念）；save 异步，需轮询 task；
// save 需要并列的 fid_list + fid_token_list（per-file 令牌，不同于 stoken）。
package share

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/langhuachuanshi/quark-go/quark/invoker"
)

// save 批量上限（save_as_top_fids 单次最多返回 100）。
const saveBatchSize = 100

// rePwdID 从分享 URL 抽 pwd_id（/s/(\w+)）。
var rePwdID = regexp.MustCompile(`/s/([A-Za-z0-9]+)`)

// rePasscode 从 URL 抽 passcode（pwd=(\w+)），可选。
var rePasscode = regexp.MustCompile(`[?&]pwd=([A-Za-z0-9]+)`)

// Transfer 一键转存他人分享，返回转存到目标目录后的新 fid 列表。
//
// shareURL 形如 "https://pan.quark.cn/s/xxxxx"（带密码的可在 URL 里 ?pwd=yyy，或用 passcode 参数）。
// passcode 非空时优先用；否则尝试从 URL 抽取；公开分享留空。
// toDirFID 是转存目标目录，留空或 "0" 表示根目录。
// 默认转存分享根目录下的全部文件/文件夹。
func (s *Service) Transfer(ctx context.Context, shareURL, passcode, toDirFID string) ([]string, error) {
	pwdID, urlPwd, err := ParseShareURL(shareURL)
	if err != nil {
		return nil, err
	}
	if passcode == "" {
		passcode = urlPwd
	}
	if toDirFID == "" {
		toDirFID = "0"
	}

	stoken, err := s.GetShareToken(ctx, pwdID, passcode)
	if err != nil {
		return nil, err
	}
	files, err := s.ListShareFiles(ctx, pwdID, stoken)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, invoker.NewAPIError(0, "分享内无可转存文件")
	}

	// 按 100 分批转存，每批 SaveShare + WaitTask。
	var allNewFIDs []string
	for start := 0; start < len(files); start += saveBatchSize {
		end := start + saveBatchSize
		if end > len(files) {
			end = len(files)
		}
		batch := files[start:end]
		taskID, err := s.SaveShare(ctx, pwdID, stoken, toDirFID, batch)
		if err != nil {
			return allNewFIDs, err
		}
		fids, err := s.WaitTask(ctx, taskID)
		if err != nil {
			return allNewFIDs, err
		}
		allNewFIDs = append(allNewFIDs, fids...)
	}
	return allNewFIDs, nil
}

// GetShareToken 换取访问分享的 stoken（步骤①）。
// 公开分享 passcode 传空。
func (s *Service) GetShareToken(ctx context.Context, pwdID, passcode string) (string, error) {
	if pwdID == "" {
		return "", invoker.NewAPIError(0, "pwdID is required")
	}
	body := map[string]any{"pwd_id": pwdID, "passcode": passcode}
	var resp struct {
		Status int    `json:"status"`
		Code   int    `json:"code"`
		Msg    string `json:"message"`
		Data   struct {
			Stoken string `json:"stoken"`
		} `json:"data"`
	}
	if err := invoker.PostAndDecode(ctx, s.inv, "/share/sharepage/token", body, nil, &resp); err != nil {
		return "", err
	}
	// token 接口用 status==200 判有效（status!=200 多为分享失效）。
	if resp.Status != 200 {
		return "", invoker.NewAPIError(resp.Code, resp.Msg)
	}
	if resp.Data.Stoken == "" {
		return "", invoker.NewAPIError(resp.Code, "响应未返回 stoken")
	}
	return resp.Data.Stoken, nil
}

// ListShareFiles 列出分享根目录的全部文件（步骤②，自动分页）。
func (s *Service) ListShareFiles(ctx context.Context, pwdID, stoken string) ([]*ShareFile, error) {
	if pwdID == "" || stoken == "" {
		return nil, invoker.NewAPIError(0, "pwdID and stoken are required")
	}
	const pdirFID = "0" // 默认列分享根目录
	const size = 50

	var all []*ShareFile
	page := 1
	for {
		params := map[string]any{
			"pr":           "ucpro",
			"fr":           "pc",
			"pwd_id":       pwdID,
			"stoken":       stoken,
			"pdir_fid":     pdirFID,
			"force":        "0",
			"_page":        page,
			"_size":        size,
			"_fetch_total": "1",
			"_sort":        "file_type:asc,updated_at:desc",
			"ver":          "2",
		}
		var resp struct {
			Code int    `json:"code"`
			Msg  string `json:"message"`
			Data struct {
				List []struct {
					FID           string `json:"fid"`
					ShareFIDToken string `json:"share_fid_token"`
					FileName      string `json:"file_name"`
					Size          int64  `json:"size"`
					Dir           bool   `json:"dir"`
				} `json:"list"`
			} `json:"data"`
			Metadata struct {
				Total int `json:"_total"`
			} `json:"metadata"`
		}
		if err := invoker.GetAndDecode(ctx, s.inv, "/share/sharepage/detail", params, &resp); err != nil {
			return nil, err
		}
		if resp.Code != 0 {
			return nil, invoker.NewAPIError(resp.Code, resp.Msg)
		}
		for _, it := range resp.Data.List {
			all = append(all, &ShareFile{
				FID:           it.FID,
				ShareFIDToken: it.ShareFIDToken,
				FileName:      it.FileName,
				Size:          it.Size,
				IsFolder:      it.Dir,
			})
		}
		if len(resp.Data.List) < size || len(all) >= resp.Metadata.Total {
			break
		}
		page++
	}
	return all, nil
}

// SaveShare 把 files 转存到 toDirFID（步骤③），返回异步任务 task_id。
// 单次建议 ≤100 个文件（save_as_top_fids 上限 100），Transfer 已自动分批。
func (s *Service) SaveShare(ctx context.Context, pwdID, stoken, toDirFID string, files []*ShareFile) (string, error) {
	if len(files) == 0 {
		return "", invoker.NewAPIError(0, "files is required")
	}
	fidList := make([]string, 0, len(files))
	fidTokenList := make([]string, 0, len(files))
	for _, f := range files {
		fidList = append(fidList, f.FID)
		fidTokenList = append(fidTokenList, f.ShareFIDToken)
	}
	body := map[string]any{
		"fid_list":       fidList,
		"fid_token_list": fidTokenList,
		"to_pdir_fid":    toDirFID,
		"pwd_id":         pwdID,
		"stoken":         stoken,
		"pdir_fid":       "0",
		"scene":          "link",
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"message"`
		Data struct {
			TaskID string `json:"task_id"`
		} `json:"data"`
	}
	if err := invoker.PostAndDecode(ctx, s.inv, "/share/sharepage/save", body, nil, &resp); err != nil {
		return "", err
	}
	if resp.Code != 0 {
		return "", invoker.NewAPIError(resp.Code, resp.Msg)
	}
	if resp.Data.TaskID == "" {
		return "", invoker.NewAPIError(0, "save 未返回 task_id")
	}
	return resp.Data.TaskID, nil
}

// WaitTask 轮询转存任务直到完成（步骤④），返回转存后的新 fid 列表（save_as_top_fids）。
// 超时约 60s（120 次 × 500ms）。
func (s *Service) WaitTask(ctx context.Context, taskID string) ([]string, error) {
	if taskID == "" {
		return nil, invoker.NewAPIError(0, "taskID is required")
	}
	for attempt := 0; attempt < 120; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		params := map[string]any{
			"task_id":     taskID,
			"retry_index": attempt,
		}
		var resp struct {
			Status int    `json:"status"`
			Code   int    `json:"code"`
			Msg    string `json:"message"`
			Data   struct {
				Status int `json:"status"` // 2 = 完成
				SaveAs struct {
					SaveAsTopFIDs []string `json:"save_as_top_fids"`
				} `json:"save_as"`
			} `json:"data"`
		}
		if err := invoker.GetAndDecode(ctx, s.inv, "/task", params, &resp); err != nil {
			return nil, err
		}
		if resp.Status != 200 {
			return nil, invoker.NewAPIError(resp.Code, resp.Msg)
		}
		if resp.Data.Status == 2 {
			return resp.Data.SaveAs.SaveAsTopFIDs, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil, fmt.Errorf("转存任务超时未完成: task_id=%s", taskID)
}

// ParseShareURL 从分享 URL 抽取 pwd_id 和可选 passcode。
// 形如 "https://pan.quark.cn/s/xxxxx"（带密码 "?pwd=yyy" 也会抽出来）。
func ParseShareURL(rawURL string) (pwdID, passcode string, err error) {
	m := rePwdID.FindStringSubmatch(rawURL)
	if m == nil {
		return "", "", invoker.NewAPIError(0, "无法从 URL 解析 pwd_id（期望含 /s/xxxxx）")
	}
	pwdID = m[1]
	if pm := rePasscode.FindStringSubmatch(rawURL); pm != nil {
		passcode = pm[1]
	}
	return pwdID, passcode, nil
}
