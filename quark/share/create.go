package share

import (
	"context"
	"crypto/rand"
	"strconv"
	"time"

	"github.com/langhuachuanshi/pan-go/quark/invoker"
	"github.com/langhuachuanshi/pan-go/quark/types"
)

// genPasscode 生成符合夸克规范的提取码：4位，含大小写字母和数字。
// 如 "aB3x"。夸克拒绝纯数字（如 "1234"）。
func genPasscode() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 4)
	rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

// 本文件实现夸克网盘创建分享的完整流程。
//
// 夸克创建分享是 3 步（抓包确认）：
//  1. POST /share        → 拿 task_id + share_id（异步任务，立即返回 task_resp）
//  2. GET  /task         → 轮询任务状态（可选，步骤1的task_sync=true已同步完成）
//  3. POST /share/password → 用 share_id 完成分享，返回最终 share_url（短链）
//
// 关键：第3步即使无密码也必须调，浏览器就是这么做的，否则链接无效（"已删除"）。

// CreateRequest 创建分享请求。
type CreateRequest struct {
	FIDs         []string // 文件/文件夹 fid 列表（必填）
	Title        string   // 分享标题（可选）
	Forever      bool     // 是否永久有效（true=永久，false 用 ExpiredDays）
	ExpiredDays  int      // 有效天数（Forever=false 时生效，如 1/7/30）
	WithPasscode bool     // 是否带提取码（私密分享）
	Passcode     string   // 提取码（WithPasscode=true 时用）
}

// Create 创建分享链接，返回可用的 https://pan.quark.cn/s/xxx 短链。
//
// 支持永久/限时（1/7/30天）、公开/私密（带提取码）。
func (s *Service) Create(ctx context.Context, req *CreateRequest) (*types.CreateShareResponse, error) {
	if req == nil || len(req.FIDs) == 0 {
		return nil, invoker.NewAPIError(0, "fids is required")
	}

	// 转 expired_type。
	expiredType := types.ExpiredForever
	if !req.Forever {
		switch req.ExpiredDays {
		case 1:
			expiredType = types.Expired1Day
		case 7:
			expiredType = types.Expired7Day
		case 30:
			expiredType = types.Expired30Day
		default:
			expiredType = types.Expired30Day
		}
	}
	urlType := 1 // 公开
	if req.WithPasscode {
		urlType = 2
	}

	// —— 步骤1：POST /share 拿 task_id + share_id ——
	// 注意：私密分享的 passcode 在第1步就传（抓包确认），不是第3步。
	// task_sync=1 强制服务端同步完成任务再返回（文件夹分享是异步任务，
	// 不带此参数时 task_resp 为空导致拿不到 share_id，2026-09-12 实测）。
	body1 := map[string]any{
		"fid_list":     req.FIDs,
		"title":        req.Title,
		"url_type":     urlType,
		"expired_type": expiredType,
		"task_sync":    1,
	}
	passcode := ""
	if req.WithPasscode {
		passcode = req.Passcode
		if passcode == "" {
			passcode = genPasscode() // 未指定则生成符合规范的提取码
		}
		body1["passcode"] = passcode
	}
	var r1 struct {
		Code int    `json:"code"`
		Msg  string `json:"message"`
		Data struct {
			TaskID   string `json:"task_id"`
			TaskResp struct {
				Code int    `json:"code"`
				Msg  string `json:"message"`
				Data struct {
					ShareID string `json:"share_id"`
				} `json:"data"`
			} `json:"task_resp"`
		} `json:"data"`
	}
	if err := invoker.PostAndDecode(ctx, s.inv, "/share", body1, nil, &r1); err != nil {
		return nil, err
	}
	if r1.Code != 0 {
		return nil, invoker.NewAPIError(r1.Code, r1.Msg)
	}
	if r1.Data.TaskResp.Code != 0 {
		return nil, invoker.NewAPIError(r1.Data.TaskResp.Code, r1.Data.TaskResp.Msg)
	}
	shareID := r1.Data.TaskResp.Data.ShareID
	// —— 步骤2：轮询 task 拿 share_id ——
	// 文件分享同步完成（步骤1的 task_resp 直接带 share_id）；
	// 文件夹分享是异步任务（响应 task_sync=false、无 task_resp），
	// 需轮询 GET /task 到 status=2，share_id 平铺在 data 上（2026-09-12 实测）。
	if shareID == "" && r1.Data.TaskID != "" {
		for i := 0; i < 10; i++ {
			var rt struct {
				Code int `json:"code"`
				Data struct {
					Status  int    `json:"status"`
					ShareID string `json:"share_id"`
				} `json:"data"`
			}
			err := s.inv.Get(ctx, "/task", map[string]string{
				"task_id":     r1.Data.TaskID,
				"retry_index": strconv.Itoa(i),
			}, &rt)
			if err == nil && rt.Data.ShareID != "" {
				shareID = rt.Data.ShareID
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	if shareID == "" {
		return nil, invoker.NewAPIError(0, "创建分享未返回 share_id")
	}

	// —— 步骤3：POST /share/password 拿最终 share_url（body 只含 share_id）——
	body3 := map[string]any{
		"share_id": shareID,
	}
	var r3 struct {
		Code int                       `json:"code"`
		Msg  string                    `json:"message"`
		Data types.CreateShareResponse `json:"data"`
	}
	// 分享创建后可能需要短暂时间才可查，重试几次。
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := invoker.PostAndDecode(ctx, s.inv, "/share/password", body3, nil, &r3); err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if r3.Code == 0 && r3.Data.ShareURL != "" {
			// 补 share_id（第3步响应不含，但有用）。
			r3.Data.ShareID = shareID
			if req.WithPasscode {
				r3.Data.Passcode = passcode
			}
			return &r3.Data, nil
		}
		lastErr = invoker.NewAPIError(r3.Code, r3.Msg)
		time.Sleep(500 * time.Millisecond)
	}
	return nil, lastErr
}
