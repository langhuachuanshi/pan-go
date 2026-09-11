// 独立验证：动态签名 + create_session 能否突破创建分享 403。
// 不改动主流程，仅验证算法正确性。
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/langhuachuanshi/alipan-go/alipan/device"
)

// web 客户端伪装常量（RawChen 方案）。
const (
	webUA     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	webCanary = "client=web,app=adrive,version=v3.17.0"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 读已有 token。
	tf := os.Getenv("USERPROFILE") + `\.aligo\test-account.json`
	tdata, _ := os.ReadFile(tf)
	var tok struct {
		AccessToken    string `json:"access_token"`
		RefreshToken   string `json:"refresh_token"`
		DefaultDriveID string `json:"default_drive_id"`
		UserID         string `json:"user_id"`
		DeviceID       string `json:"x_device_id"`
	}
	json.Unmarshal(tdata, &tok)
	fmt.Printf("user_id=%s drive_id=%s device_id=%s\n", tok.UserID, tok.DefaultDriveID, tok.DeviceID)

	// 1. 生成密钥对。
	kp, err := device.GenerateKey()
	if err != nil {
		fmt.Println("生成密钥失败:", err)
		return
	}
	fmt.Printf("私钥: %s\n", kp.PrivateKeyHex())
	fmt.Printf("公钥: %s\n", kp.PublicKeyHex())

	// 2. create_session 激活设备。
	sessionDevID, err := createSession(ctx, tok.AccessToken, tok.UserID, kp.PublicKeyHex(), kp)
	if err != nil {
		fmt.Println("create_session 失败:", err)
		return
	}
	fmt.Printf("create_session 成功，deviceId=%s\n", sessionDevID)

	// 3. 用返回的 deviceId 生成签名。
	sig, err := kp.Sign(sessionDevID, tok.UserID)
	if err != nil {
		fmt.Println("签名失败:", err)
		return
	}
	fmt.Printf("x-signature=%s\n", sig[:40]+"...")

	// 4. 找一个文件用来测创建分享。
	fileID := findFile(ctx, tok.AccessToken, sessionDevID, sig, tok.UserID, tok.DefaultDriveID)
	if fileID == "" {
		fmt.Println("没找到可分享文件")
		return
	}
	fmt.Printf("测试文件: %s\n", fileID)

	// 5. 创建分享。
	shareURL, err := createShare(ctx, tok.AccessToken, sessionDevID, sig, fileID, tok.DefaultDriveID)
	if err != nil {
		fmt.Println("创建分享失败:", err)
		return
	}
	fmt.Printf("\n[成功] 分享链接: %s\n", shareURL)
}

func do(ctx context.Context, method, url string, body any, at, devID, sig string) ([]byte, int, error) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequestWithContext(ctx, method, url, r)
	req.Header.Set("Authorization", "Bearer "+at)
	req.Header.Set("User-Agent", webUA)
	req.Header.Set("Referer", "https://aliyundrive.com")
	req.Header.Set("x-canary", webCanary)
	req.Header.Set("x-device-id", devID)
	if sig != "" {
		req.Header.Set("x-signature", sig)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return data, resp.StatusCode, nil
}

func createSession(ctx context.Context, at, userID, pubKey string, kp *device.KeyPair) (string, error) {
	// create_session 需要本地生成的 x-device-id 和对应的 x-signature（用本地 device_id 算）。
	localDevID := newLocalDeviceID()
	sig, err := kp.Sign(localDevID, userID)
	if err != nil {
		return "", err
	}
	body := map[string]any{
		"deviceName": "alipan-go-test",
		"modelName":  "Windows",
		"nonce":      device.Nonce,
		"pubKey":     pubKey,
	}
	data, status, err := do(ctx, "POST", "https://api.aliyundrive.com/users/v1/users/device/create_session", body, at, localDevID, sig)
	if err != nil {
		return "", err
	}
	fmt.Printf("create_session resp status=%d body=%s\n", status, string(data))
	if status != 200 {
		return "", fmt.Errorf("status %d: %s", status, string(data))
	}
	var resp struct {
		DeviceID string `json:"device_id"`
	}
	json.Unmarshal(data, &resp)
	if resp.DeviceID == "" {
		// 服务端不返回新 device_id，复用本地生成的（已被服务端认可）。
		return localDevID, nil
	}
	return resp.DeviceID, nil
}

// newLocalDeviceID 生成 32 位无连字符 hex UUID。
func newLocalDeviceID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[:])
}

func findFile(ctx context.Context, at, devID, sig, userID, driveID string) string {
	body := map[string]any{
		"drive_id": driveID, "parent_file_id": "root", "limit": 50,
	}
	data, status, err := do(ctx, "POST", "https://api.aliyundrive.com/adrive/v3/file/list", body, at, devID, sig)
	if err != nil || status != 200 {
		fmt.Printf("list 失败 status=%d: %s\n", status, string(data))
		return ""
	}
	var resp struct {
		Items []struct {
			FileID string `json:"file_id"`
			Type   string `json:"type"`
			Name   string `json:"name"`
		} `json:"items"`
	}
	json.Unmarshal(data, &resp)
	for _, f := range resp.Items {
		if f.Type == "file" {
			return f.FileID
		}
	}
	// 根目录可能只有文件夹，进第一个文件夹找。
	for _, f := range resp.Items {
		if f.Type == "folder" {
			body2 := map[string]any{"drive_id": driveID, "parent_file_id": f.FileID, "limit": 50}
			d2, s2, _ := do(ctx, "POST", "https://api.aliyundrive.com/adrive/v3/file/list", body2, at, devID, sig)
			if s2 == 200 {
				var r2 struct {
					Items []struct {
						FileID string `json:"file_id"`
						Type   string `json:"type"`
					} `json:"items"`
				}
				json.Unmarshal(d2, &r2)
				for _, ff := range r2.Items {
					if ff.Type == "file" {
						return ff.FileID
					}
				}
			}
		}
	}
	return ""
}

func createShare(ctx context.Context, at, devID, sig, fileID, driveID string) (string, error) {
	body := map[string]any{
		"drive_id":    driveID,
		"file_id_list": []string{fileID},
		"share_pwd":   "1234",
		"expiration":  "",
	}
	data, status, err := do(ctx, "POST", "https://api.aliyundrive.com/adrive/v2/share_link/create", body, at, devID, sig)
	if err != nil {
		return "", err
	}
	fmt.Printf("create_share resp status=%d body=%s\n", status, string(data))
	if status != 200 {
		return "", fmt.Errorf("status %d: %s", status, string(data))
	}
	var resp struct {
		ShareURL string `json:"share_url"`
	}
	json.Unmarshal(data, &resp)
	return resp.ShareURL, nil
}
