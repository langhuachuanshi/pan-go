// 验证断点续传可行性：传第1片→(模拟中断)→续传第2、3片→complete。
// 如果 complete 成功且文件完整，说明 OSS 保留已传分片，断点续传可行。
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
	"time"
)

const resourceDriveID = "1802078720"
const chunkSize = 10 * 1024 * 1024

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	at := readToken()["access_token"].(string)

	// 25MB 文件 = 3 片。
	filePath := "tmp/resume-verify.bin"
	os.MkdirAll("tmp", 0o755)
	f, _ := os.Create(filePath)
	chunk := bytes.Repeat([]byte{0xCD}, chunkSize)
	for i := 0; i < 2; i++ {
		f.Write(chunk)
	}
	f.Write(chunk[:5*1024*1024])
	f.Close()
	fileData, _ := os.ReadFile(filePath)
	fileSize := int64(len(fileData))
	h := sha1.Sum(fileData)
	fmt.Printf("[文件] %d 字节, sha1=%s\n", fileSize, hex.EncodeToString(h[:]))

	// 1. create 拿 upload_id。
	createBody, _ := json.Marshal(map[string]any{
		"drive_id": resourceDriveID, "parent_file_id": "root",
		"name": "resume-verify.bin", "type": "file", "size": fileSize,
		"check_name_mode": "auto_rename",
		"part_info_list": []map[string]int{{"part_number": 1}, {"part_number": 2}, {"part_number": 3}},
	})
	cdata, _ := doPost(ctx, at, "https://api.aliyundrive.com/adrive/v2/file/createWithFolders", createBody)
	var cr struct {
		FileID   string `json:"file_id"`
		UploadID string `json:"upload_id"`
		PartInfoList []struct {
			PartNumber int    `json:"part_number"`
			UploadURL  string `json:"upload_url"`
		} `json:"part_info_list"`
	}
	json.Unmarshal(cdata, &cr)
	fmt.Printf("[create] file_id=%s upload_id=%s\n", cr.FileID, cr.UploadID)

	// 2. 只传第1片。
	fmt.Println("[传第1片]")
	putPart(ctx, cr.PartInfoList[0].UploadURL, fileData[:chunkSize])

	// 3. 模拟中断：重新 get_upload_url 拿第2、3片的新URL（旧URL可能过期）。
	fmt.Println("[模拟中断后，重新拿URL续传第2、3片]")
	getBody, _ := json.Marshal(map[string]any{
		"drive_id": resourceDriveID, "file_id": cr.FileID, "upload_id": cr.UploadID,
		"part_info_list": []map[string]int{{"part_number": 2}, {"part_number": 3}},
	})
	gdata, _ := doPost(ctx, at, "https://api.aliyundrive.com/v2/file/get_upload_url", getBody)
	var gr struct {
		PartInfoList []struct {
			PartNumber int    `json:"part_number"`
			UploadURL  string `json:"upload_url"`
		} `json:"part_info_list"`
	}
	json.Unmarshal(gdata, &gr)

	// 4. 续传第2、3片（第1片已传，跳过）。
	for _, p := range gr.PartInfoList {
		start := (p.PartNumber - 1) * chunkSize
		end := start + chunkSize
		if end > len(fileData) {
			end = len(fileData)
		}
		fmt.Printf("[续传第%d片] %d-%d\n", p.PartNumber, start, end)
		putPart(ctx, p.UploadURL, fileData[start:end])
	}

	// 5. complete（带全部3片）。
	fmt.Println("[complete]")
	compBody, _ := json.Marshal(map[string]any{
		"drive_id": resourceDriveID, "file_id": cr.FileID, "upload_id": cr.UploadID,
		"part_info_list": []map[string]int{{"part_number": 1}, {"part_number": 2}, {"part_number": 3}},
	})
	cdata2, cstatus := doPost(ctx, at, "https://api.aliyundrive.com/v2/file/complete", compBody)
	fmt.Printf("[complete] status=%d body=%s\n", cstatus, string(cdata2)[:min(300, len(string(cdata2)))])

	var cf struct {
		FileID string `json:"file_id"`
		Size   int64  `json:"size"`
		Name   string `json:"name"`
	}
	json.Unmarshal(cdata2, &cf)
	if cf.Size == fileSize {
		fmt.Printf("\n🎉 断点续传成功！文件 %s size=%d 完整\n", cf.Name, cf.Size)
	} else {
		fmt.Printf("\n❌ complete 失败或大小不符\n")
	}
}

func doPost(ctx context.Context, at, url string, body []byte) ([]byte, int) {
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Authorization", at)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "AliApp(AYSD/5.8.0) com.alicloud.databox/37029260 Channel/36176927979800@rimet_android_5.8.0 language/zh-CN /Android Mobile/Xiaomi Redmi")
	req.Header.Set("Referer", "https://aliyundrive.com")
	req.Header.Set("x-canary", "client=Android,app=adrive,version=v5.8.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return data, resp.StatusCode
}

func putPart(ctx context.Context, url string, data []byte) {
	req, _ := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(data))
	req.Header.Set("User-Agent", "AliApp(AYSD/5.8.0)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("  PUT错误:", err)
		return
	}
	resp.Body.Close()
	fmt.Printf("  PUT part status=%d\n", resp.StatusCode)
}

func readToken() map[string]any {
	data, _ := os.ReadFile(os.Getenv("USERPROFILE") + `\.aligo\test-account.json`)
	var t map[string]any
	json.Unmarshal(data, &t)
	return t
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
