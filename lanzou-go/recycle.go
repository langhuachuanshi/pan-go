package lanzou

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GetRecycleList 获取回收站文件列表
// page: 页码，从1开始
func (c *Client) GetRecycleList(page int) (*RecycleList, error) {
	if !c.isLoggedIn() {
		return nil, ErrNotLoggedIn
	}
	c.initUID()
	data := map[string]string{
		"task": "7",
		"pg":   fmt.Sprintf("%d", page),
	}
	body, _, err := c.post(c.apiURL(pathTaskAPI), data, nil)
	if err != nil {
		return nil, fmt.Errorf("get recycle list failed: %w", err)
	}

	var resp RecycleList
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("%w: invalid recycle list response", ErrAPIError)
	}
	if resp.Zt == 0 {
		return nil, fmt.Errorf("%w: %s", ErrAPIError, resp.Info)
	}
	return &resp, nil
}

// MoveToTrash 将文件移入回收站
// fids: 文件ID列表
func (c *Client) MoveToTrash(fids []string) error {
	if !c.isLoggedIn() {
		return ErrNotLoggedIn
	}
	c.initUID()
	data := map[string]string{
		"task":    "6",
		"file_id": strings.Join(fids, "-"),
	}
	body, _, err := c.post(c.apiURL(pathTaskAPI), data, nil)
	if err != nil {
		return fmt.Errorf("move to trash failed: %w", err)
	}
	return c.checkRespBody(body)
}

// RestoreFiles 从回收站恢复文件
// fids: 文件ID列表
func (c *Client) RestoreFiles(fids []string) error {
	if !c.isLoggedIn() {
		return ErrNotLoggedIn
	}
	c.initUID()
	data := map[string]string{
		"task":    "8",
		"file_id": strings.Join(fids, "-"),
	}
	body, _, err := c.post(c.apiURL(pathTaskAPI), data, nil)
	if err != nil {
		return fmt.Errorf("restore files failed: %w", err)
	}
	return c.checkRespBody(body)
}

// CleanRecycle 清空回收站
func (c *Client) CleanRecycle() error {
	if !c.isLoggedIn() {
		return ErrNotLoggedIn
	}
	c.initUID()
	data := map[string]string{
		"task": "9",
	}
	body, _, err := c.post(c.apiURL(pathTaskAPI), data, nil)
	if err != nil {
		return fmt.Errorf("clean recycle failed: %w", err)
	}
	return c.checkRespBody(body)
}
