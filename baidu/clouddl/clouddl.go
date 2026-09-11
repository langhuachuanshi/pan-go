// Package clouddl 实现百度网盘离线下载（网页端 / BDUSS 方案）。
//
// 接口走 pan.baidu.com/rest/2.0/services/cloud_dl，鉴权靠 BDUSS cookie。
//
// 支持：
//   - AddTask：添加 URL/磁力链离线下载任务
//   - QueryTask：按 task_id 精确查询任务进度
//   - ListTasks：列出所有离线下载任务
//   - CancelTask：取消任务
//   - DeleteTask：删除任务记录
//   - ClearTasks：清空所有已完成任务记录
//
// 参考：BaiduPCS-Go。
package clouddl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/langhuachuanshi/pan-go/baidu/invoker"
)

const baseURL = "https://pan.baidu.com/rest/2.0/services/cloud_dl"

// Service 离线下载入口。
type Service struct {
	inv invoker.Invoker
}

// New 创建 clouddl Service。
func New(inv invoker.Invoker) *Service { return &Service{inv: inv} }

// —— 数据类型 ——

// TaskInfo 离线下载任务信息。
type TaskInfo struct {
	TaskID       int64           // 任务 ID
	Status       int             // 0=成功,1=进行中,2=系统错误,3=资源不存在,4=超时,5=下载失败,6=空间不足,7=已取消
	StatusText   string          // 状态描述
	FileSize     int64           // 文件总大小
	FinishedSize int64           // 已下载大小
	CreateTime   int64           // 创建时间（Unix 秒）
	StartTime    int64           // 开始时间
	FinishTime   int64           // 完成时间
	SavePath     string          // 保存路径
	SourceURL    string          // 资源地址
	TaskName     string          // 任务名称
	OdType       int             // 类型
	FileList     []*CloudDlFile  // 文件列表
	Result       int             // 0=查询成功
}

// CloudDlFile 离线下载的文件信息。
type CloudDlFile struct {
	FileName string // 文件名
	FileSize int64  // 文件大小
}

// rawTaskInfo API 返回的原始任务信息（数字字段为字符串）。
type rawTaskInfo struct {
	TaskID       string `json:"task_id"`
	Status       string `json:"status"`
	FileSize     string `json:"file_size"`
	FinishedSize string `json:"finished_size"`
	CreateTime   string `json:"create_time"`
	StartTime    string `json:"start_time"`
	FinishTime   string `json:"finish_time"`
	SavePath     string `json:"save_path"`
	SourceURL    string `json:"source_url"`
	TaskName     string `json:"task_name"`
	OdType       string `json:"od_type"`
	Result       int    `json:"result"`
	FileList     []*struct {
		FileName string `json:"file_name"`
		FileSize string `json:"file_size"`
	} `json:"file_list"`
}

var statusText = map[int]string{
	0: "下载成功",
	1: "下载进行中",
	2: "系统错误",
	3: "资源不存在",
	4: "下载超时",
	5: "资源存在但下载失败",
	6: "存储空间不足",
	7: "任务取消",
}

// —— 公开方法 ——

// AddTask 添加离线下载任务。
// sourceURL: 下载链接（HTTP/HTTPS/磁力链）
// savePath: 保存到的网盘目录（绝对路径，如 /video）
// 返回任务 ID。
func (s *Service) AddTask(ctx context.Context, sourceURL, savePath string) (int64, error) {
	q := url.Values{}
	q.Set("method", "add_task")
	q.Set("app_id", "250528")
	q.Set("task_from", "0")
	q.Set("selected_idx", "1")
	q.Set("save_path", savePath)
	q.Set("source_url", sourceURL)
	fullURL := baseURL + "?" + q.Encode()

	data, _, err := s.inv.PostFormRaw(ctx, fullURL, nil)
	if err != nil {
		return 0, fmt.Errorf("添加离线任务失败: %w", err)
	}

	var resp struct {
		ErrorCode int    `json:"error_code"`
		ErrorMsg  string `json:"error_msg"`
		TaskID    int64  `json:"task_id"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("解析响应失败: %w", err)
	}
	if resp.ErrorCode != 0 {
		return 0, fmt.Errorf("添加离线任务失败: %s (code=%d)", resp.ErrorMsg, resp.ErrorCode)
	}
	return resp.TaskID, nil
}

// QueryTask 按 task_id 查询任务进度。最多 100 个。
func (s *Service) QueryTask(ctx context.Context, taskIDs []int64) ([]*TaskInfo, error) {
	if len(taskIDs) == 0 {
		return nil, nil
	}
	if len(taskIDs) > 100 {
		taskIDs = taskIDs[:100]
	}
	idStrs := make([]string, len(taskIDs))
	for i, id := range taskIDs {
		idStrs[i] = strconv.FormatInt(id, 10)
	}

	q := url.Values{}
	q.Set("method", "query_task")
	q.Set("app_id", "250528")
	q.Set("op_type", "1")
	q.Set("task_ids", strings.Join(idStrs, ","))
	fullURL := baseURL + "?" + q.Encode()

	data, _, err := s.inv.GetRaw(ctx, fullURL)
	if err != nil {
		return nil, fmt.Errorf("查询离线任务失败: %w", err)
	}

	var resp struct {
		ErrorCode int                     `json:"error_code"`
		ErrorMsg  string                  `json:"error_msg"`
		TaskInfo  map[string]*rawTaskInfo `json:"task_info"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	var tasks []*TaskInfo
	for _, idStr := range idStrs {
		raw, ok := resp.TaskInfo[idStr]
		if !ok || raw == nil {
			continue
		}
		ti := convertTask(raw)
		ti.TaskID, _ = strconv.ParseInt(idStr, 10, 64)
		tasks = append(tasks, ti)
	}
	return tasks, nil
}

// ListTasks 列出所有离线下载任务（最近 1000 条）。
func (s *Service) ListTasks(ctx context.Context) ([]*TaskInfo, error) {
	q := url.Values{}
	q.Set("method", "list_task")
	q.Set("app_id", "250528")
	q.Set("need_task_info", "1")
	q.Set("status", "255")
	q.Set("start", "0")
	q.Set("limit", "1000")
	fullURL := baseURL + "?" + q.Encode()

	data, _, err := s.inv.PostFormRaw(ctx, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("列出离线任务失败: %w", err)
	}

	var resp struct {
		ErrorCode int            `json:"error_code"`
		ErrorMsg  string         `json:"error_msg"`
		TaskInfo  []*rawTaskInfo `json:"task_info"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	var tasks []*TaskInfo
	for _, raw := range resp.TaskInfo {
		if raw == nil {
			continue
		}
		ti := convertTask(raw)
		ti.TaskID, _ = strconv.ParseInt(raw.TaskID, 10, 64)
		tasks = append(tasks, ti)
	}
	return tasks, nil
}

// CancelTask 取消离线下载任务。
func (s *Service) CancelTask(ctx context.Context, taskID int64) error {
	return s.manipTask(ctx, "cancel_task", taskID)
}

// DeleteTask 删除离线下载任务记录。
func (s *Service) DeleteTask(ctx context.Context, taskID int64) error {
	return s.manipTask(ctx, "delete_task", taskID)
}

// ClearTasks 清空所有已完成的任务记录。返回清除数量。
func (s *Service) ClearTasks(ctx context.Context) (int, error) {
	q := url.Values{}
	q.Set("method", "clear_task")
	q.Set("app_id", "250528")
	fullURL := baseURL + "?" + q.Encode()

	data, _, err := s.inv.PostFormRaw(ctx, fullURL, nil)
	if err != nil {
		return 0, fmt.Errorf("清空离线任务失败: %w", err)
	}

	var resp struct {
		ErrorCode int    `json:"error_code"`
		ErrorMsg  string `json:"error_msg"`
		Total     int    `json:"total"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("解析响应失败: %w", err)
	}
	return resp.Total, nil
}

// manipTask 取消/删除任务。
func (s *Service) manipTask(ctx context.Context, method string, taskID int64) error {
	q := url.Values{}
	q.Set("method", method)
	q.Set("app_id", "250528")
	q.Set("task_id", strconv.FormatInt(taskID, 10))
	fullURL := baseURL + "?" + q.Encode()

	data, _, err := s.inv.PostFormRaw(ctx, fullURL, nil)
	if err != nil {
		return fmt.Errorf("%s 失败: %w", method, err)
	}

	var resp struct {
		ErrorCode int    `json:"error_code"`
		ErrorMsg  string `json:"error_msg"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}
	if resp.ErrorCode != 0 {
		return fmt.Errorf("%s 失败: %s (code=%d)", method, resp.ErrorMsg, resp.ErrorCode)
	}
	return nil
}

// convertTask 将原始响应转为 TaskInfo。
func convertTask(raw *rawTaskInfo) *TaskInfo {
	ti := &TaskInfo{
		Status:       atoi(raw.Status),
		FileSize:     atoi64(raw.FileSize),
		FinishedSize: atoi64(raw.FinishedSize),
		CreateTime:   atoi64(raw.CreateTime),
		StartTime:    atoi64(raw.StartTime),
		FinishTime:   atoi64(raw.FinishTime),
		SavePath:     raw.SavePath,
		SourceURL:    raw.SourceURL,
		TaskName:     raw.TaskName,
		OdType:       atoi(raw.OdType),
		Result:       raw.Result,
	}
	ti.StatusText = statusText[ti.Status]
	if ti.StatusText == "" {
		ti.StatusText = "未知状态: " + strconv.Itoa(ti.Status)
	}

	for _, f := range raw.FileList {
		if f == nil {
			continue
		}
		ti.FileList = append(ti.FileList, &CloudDlFile{
			FileName: f.FileName,
			FileSize: atoi64(f.FileSize),
		})
	}
	return ti
}

func atoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

func atoi64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
