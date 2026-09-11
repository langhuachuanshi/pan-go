package auth

import (
	"encoding/json"
	"os"
	"path/filepath"

	"crypto/rand"
	"encoding/hex"

	"github.com/langhuachuanshi/alipan-go/alipan/types"
)

// 本文件实现 token 持久化和 device id 生成，对应该 aligo Config.py 的配置目录逻辑。
// 配置文件兼容 aligo：~/.aligo/<name>.json，内容是 Token 的 JSON（含 x_device_id 扩展字段）。

func resolveConfigDir(configDir string) (string, error) {
	if configDir != "" {
		return configDir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".aligo"), nil
}

func tokenFilePath(name, configDir string) (string, error) {
	if name == "" {
		name = "aligo"
	}
	dir, err := resolveConfigDir(configDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".json"), nil
}

// loadToken 从本地配置文件读取 token。文件不存在返回 nil。
func loadToken(name, configDir string) *types.Token {
	path, err := tokenFilePath(name, configDir)
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var t types.Token
	if err := json.Unmarshal(data, &t); err != nil {
		return nil
	}
	return &t
}

// saveToken 把 token 写入配置文件。
func saveToken(t *types.Token, name, configDir string) {
	if t == nil {
		return
	}
	dir, err := resolveConfigDir(configDir)
	if err != nil {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	path := filepath.Join(dir, defaultName(name)+".json")
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

// DeleteTokenFile 删除配置文件（对应 aligo logout）。
func DeleteTokenFile(name, configDir string) error {
	path, err := tokenFilePath(name, configDir)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func defaultName(name string) string {
	if name == "" {
		return "aligo"
	}
	return name
}

// deviceID 返回 token 中持久化的 device id，没有则空串。
func deviceID(t *types.Token) string {
	if t != nil && t.XDeviceID != nil && *t.XDeviceID != "" {
		return *t.XDeviceID
	}
	return ""
}

// newDeviceID 生成 32 位无连字符 hex UUID（对齐 aligo uuid4().hex）。
func newDeviceID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[:])
}
