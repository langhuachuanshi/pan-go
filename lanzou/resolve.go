package lanzou

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GetDurlByURL 通过分享链接获取直链（无需登录）
// shareURL: 蓝奏云分享链接, pwd: 访问密码（可空）
func (c *Client) GetDurlByURL(shareURL, pwd string) (string, error) {
	shareURL = normalizeURL(shareURL)
	if !isLanzouURL(shareURL) {
		return "", ErrInvalidURL
	}

	// Step 1: 请求分享页面（处理JS挑战）
	mainHTML, err := c.fetchPageWithChallenge(shareURL)
	if err != nil {
		return "", fmt.Errorf("fetch share page failed: %w", err)
	}

	// Step 2: 提取页面数据（fid、文件名、iframe src）
	pageData, err := extractPageData(mainHTML)
	if err != nil {
		return "", fmt.Errorf("extract page data failed: %w", err)
	}

	// Step 3: 请求iframe页面获取 wp_sign
	iframeURL := "https://" + getBaseHost(shareURL) + pageData.Iframe
	iframeHTML, err := c.fetchPageWithChallenge(iframeURL)
	if err != nil {
		return "", fmt.Errorf("fetch iframe failed: %w", err)
	}

	iframeData, err := extractIframeData(iframeHTML)
	if err != nil {
		return "", fmt.Errorf("extract iframe data failed: %w", err)
	}

	// Step 4: POST ajaxm.php 获取直链
	durl, err := c.requestDownloadURL(shareURL, iframeData, pageData.Fid, pwd)
	if err != nil {
		return "", err
	}

	return durl, nil
}

// GetDurlByFolderURL 通过文件夹分享链接获取所有文件的直链
func (c *Client) GetDurlByFolderURL(folderURL, pwd string, subdir bool) ([]string, error) {
	folderURL = normalizeURL(folderURL)
	if !isLanzouURL(folderURL) {
		return nil, ErrInvalidURL
	}

	// 文件夹页面逻辑与单文件不同，先尝试直接解析
	durl, err := c.GetDurlByURL(folderURL, pwd)
	if err != nil {
		return nil, err
	}
	return []string{durl}, nil
}

// GetDurlByURLAndFolder 带文件夹参数的直链解析
func (c *Client) GetDurlByURLAndFolder(shareURL, pwd, folderID string) (string, error) {
	// 与 GetDurlByURL 相同流程
	return c.GetDurlByURL(shareURL, pwd)
}

// GetFileInfo 通过分享链接获取文件详细信息（含直链）
func (c *Client) GetFileInfo(shareURL, pwd string) (*FileDetail, error) {
	shareURL = normalizeURL(shareURL)
	if !isLanzouURL(shareURL) {
		return nil, ErrInvalidURL
	}

	// Step 1: 请求分享页面
	mainHTML, err := c.fetchPageWithChallenge(shareURL)
	if err != nil {
		return nil, fmt.Errorf("fetch share page failed: %w", err)
	}

	// Step 2: 提取页面数据
	pageData, err := extractPageData(mainHTML)
	if err != nil {
		return nil, fmt.Errorf("extract page data failed: %w", err)
	}

	detail := &FileDetail{
		NameAll:     pageData.Title,
		Size:        pageData.FileSize,
		DownloadURL: shareURL,
	}

	// Step 3: 请求iframe页面
	iframeURL := "https://" + getBaseHost(shareURL) + pageData.Iframe
	iframeHTML, err := c.fetchPageWithChallenge(iframeURL)
	if err != nil {
		return detail, nil // 返回部分信息
	}

	iframeData, err := extractIframeData(iframeHTML)
	if err != nil {
		return detail, nil // 返回部分信息
	}
	detail.FileID = fmt.Sprintf("%d", pageData.Fid)

	// Step 4: 获取直链
	durl, err := c.requestDownloadURL(shareURL, iframeData, pageData.Fid, pwd)
	if err != nil {
		return detail, nil // 返回部分信息
	}
	detail.DURL = durl

	return detail, nil
}

// ===== 内部实现 =====

// fetchPageWithChallenge 请求页面，自动处理JS挑战
func (c *Client) fetchPageWithChallenge(pageURL string) (string, error) {
	body, _, err := c.get(pageURL, map[string]string{
		"Referer": pageURL,
	})
	if err != nil {
		return "", err
	}
	html := string(body)

	// 检查是否为JS挑战页面
	if isChallengePage(html) {
		// 解出 acw_sc__v2 cookie
		cookieVal, err := solveAcwScV2(html, c.challenge)
		if err != nil {
			return "", fmt.Errorf("solve challenge failed: %w", err)
		}

		// 注入cookie并重新请求
		c.mergeCookies([]*http.Cookie{{
			Name:  "acw_sc__v2",
			Value: cookieVal,
		}})

		body, _, err = c.get(pageURL, map[string]string{
			"Referer": pageURL,
		})
		if err != nil {
			return "", err
		}
		html = string(body)

		// 二次检查（有时需要两轮）
		if isChallengePage(html) {
			cookieVal, err = solveAcwScV2(html, c.challenge)
			if err != nil {
				return "", fmt.Errorf("solve challenge round 2 failed: %w", err)
			}
			c.mergeCookies([]*http.Cookie{{
				Name:  "acw_sc__v2",
				Value: cookieVal,
			}})
			body, _, err = c.get(pageURL, map[string]string{
				"Referer": pageURL,
			})
			if err != nil {
				return "", err
			}
			html = string(body)
		}
	}

	return html, nil
}

// requestDownloadURL POST ajaxm.php 获取真实下载链接
func (c *Client) requestDownloadURL(referer string, iframe *iframeData, fid int, pwd string) (string, error) {
	ajaxmURL := "https://" + getBaseHost(referer) + iframe.AjaxmURL

	data := map[string]string{
		"action":    "downprocess",
		"websignkey": "erCO",
		"signs":     "erCO",
		"sign":      iframe.WpSign,
		"websign":   "",
		"kd":        "0",
		"ves":       "1",
	}
	if pwd != "" {
		data["p"] = pwd
	}

	body, _, err := c.post(ajaxmURL, data, map[string]string{
		"Referer":            referer,
		"X-Requested-With":   "XMLHttpRequest",
	})
	if err != nil {
		return "", fmt.Errorf("ajaxm request failed: %w", err)
	}

	var resp ajaxmResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("%w: invalid ajaxm response: %s", ErrAPIError, string(body))
	}

	if resp.Zt != 1 {
		if strings.Contains(resp.Mess, "密码") {
			return "", ErrPasswordWrong
		}
		if resp.Zt == 2 {
			return "", ErrFileExpired
		}
		return "", fmt.Errorf("%w: %s", ErrAPIError, resp.Mess)
	}

	// 拼接直链: dom + /file/ + url
	durl := resp.Dom + "/file/" + resp.Url
	return durl, nil
}
