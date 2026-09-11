package lanzou

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GetDirList 获取子文件夹列表
// fid: 父文件夹ID，根目录传 -1（蓝奏云根目录的 folder_id 是 -1，不是 0）
func (c *Client) GetDirList(fid int) (*FolderList, error) {
	if !c.isLoggedIn() {
		return nil, ErrNotLoggedIn
	}
	c.initUIDAndVei()
	data := map[string]string{
		"task":      "47",
		"folder_id": fmt.Sprintf("%d", fid),
		"vei":       c.vei,
	}
	body, _, err := c.post(c.apiURL(pathTaskAPI), data, nil)
	if err != nil {
		return nil, fmt.Errorf("get dir list failed: %w", err)
	}

	var resp FolderList
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("%w: invalid folder list response", ErrAPIError)
	}
	// zt=1 正常, zt=2 也是成功（空列表时蓝奏云返回 zt=2）
	if resp.Zt != 1 && resp.Zt != 2 {
		return nil, fmt.Errorf("%w: zt=%d info=%s", ErrAPIError, resp.Zt, string(resp.Info))
	}
	return &resp, nil
}

// NewFolder 创建文件夹
// name: 文件夹名称, parentID: 父文件夹ID（根目录传 -1）
func (c *Client) NewFolder(name string, parentID int) (*FolderInfo, error) {
	if !c.isLoggedIn() {
		return nil, ErrNotLoggedIn
	}
	c.initUIDAndVei()
	data := map[string]string{
		"task":      "2",
		"parent_id": fmt.Sprintf("%d", parentID),
		"folder_name": name,
		"folder_description": "",
		"vei":       c.vei,
	}
	body, _, err := c.post(c.apiURL(pathTaskAPI), data, nil)
	if err != nil {
		return nil, fmt.Errorf("create folder failed: %w", err)
	}

	var resp struct {
		Zt   int    `json:"zt"`
		Info string `json:"info"`
		Text string `json:"text"` // 直接返回文件夹ID字符串
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("%w: invalid create folder response", ErrAPIError)
	}
	if resp.Zt != 1 {
		return nil, fmt.Errorf("%w: %s", ErrAPIError, resp.Info)
	}

	return &FolderInfo{
		FolID: resp.Text,
		Name:  name,
	}, nil
}

// DeleteFolder 删除文件夹
// fids: 文件夹ID列表
func (c *Client) DeleteFolder(fids []string) error {
	if !c.isLoggedIn() {
		return ErrNotLoggedIn
	}
	c.initUIDAndVei()
	data := map[string]string{
		"task":      "3",
		"folder_id": strings.Join(fids, "-"),
		"vei":       c.vei,
	}
	body, _, err := c.post(c.apiURL(pathTaskAPI), data, nil)
	if err != nil {
		return fmt.Errorf("delete folder failed: %w", err)
	}
	return c.checkRespBody(body)
}

// MoveFolder 移动文件夹到指定位置
// fids: 文件夹ID列表, fid: 目标文件夹ID
func (c *Client) MoveFolder(fids []string, fid int) error {
	if !c.isLoggedIn() {
		return ErrNotLoggedIn
	}
	c.initUIDAndVei()
	data := map[string]string{
		"task":        "24",
		"folder_id":   fmt.Sprintf("%d", fid),
		"folder_ids":  strings.Join(fids, "-"),
	}
	body, _, err := c.post(c.apiURL(pathTaskAPI), data, nil)
	if err != nil {
		return fmt.Errorf("move folder failed: %w", err)
	}
	return c.checkRespBody(body)
}
