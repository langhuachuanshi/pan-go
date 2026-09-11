package file

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// 本文件实现上传所需的哈希计算，对应该 aligo core/Create.py 的 _get_proof_code。

// computeProofCode 计算 proof_code：
//   md5_int = int(md5(access_token).hexdigest()[:16], 16)
//   offset  = md5_int % file_size   (file_size==0 时 offset=0)
//   读 [offset, offset+8) 共 8 字节，base64 编码。
func computeProofCode(accessToken, filePath string, fileSize int64) (string, error) {
	if fileSize == 0 {
		return "", nil
	}
	h := md5.Sum([]byte(accessToken))
	md5Hex := hex.EncodeToString(h[:])[:16]
	md5Int, err := parseHexUint64(md5Hex)
	if err != nil {
		return "", err
	}
	offset := md5Int % uint64(fileSize)

	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Seek(int64(offset), io.SeekStart); err != nil {
		return "", err
	}
	buf := make([]byte, 8)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf[:n]), nil
}

func parseHexUint64(s string) (uint64, error) {
	var v uint64
	for i := 0; i < len(s); i++ {
		c := s[i]
		var d uint64
		switch {
		case c >= '0' && c <= '9':
			d = uint64(c - '0')
		case c >= 'a' && c <= 'f':
			d = uint64(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = uint64(c-'A') + 10
		default:
			return 0, fmt.Errorf("alipan: invalid hex char %q", c)
		}
		v = v<<4 | d
	}
	return v, nil
}

// computeSHA1Upper 计算文件整内容 SHA1（大写 hex），同时返回前 1024 字节的 pre_hash。
func computeSHA1Upper(filePath string) (contentHash, preHash string, err error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	h := sha1.New()
	preH := sha1.New()
	preBuf := make([]byte, 1024)
	n, err := io.ReadFull(f, preBuf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", "", err
	}
	if n > 0 {
		preH.Write(preBuf[:n])
		h.Write(preBuf[:n])
	}
	if _, err := io.Copy(h, f); err != nil {
		return "", "", err
	}
	contentHash = fmt.Sprintf("%X", h.Sum(nil))
	if n > 0 {
		preHash = fmt.Sprintf("%X", preH.Sum(nil))
	}
	return contentHash, preHash, nil
}

// chunkSize 根据文件大小决定分片大小：< ~100GB→10MB，否则 256MB。
func chunkSize(fileSize int64) int64 {
	const (
		chunk10MB  int64 = 10 * 1024 * 1024
		chunk256MB int64 = 256 * 1024 * 1024
		threshold        = 100 * 1024 * 1024 * 1024
	)
	if fileSize < threshold {
		return chunk10MB
	}
	return chunk256MB
}
