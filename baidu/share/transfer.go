package share

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/langhuachuanshi/baidupan-go/baidu/invoker"
)

// TransferResult 转存结果。
type TransferResult struct {
	Filename string // 转存的文件名
	FsID     int64  // 转存后的 fs_id
	Path     string // 转存后的路径
}

// sharePageInfo 从分享页面提取的信息。
type sharePageInfo struct {
	BDstoken string
	UK       string
	ShareUK  string
	ShareID  string
}

// pageInfoRE 从分享页面 JS 提取 bdstoken/uk/share_uk/shareid。
var pageInfoRE = regexp.MustCompile(`"bdstoken":"(.*?)".*?"uk":(\d+).*?"share_uk":"?(\d+)"?.*?"shareid":"?(\d+)"?`)

// extractSharePage 访问分享页面，提取鉴权信息。
func (s *Service) extractSharePage(ctx context.Context, httpClient *http.Client, surl string) (*sharePageInfo, error) {
	shareURL := "https://pan.baidu.com/s/" + surl
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, shareURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://pan.baidu.com/disk/home")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("访问分享页失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取分享页失败: %w", err)
	}

	// 检查页面状态
	bodyStr := string(body)
	if strings.Contains(bodyStr, "error-404") || strings.Contains(bodyStr, "platform-non-found") {
		return nil, fmt.Errorf("分享链接不存在或已失效")
	}

	match := pageInfoRE.FindStringSubmatch(bodyStr)
	if len(match) < 5 {
		return nil, fmt.Errorf("未能从分享页提取鉴权信息（可能需要 STOKEN）")
	}

	return &sharePageInfo{
		BDstoken: match[1],
		UK:       match[2],
		ShareUK:  match[3],
		ShareID:  match[4],
	}, nil
}

// shareVerifyResp 密码验证响应。
type shareVerifyResp struct {
	Errno  int    `json:"errno"`
	Randsk string `json:"randsk"`
}

// transferShareListResp 转存时获取分享文件列表的响应（与 share.go 的 shareListResp 不同）。
type transferShareListResp struct {
	Errno int `json:"errno"`
	List  []struct {
		FSID           int64  `json:"fs_id"`
		ServerFilename string `json:"server_filename"`
		Path           string `json:"path"`
		Size           int64  `json:"size"`
		IsDir          int    `json:"isdir"`
	} `json:"list"`
}

// shareTransferResp 转存响应。
type shareTransferResp struct {
	Errno int                        `json:"errno"`
	Info  []shareTransferInfoItem    `json:"info"`
}

type shareTransferInfoItem struct {
	Errno int    `json:"errno"`
	Path  string `json:"path"`
	FSID  int64  `json:"fsid"`
}

// TransferSave 转存分享文件到自己的网盘。
//
// 流程：
//  1. 访问分享页获取鉴权信息
//  2. 验证提取码（如有）
//  3. 获取分享文件列表
//  4. 转存到目标目录
//
// surl: 分享链接的短码（如 1abc2def345），从 https://pan.baidu.com/s/1abc2def345 提取
// pwd: 提取码（4位，无密码则传空字符串）
// destDir: 保存到自己的网盘目录（绝对路径，如 /）
func (s *Service) TransferSave(ctx context.Context, httpClient *http.Client, surl, pwd, destDir string) ([]*TransferResult, error) {
	if destDir == "" {
		destDir = "/"
	}

	// 1. 访问分享页
	info, err := s.extractSharePage(ctx, httpClient, surl)
	if err != nil {
		return nil, err
	}

	// 2. 验证密码（如果有密码）
	if pwd != "" {
		verifyURL := fmt.Sprintf(
			"https://pan.baidu.com/share/verify?shareid=%s&time=%s&clienttype=1&uk=%s",
			info.ShareID, "", info.ShareUK,
		)
		// 简化：直接拼接
		verifyURL = fmt.Sprintf(
			"https://pan.baidu.com/share/verify?shareid=%s&uk=%s&t=%d",
			info.ShareID, info.ShareUK, 0,
		)

		body := map[string]string{
			"pwd":       pwd,
			"vcode":     "",
			"vcode_str": "",
			"bdstoken":  info.BDstoken,
		}

		data, _, err := s.inv.PostFormRaw(ctx, verifyURL, body)
		if err != nil {
			return nil, fmt.Errorf("验证提取码失败: %w", err)
		}

		var verifyResp shareVerifyResp
		if err := json.Unmarshal(data, &verifyResp); err != nil {
			return nil, fmt.Errorf("解析验证响应失败: %w", err)
		}
		if verifyResp.Errno != 0 {
			if verifyResp.Errno == -9 {
				return nil, fmt.Errorf("提取码错误")
			}
			return nil, invoker.NewAPIError(verifyResp.Errno, "验证失败")
		}
	}

	// 3. 获取分享文件列表
	listURL := fmt.Sprintf(
		"https://pan.baidu.com/share/list?app_id=250528&channel=chunlei&clienttype=0&web=1"+
			"&shareid=%s&from=%s&bdstoken=%s&shorturl=%s&root=1",
		info.ShareID, info.ShareUK, info.BDstoken, surl,
	)
	data, _, err := s.inv.GetRaw(ctx, listURL)
	if err != nil {
		return nil, fmt.Errorf("获取分享文件列表失败: %w", err)
	}

	var listResp transferShareListResp
	if err := json.Unmarshal(data, &listResp); err != nil {
		return nil, fmt.Errorf("解析文件列表失败: %w", err)
	}
	if listResp.Errno != 0 {
		return nil, invoker.NewAPIError(listResp.Errno, "获取文件列表失败")
	}
	if len(listResp.List) == 0 {
		return nil, fmt.Errorf("分享中没有文件")
	}

	// 4. 转存
	var fsIDs []string
	for _, f := range listResp.List {
		fsIDs = append(fsIDs, fmt.Sprintf("%d", f.FSID))
	}
	fsidList := "[" + strings.Join(fsIDs, ",") + "]"

	transferURL := fmt.Sprintf(
		"https://pan.baidu.com/share/transfer?app_id=250528&channel=chunlei&clienttype=0&web=1"+
			"&shareid=%s&from=%s&bdstoken=%s",
		info.ShareID, info.ShareUK, info.BDstoken,
	)

	transferBody := map[string]string{
		"fsidlist": fsidList,
		"path":     destDir,
	}

	data, _, err = s.inv.PostFormRaw(ctx, transferURL, transferBody)
	if err != nil {
		return nil, fmt.Errorf("转存失败: %w", err)
	}

	var transferResp shareTransferResp
	if err := json.Unmarshal(data, &transferResp); err != nil {
		return nil, fmt.Errorf("解析转存响应失败: %w", err)
	}
	if transferResp.Errno != 0 {
		return nil, invoker.NewAPIError(transferResp.Errno, "转存失败")
	}

	var results []*TransferResult
	for i, item := range transferResp.Info {
		filename := ""
		if i < len(listResp.List) {
			filename = listResp.List[i].ServerFilename
		}
		results = append(results, &TransferResult{
			Filename: filename,
			Path:     item.Path,
			FsID:     item.FSID,
		})
	}

	return results, nil
}

// TransferQuery 简化转存：通过分享 URL 和密码转存到指定目录。
// shareURL: 完整分享链接（如 https://pan.baidu.com/s/1xxx?pwd=abcd 或 https://pan.baidu.com/s/1xxx）
// destDir: 目标目录，默认 "/"
func (s *Service) TransferQuery(ctx context.Context, httpClient *http.Client, shareURL, destDir string) ([]*TransferResult, error) {
	// 从 URL 解析 surl 和 pwd
	u, err := url.Parse(shareURL)
	if err != nil {
		return nil, fmt.Errorf("解析分享 URL 失败: %w", err)
	}

	// 提取 surl (去掉 /s/ 前缀)
	surl := strings.TrimPrefix(u.Path, "/s/")
	surl = strings.TrimPrefix(surl, "1") // 去掉开头的 1（如 1abc → abc）
	if surl == "" {
		return nil, fmt.Errorf("无法从 URL 提取分享码")
	}

	// 提取密码（从 query 参数或 path）
	pwd := u.Query().Get("pwd")
	if pwd == "" {
		pwd = u.Query().Get("提取码")
	}

	return s.TransferSave(ctx, httpClient, surl, pwd, destDir)
}
