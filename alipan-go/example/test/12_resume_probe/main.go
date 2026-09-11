// 验证阿里云盘断点续传协议：createWithFolders 拿 upload_id，
// 传前2片，然后调 get_upload_url 看服务端是否返回已传分片状态。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const resourceDriveID = "1802078720"

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	tok := readToken()
	at := tok["access_token"].(string)

	// 1. 生成 25MB 测试文件（3 个 10MB 分片）。
	filePath := "tmp/resume-test.bin"
	os.MkdirAll("tmp", 0o755)
	fmt.Println("[1] 生成 25MB 测试文件...")
	f, _ := os.Create(filePath)
	chunk := bytes.Repeat([]byte{0xAB}, 10*1024*1024)
	for i := 0; i < 2; i++ {
		f.Write(chunk)
	}
	f.Write(chunk[:5*1024*1024]) // 第三片 5MB
	f.Close()
	fi, _ := os.Stat(filePath)
	fmt.Printf("    文件大小: %d 字节 (%.1fMB)\n", fi.Size(), float64(fi.Size())/1024/1024)

	// 2. createWithFolders 拿 upload_id + 3 个分片 URL。
	fmt.Println("[2] createWithFolders...")
	createBody, _ := json.Marshal(map[string]any{
		"drive_id":       resourceDriveID,
		"parent_file_id": "root",
		"name":           "resume-test.bin",
		"type":           "file",
		"size":           fi.Size(),
		"check_name_mode": "auto_rename",
		"part_info_list": []map[string]int{
			{"part_number": 1}, {"part_number": 2}, {"part_number": 3},
		},
	})
	cdata, cstatus := doPost(ctx, at, "https://api.aliyundrive.com/adrive/v2/file/createWithFolders", createBody)
	fmt.Printf("    status=%d\n", cstatus)
	var cr struct {
		FileID       string `json:"file_id"`
		UploadID     string `json:"upload_id"`
		PartInfoList []struct {
			PartNumber int    `json:"part_number"`
			UploadURL  string `json:"upload_url"`
		} `json:"part_info_list"`
		RapidUpload bool `json:"rapid_upload"`
	}
	json.Unmarshal(cdata, &cr)
	fmt.Printf("    file_id=%s upload_id=%s 分片数=%d rapid=%v\n", cr.FileID, cr.UploadID, len(cr.PartInfoList), cr.RapidUpload)
	if cr.RapidUpload || len(cr.PartInfoList) < 3 {
		fmt.Println("    秒传或分片数不对，无法测试续传")
		return
	}

	// 3. 只传第 1 片（模拟中断，第2、3片不传）。
	fmt.Println("[3] 只传第1片（模拟中断）...")
	fileData, _ := os.ReadFile(filePath)
	putPart(ctx, cr.PartInfoList[0].UploadURL, fileData[:10*1024*1024])
	fmt.Println("    第1片已传")

	// 4. 关键：调 get_upload_url，带全部3个 part_number，看服务端返回什么。
	fmt.Println("[4] get_upload_url 查询已传状态（关键）...")
	getBody, _ := json.Marshal(map[string]any{
		"drive_id":  resourceDriveID,
		"file_id":   cr.FileID,
		"upload_id": cr.UploadID,
		"part_info_list": []map[string]int{
			{"part_number": 1}, {"part_number": 2}, {"part_number": 3},
		},
	})
	gdata, gstatus := doPost(ctx, at, "https://api.aliyundrive.com/v2/file/get_upload_url", getBody)
	fmt.Printf("    status=%d\n", gstatus)
	// 完整打印响应，看有没有"已传"标记。
	fmt.Printf("    完整响应: %s\n", string(gdata))

	var gr struct {
		PartInfoList []struct {
			PartNumber int    `json:"part_number"`
			UploadURL  string `json:"upload_url"`
			ETag       string `json:"etag"`
			PartSize   int    `json:"part_size"`
		} `json:"part_info_list"`
	}
	json.Unmarshal(gdata, &gr)
	fmt.Println("    分片状态:")
	for _, p := range gr.PartInfoList {
		etagMark := ""
		if p.ETag != "" {
			etagMark = " (有etag=已传)"
		}
		fmt.Printf("      part %d: url_len=%d etag=%q size=%d%s\n",
			p.PartNumber, len(p.UploadURL), p.ETag, p.PartSize, etagMark)
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
		fmt.Println("    错误:", err)
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
		fmt.Println("    PUT错误:", err)
		return
	}
	resp.Body.Close()
	fmt.Printf("    PUT status=%d\n", resp.StatusCode)
}

func readToken() map[string]any {
	data, _ := os.ReadFile(os.Getenv("USERPROFILE") + `\.aligo\test-account.json`)
	var t map[string]any
	json.Unmarshal(data, &t)
	return t
}
