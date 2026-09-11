// 测试快传分享：资源盘建文件夹→上传文件→快传。
// 资源盘 drive_id=1802078720（list_my_drives 里 name=resource 的）。
package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const resourceDriveID = "1802078720"

var devID string

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	tdata, _ := os.ReadFile(os.Getenv("USERPROFILE") + `\.aligo\test-account.json`)
	var tok struct {
		AccessToken string `json:"access_token"`
		XDeviceID   string `json:"x_device_id"`
	}
	json.Unmarshal(tdata, &tok)
	at := tok.AccessToken
	devID = tok.XDeviceID

	fmt.Println("=== 资源盘快传分享全流程测试 ===")
	fmt.Printf("资源盘 drive_id=%s\n\n", resourceDriveID)

	// 1. 资源盘根目录建文件夹。
	folderBody, _ := json.Marshal(map[string]any{
		"drive_id":       resourceDriveID,
		"parent_file_id": "root",
		"name":           "alipan-go-share-demo",
		"type":           "folder",
		"check_name_mode": "auto_rename",
	})
	folderID := post(ctx, at, "https://api.aliyundrive.com/adrive/v2/file/createWithFolders", folderBody, "建文件夹", func(b []byte) string {
		var r struct{ FileID string `json:"file_id"` }
		json.Unmarshal(b, &r)
		return r.FileID
	})
	if folderID == "" {
		return
	}

	// 2. 写本地测试文件并上传。
	tmpPath := filepath.Join("tmp", "resource-share-test.txt")
	os.MkdirAll("tmp", 0o755)
	content := []byte("资源盘快传测试 " + time.Now().Format(time.RFC3339))
	os.WriteFile(tmpPath, content, 0o644)
	hash := sha1.Sum(content)
	contentHash := hex.EncodeToString(hash[:])

	// 上传：createWithFolders(file) → 秒传判断 → complete。
	uploadFileID := uploadFile(ctx, at, tmpPath, folderID, contentHash, int64(len(content)))
	if uploadFileID == "" {
		return
	}

	// 3. 快传分享。
	shareBody, _ := json.Marshal(map[string]any{
		"drive_file_list": []map[string]string{
			{"drive_id": resourceDriveID, "file_id": uploadFileID},
		},
	})
	fmt.Println("\n[3] 快传分享...")
	shareURL, shareID := post2(ctx, at, "https://api.aliyundrive.com/adrive/v1/share/create", shareBody, func(b []byte) (string, string) {
		var r struct {
			ShareURL string `json:"share_url"`
			ShareID  string `json:"share_id"`
		}
		json.Unmarshal(b, &r)
		return r.ShareURL, r.ShareID
	})
	if shareURL != "" {
		fmt.Printf("\n🎉🎉🎉 [成功] 快传分享链接: %s\n", shareURL)
		fmt.Printf("    share_id=%s\n", shareID)
	} else {
		fmt.Println("\n[失败] 未拿到 share_url")
	}
}

func uploadFile(ctx context.Context, at, localPath, parentID, contentHash string, size int64) string {
	body, _ := json.Marshal(map[string]any{
		"drive_id":        resourceDriveID,
		"parent_file_id":  parentID,
		"name":            filepath.Base(localPath),
		"type":            "file",
		"check_name_mode": "auto_rename",
		"size":            size,
		"content_hash":    contentHash,
		"content_hash_name": "sha1",
		"proof_version":   "v1",
		"part_info_list":  []map[string]int{{"part_number": 1}},
	})
	fmt.Println("[2] 上传文件...")
	data, status := doReq(ctx, at, "https://api.aliyundrive.com/adrive/v2/file/createWithFolders", body)
	var r struct {
		FileID      string `json:"file_id"`
		UploadID    string `json:"upload_id"`
		PartInfoList []struct {
			UploadURL string `json:"upload_url"`
		} `json:"part_info_list"`
		RapidUpload bool `json:"rapid_upload"`
	}
	json.Unmarshal(data, &r)
	if status != 201 && status != 200 {
		fmt.Printf("  create 失败 status=%d body=%s\n", status, string(data))
		return ""
	}
	if r.RapidUpload {
		fmt.Printf("  秒传成功 file_id=%s\n", r.FileID)
		return r.FileID
	}
	// PUT 上传分片。
	if len(r.PartInfoList) > 0 && r.PartInfoList[0].UploadURL != "" {
		fileData, _ := os.ReadFile(localPath)
		req, _ := http.NewRequestWithContext(ctx, "PUT", r.PartInfoList[0].UploadURL, bytes.NewReader(fileData))
		req.Header.Set("User-Agent", "Mozilla/5.0")
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode >= 300 {
			fmt.Printf("  PUT 上传失败: %v status=%d\n", err, httpOk(resp))
			return ""
		}
		resp.Body.Close()
	}
	// complete。
	compBody, _ := json.Marshal(map[string]any{
		"drive_id":  resourceDriveID,
		"file_id":   r.FileID,
		"upload_id": r.UploadID,
	})
	cdata, cstatus := doReq(ctx, at, "https://api.aliyundrive.com/v2/file/complete", compBody)
	var cf struct{ FileID string `json:"file_id"` }
	json.Unmarshal(cdata, &cf)
	fmt.Printf("  上传完成 file_id=%s (complete status=%d)\n", cf.FileID, cstatus)
	return cf.FileID
}

func httpOk(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}

func post(ctx context.Context, at, url string, body []byte, label string, parse func([]byte) string) string {
	data, status := doReq(ctx, at, url, body)
	v := parse(data)
	fmt.Printf("[1] %s status=%d -> %s\n", label, status, v)
	return v
}

func post2(ctx context.Context, at, url string, body []byte, parse func([]byte) (string, string)) (string, string) {
	data, status := doReq(ctx, at, url, body)
	a, b := parse(data)
	fmt.Printf("  status=%d share_url=%s\n", status, a)
	if a == "" {
		fmt.Printf("  完整响应: %s\n", string(data))
	}
	return a, b
}

func doReq(ctx context.Context, at, url string, body []byte) ([]byte, int) {
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	setWebHdr(req, at)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("  请求错误: %v\n", err)
		return nil, 0
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return data, resp.StatusCode
}

func setWebHdr(req *http.Request, at string) {
	req.Header.Set("Authorization", at)
	req.Header.Set("User-Agent", "AliApp(AYSD/5.8.0) com.alicloud.databox/37029260 Channel/36176927979800@rimet_android_5.8.0 language/zh-CN /Android Mobile/Xiaomi Redmi")
	req.Header.Set("Referer", "https://aliyundrive.com")
	req.Header.Set("x-canary", "client=Android,app=adrive,version=v5.8.0")
	req.Header.Set("x-device-id", devID)
	req.Header.Set("x-signature", "f4b7bed5d8524a04051bd2da876dd79afe922b8205226d65855d02b267422adb1e0d8a816b021eaf5c36d101892180f79df655c5712b348c2a540ca136e6b22001")
	req.Header.Set("Content-Type", "application/json")
}
