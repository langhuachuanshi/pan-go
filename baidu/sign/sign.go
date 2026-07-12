// Package sign 提供百度网盘网页端各接口所需的签名算法。
//
// 包含：
//   - Sign2：标准 RC4 流密码，用于 PanHome 签名（/api/download）
//   - LocateDownloadSign：SHA1 签名，用于 PCS locatedownload
//   - DevUID：BDUSS 的设备指纹
//   - ShareSURLInfoSign：分享短链信息查询的 MD5 签名
//
// 算法来源：BaiduPCS-Go（经实测验证，与百度网页端行为一致）。
package sign

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"time"
)

// Sign2 标准 RC4 流密码。
// j 是密钥（对应 panhome sign3），r 是明文（对应 panhome sign1）。
// 返回 XOR 后的原始字节（调用方负责后续编码，如 Base64）。
func Sign2(j, r []rune) []byte {
	var (
		a  = make([]rune, 256)
		p  = make([]rune, 256)
		o  = make([]byte, len(r))
		v  = len(j)
		q  int
		u  rune
		i  int
		k  rune
		dr int
	)
	if v == 0 {
		return o
	}
	for ; q < 256; q++ {
		dr = q % v
		a[q] = j[dr : 1+dr][0]
		p[q] = rune(q)
	}
	for q = 0; q < 256; q++ {
		u = (u + p[q] + a[q]) % 256
		p[q], p[u] = p[u], p[q]
	}
	u = 0
	for q = 0; q < len(r); q++ {
		i = (i + 1) % 256
		u = (u + p[i]) % 256
		p[i], p[u] = p[u], p[i]
		k = p[(p[i]+p[u])%256]
		o[q] = byte(r[q] ^ k)
	}
	return o
}

// LocateDownloadSign PCS locatedownload 签名结果。
type LocateDownloadSign struct {
	Time   int64  // Unix 时间戳
	Rand   string // SHA1 签名 hex
	DevUID string // 设备指纹
}

// NewLocateDownloadSign 创建 LocateDownload 签名。
// uid 是百度用户 ID，bduss 是登录凭证。
func NewLocateDownloadSign(uid uint64, bduss string) *LocateDownloadSign {
	return newLocateDownloadSignWithTime(time.Now().Unix(), DevUID(bduss), uid, bduss)
}

func newLocateDownloadSignWithTime(timeUnix int64, devuid string, uid uint64, bduss string) *LocateDownloadSign {
	s := &LocateDownloadSign{
		Time:   timeUnix,
		DevUID: devuid,
	}
	s.sign(uid, bduss)
	return s
}

// locateDownloadKey 硬编码的 32 字节签名密钥（来自百度网页端 JS）。
var locateDownloadKey = []byte{
	'\x65', '\x62', '\x72', '\x63', '\x55', '\x59', '\x69', '\x75',
	'\x78', '\x61', '\x5a', '\x76', '\x32', '\x58', '\x47', '\x75',
	'\x37', '\x4b', '\x49', '\x59', '\x4b', '\x78', '\x55', '\x72',
	'\x71', '\x66', '\x6e', '\x4f', '\x66', '\x70', '\x44', '\x46',
}

func (s *LocateDownloadSign) sign(uid uint64, bduss string) {
	bdussSha1 := sha1.Sum([]byte(bduss))
	bdussHex := make([]byte, hex.EncodedLen(len(bdussSha1)))
	hex.Encode(bdussHex, bdussSha1[:])

	h := sha1.New()
	h.Write(bdussHex)
	h.Write([]byte(strconv.FormatUint(uid, 10)))
	h.Write(locateDownloadKey)
	h.Write([]byte(strconv.FormatInt(s.Time, 10)))
	h.Write([]byte(s.DevUID))

	randHex := make([]byte, hex.EncodedLen(sha1.Size))
	hex.Encode(randHex, h.Sum(nil))
	s.Rand = string(randHex)
}

// URLParam 返回 locate download 的 URL query 参数串。
// 格式: time=...&rand=...&devuid=...&cuid=...
func (s *LocateDownloadSign) URLParam() string {
	return "time=" + strconv.FormatInt(s.Time, 10) +
		"&rand=" + s.Rand +
		"&devuid=" + s.DevUID +
		"&cuid=" + s.DevUID
}

// DevUID 从 BDUSS 计算设备指纹。
// 算法: MD5(bduss) -> 大写 hex，后缀 |0。
func DevUID(bduss string) string {
	h := md5.Sum([]byte(bduss))
	hexStr := make([]byte, 32)
	hex.Encode(hexStr, h[:])
	// 转大写
	for i, b := range hexStr {
		if b >= 'a' && b <= 'f' {
			hexStr[i] = b - 32
		}
	}
	return string(hexStr) + "|0"
}

// ShareSURLInfoSign 分享短链信息查询的签名。
// sign = md5(shareID + "_sharesurlinfo!@#") 的 hex 编码。
func ShareSURLInfoSign(shareID int64) string {
	s := strconv.FormatInt(shareID, 10)
	h := md5.Sum([]byte(s + "_sharesurlinfo!@#"))
	return hex.EncodeToString(h[:])
}
