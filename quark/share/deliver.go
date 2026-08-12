// 本文件实现"聚合发货":把多个商品分享聚合成一个新的限时分享。
//
// 适用场景:商品源文件在自己网盘,但工具只存了每个商品的"分享链接",没存 FID。
// 因为分享是本账号自己创建的,分享里的 fid 就是网盘里的真实 fid,可直接反查复用,
// 无需转存、不占额外空间。
//
// 流程:
//  1. 对每个商品分享:ParseShareURL → GetShareToken → ListShareFiles,收集根目录全部 fid
//  2. 汇总所有商品的 fid
//  3. Create 一个限时、带提取码的新分享
//
// 兼容:公开/私密(passcode 可选,可从 URL ?pwd= 抽)、单文件夹/散文件(统一取根目录全部项)。
package share

import (
	"context"
	"fmt"

	"github.com/langhuachuanshi/quark-go/quark/invoker"
	"github.com/langhuachuanshi/quark-go/quark/types"
)

// ResolveFIDs 从「本账号自己的」商品分享反查文件 FID。
//
// 前提:该分享是当前账号创建的(源在自己网盘),这样分享里的 fid 才是本网盘真实 fid,
// 可直接拿去给新分享用。若是他人分享,反查到的 fid 不属于本账号,不能用于 Create。
//
// passcode 非空时优先用;否则尝试从 shareURL 的 ?pwd= 抽取;公开分享留空即可。
// 返回分享根目录下全部文件/文件夹的 fid(单文件夹分享 → 那个文件夹的 fid;散文件 → 各项 fid)。
func (s *Service) ResolveFIDs(ctx context.Context, shareURL, passcode string) ([]string, error) {
	pwdID, urlPwd, err := ParseShareURL(shareURL)
	if err != nil {
		return nil, err
	}
	if passcode == "" {
		passcode = urlPwd
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
		return nil, invoker.NewAPIError(0, "分享内无文件")
	}
	fids := make([]string, 0, len(files))
	for _, f := range files {
		fids = append(fids, f.FID)
	}
	return fids, nil
}

// DeliverRequest 聚合发货请求。
type DeliverRequest struct {
	ShareURLs   []string // 商品分享链接列表(可含 ?pwd= 提取码)
	Passcodes   []string // 对应提取码(可选,下标对齐 ShareURLs;公开分享传空,私密且 URL 无 ?pwd= 时必填)
	Title       string   // 新分享标题(可选)
	ExpiredDays int      // 限时天数,合法值 1/7/30;0 视为 1
}

// DeliverByShareURLs 聚合发货:从多个商品分享反查 FID,创建一个限时、带提取码的新分享。
//
// 买家只需一个链接 + 一个提取码即可取用全部商品内容。
// 默认限时 1 天、带提取码(防白嫖)。新分享到期自动失效,不占额外网盘空间。
//
// 注意:每个商品分享必须是本账号创建的(源在自己网盘),否则反查到的 fid 不可用。
// 若商品分享是私密的,且链接里没带 ?pwd=,需在 Passcodes 里填对应提取码。
func (s *Service) DeliverByShareURLs(ctx context.Context, req *DeliverRequest) (*types.CreateShareResponse, error) {
	if req == nil || len(req.ShareURLs) == 0 {
		return nil, invoker.NewAPIError(0, "ShareURLs is required")
	}
	days := req.ExpiredDays
	if days == 0 {
		days = 1
	}

	// 1. 反查每个商品的 FID,汇总。
	var allFIDs []string
	for i, url := range req.ShareURLs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var pc string
		if i < len(req.Passcodes) {
			pc = req.Passcodes[i]
		}
		fids, err := s.ResolveFIDs(ctx, url, pc)
		if err != nil {
			return nil, fmt.Errorf("反查第 %d 个商品分享失败: %w", i+1, err)
		}
		allFIDs = append(allFIDs, fids...)
	}
	if len(allFIDs) == 0 {
		return nil, invoker.NewAPIError(0, "所有商品分享内均无文件")
	}

	// 2. 聚合创建限时带码分享。
	return s.Create(ctx, &CreateRequest{
		FIDs:         allFIDs,
		Title:        req.Title,
		Forever:      false,
		ExpiredDays:  days,
		WithPasscode: true,
	})
}
