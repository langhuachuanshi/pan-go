// Package device 实现阿里云盘的设备会话签名，对应该社区逆向的 secp256k1 ECDSA 签名算法。
//
// 阿里云盘对分享等敏感接口要求动态签名（x-signature），固定常量无法通过。
// 签名机制：本地生成 secp256k1 密钥对 → 用公钥调 create_session 激活设备 →
// 之后该会话所有请求携带用私钥生成的 x-signature。
//
// 算法（来自社区逆向，RawChen/52pojie/kougami 多方印证）：
//   message = appId + ":" + deviceId + ":" + userId + ":" + nonce
//   digest  = SHA256(message)
//   sig     = ECDSA_Sign(privateKey, digest)   // secp256k1
//   x-signature = hex(sig.r, 32字节) + hex(sig.s, 32字节) + "01"
package device

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

// AppID 阿里云盘 Web/PC 客户端的固定 appId（社区逆向值）。
const AppID = "5dde4e1bdf9e4966b387ba58f4b3fdc3"

// Nonce 固定 nonce（社区实现用 "0"）。
const Nonce = "0"

// KeyPair secp256k1 密钥对。
type KeyPair struct {
	PrivKey *secp256k1.PrivateKey
	PubKey  *secp256k1.PublicKey
}

// GenerateKey 生成新的 secp256k1 密钥对。
func GenerateKey() (*KeyPair, error) {
	priv, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("device: generate key failed: %w", err)
	}
	return &KeyPair{PrivKey: priv, PubKey: priv.PubKey()}, nil
}

// PrivateKeyHex 返回私钥的 32 字节 hex。
func (kp *KeyPair) PrivateKeyHex() string {
	privBytes := kp.PrivKey.Serialize()
	return hex.EncodeToString(privBytes)
}

// PublicKeyHex 返回未压缩公钥（65字节，04前缀）的 hex。
// 这是提交给 create_session 的 pubKey 格式。
func (kp *KeyPair) PublicKeyHex() string {
	pubBytes := kp.PubKey.SerializeUncompressed()
	return hex.EncodeToString(pubBytes)
}

// PrivateKeyFromHex 从 hex 还原私钥。
func PrivateKeyFromHex(hexKey string) (*KeyPair, error) {
	b, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("device: decode privkey hex failed: %w", err)
	}
	priv := secp256k1.PrivKeyFromBytes(b)
	return &KeyPair{PrivKey: priv, PubKey: priv.PubKey()}, nil
}

// Sign 生成 x-signature。
//
// message = appId:deviceId:userId:nonce
// 返回 hex(r) + hex(s) + "01"（r/s 各 32 字节定长，共 128 hex 字符 + "01"）。
//
// 注意：不能用 Signature.Serialize()（那是 DER 格式，带 30440220 前缀），
// 必须用 R()/S() 各取 32 字节裸拼接。
func (kp *KeyPair) Sign(deviceID, userID string) (string, error) {
	message := AppID + ":" + deviceID + ":" + userID + ":" + Nonce
	digest := sha256.Sum256([]byte(message))
	sig := ecdsa.Sign(kp.PrivKey, digest[:])

	// R()/S() 返回 ModNScalar（值类型），Bytes() 是指针接收者，需取地址。
	r := sig.R()
	s := sig.S()
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	rs := make([]byte, 0, 64)
	rs = append(rs, rBytes[:]...)
	rs = append(rs, sBytes[:]...)
	return hex.EncodeToString(rs) + "01", nil
}
