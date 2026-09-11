// Package share 实现百度网盘分享链接操作（网页端 / BDUSS 方案）。
//
// 支持：
//   - ShareSet：创建私密分享链接，可设密码和有效期
//   - ShareCancel：取消（删除）分享链接
//   - ShareList：列出所有分享记录
//   - ShareSURLInfo：获取分享详细信息（含密码）
//
// 接口走 pan.baidu.com/share/*，鉴权靠 BDUSS cookie + bdstoken。
// 参考：BaiduPCS-Go。
package share

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"strconv"
	"strings"

	"github.com/langhuachuanshi/pan-go/baidu/invoker"
	"github.com/langhuachuanshi/pan-go/baidu/sign"
)

const panBase = "https://pan.baidu.com"

// Service 分享操作入口。
type Service struct {
	inv invoker.Invoker
}

// New 创建 share Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// —— 数据类型 ——

// ShareOption 分享选项。
type ShareOption struct {
	Password string // 4 位提取码（空则自动生成）
	Period   int    // 有效期天数（0=永久，1=1天，7=7天，30=30天）
}

// Shared 创建分享的返回结果。
type Shared struct {
	Link    string `json:"link"`    // 分享链接
	Pwd     string `json:"pwd"`     // 提取码
	ShareID int64  `json:"shareid"` // 分享 ID
}

// ShareRecordInfo 分享记录（来自 /share/record 列表）。
type ShareRecordInfo struct {
	ShareID   int64   `json:"shareId"`
	FsIds     []int64 `json:"fsIds"`
	Shortlink string  `json:"shortlink"`
	Status    int     `json:"status"`          // 状态
	Public    int     `json:"public"`          // 1=公开，0=私密
	Category  int     `json:"typicalCategory"` // 文件类型
	Path      string  `json:"typicalPath"`     // 典型路径
	ExpireType int    `json:"expiredType"`     // 过期类型
	ExpireTime int64  `json:"expiredTime"`     // 过期时间戳
	ViewCount  int    `json:"vCnt"`            // 浏览次数
}

// ShareSURLInfo 分享短链详情（含密码）。
type ShareSURLInfo struct {
	Pwd      string `json:"pwd"`
	ShortURL string `json:"shorturl"`
}

// ShareRecordList 分享记录列表。
type ShareRecordList []*ShareRecordInfo

// —— 响应结构 ——

type sharePSetResp struct {
	Errno int    `json:"errno"`
	Link  string `json:"link"`
	ShareID int64 `json:"shareid"`
}

type shareCancelResp struct {
	Errno int    `json:"errno"`
	Errmsg string `json:"err_msg"`
}

type shareListResp struct {
	Errno int               `json:"errno"`
	List  ShareRecordList   `json:"list"`
}

type shareSURLInfoResp struct {
	Errno int    `json:"errno"`
	Pwd   string `json:"pwd"`
	ShortURL string `json:"shorturl"`
}

// —— 公开方法 ——

// ShareSet 创建私密分享链接。
// paths: 要分享的文件/目录绝对路径列表。
// opt: 可选密码和有效期。密码为空则自动生成4位数字; Period=0 表示永久。
func (s *Service) ShareSet(ctx context.Context, paths []string, opt *ShareOption) (*Shared, error) {
	if opt == nil {
		opt = &ShareOption{}
	}
	if opt.Password == "" || len(opt.Password) != 4 {
		opt.Password = randomPassword()
	}

	// 构建 path_list JSON
	pathJSON, _ := json.Marshal(paths)

	body := map[string]string{
		"path_list":    string(pathJSON),
		"schannel":     "4",
		"channel_list": "[]",
		"period":       strconv.Itoa(opt.Period),
		"pwd":          opt.Password,
		"share_type":   "9",
	}

	data, _, err := s.inv.PostForm(ctx, "/share/pset", body, nil)
	if err != nil {
		return nil, fmt.Errorf("创建分享失败: %w", err)
	}

	var resp sharePSetResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("解析分享响应失败: %w", err)
	}
		if resp.Errno != 0 {
			return nil, invoker.NewAPIError(resp.Errno, "创建分享失败")
		}
		if resp.Link == "" {
			return nil, invoker.NewAPIError(0, "创建分享失败: 未返回链接")
		}

	return &Shared{
		Link:    resp.Link,
		Pwd:     opt.Password,
		ShareID: resp.ShareID,
	}, nil
}

// ShareCancel 取消（删除）分享。
func (s *Service) ShareCancel(ctx context.Context, shareIDs []int64) error {
	ids := make([]string, len(shareIDs))
	for i, id := range shareIDs {
		ids[i] = strconv.FormatInt(id, 10)
	}
	body := map[string]string{
		"shareid_list": "[" + strings.Join(ids, ",") + "]",
	}

	data, _, err := s.inv.PostForm(ctx, "/share/cancel", body, nil)
	if err != nil {
		return fmt.Errorf("取消分享失败: %w", err)
	}

	var resp shareCancelResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("解析取消分享响应失败: %w", err)
	}
	if resp.Errno != 0 {
		return invoker.NewAPIError(resp.Errno, resp.Errmsg)
	}
	return nil
}

// ShareList 列出分享记录。
// page: 页码，从 1 开始。
func (s *Service) ShareList(ctx context.Context, page int) (ShareRecordList, error) {
	query := url.Values{}
	query.Set("page", strconv.Itoa(page))
	query.Set("desc", "1")
	query.Set("order", "time")

	fullURL := panBase + "/share/record?" + query.Encode()
	data, _, err := s.inv.GetRaw(ctx, fullURL)
	if err != nil {
		return nil, fmt.Errorf("获取分享列表失败: %w", err)
	}

	var resp shareListResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("解析分享列表响应失败: %w", err)
	}
	if resp.Errno != 0 {
		return nil, invoker.NewAPIError(resp.Errno, "获取分享列表失败")
	}
	if resp.List == nil {
		return ShareRecordList{}, nil
	}
	return resp.List, nil
}

// ShareSURLInfo 获取分享短链详细信息（含密码）。
func (s *Service) ShareSURLInfo(ctx context.Context, shareID int64) (*ShareSURLInfo, error) {
	signStr := sign.ShareSURLInfoSign(shareID)
	query := url.Values{}
	query.Set("shareid", strconv.FormatInt(shareID, 10))
	query.Set("sign", signStr)

	fullURL := panBase + "/share/surlinfoinrecord?" + query.Encode()
	data, _, err := s.inv.GetRaw(ctx, fullURL)
	if err != nil {
		return nil, fmt.Errorf("获取分享信息失败: %w", err)
	}

	var resp shareSURLInfoResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("解析分享信息响应失败: %w", err)
	}
	if resp.Errno != 0 {
		return nil, invoker.NewAPIError(resp.Errno, "获取分享信息失败")
	}

	// 去掉 "0" 密码（表示无密码）
	if resp.Pwd == "0" {
		resp.Pwd = ""
	}

	return &ShareSURLInfo{
		Pwd:      resp.Pwd,
		ShortURL: resp.ShortURL,
	}, nil
}

// randomPassword 生成 4 位随机数字密码。
func randomPassword() string {
	return fmt.Sprintf("%04d", rand.Intn(10000))
}
