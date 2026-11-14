package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"dbdlp/internal/domain/port"
	"github.com/emmansun/gmsm/sm4"
)

const encPrefix = "ENC("

// cipherImpl 实现 port.Cipher：AES-256-GCM（0x00）+ SM4-GCM（0x01）。
type cipherImpl struct {
	key        []byte
	defaultAlg port.Algorithm
}

// NewCipher 用 32 字节主密钥构造加解密器；defaultAlg 为默认加密算法。
func NewCipher(key []byte, defaultAlg port.Algorithm) (port.Cipher, error) {
	if len(key) != 32 {
		return nil, errors.New("密钥长度必须为 32 字节")
	}
	return &cipherImpl{key: key, defaultAlg: defaultAlg}, nil
}

// Encrypt 生成 ENC(base64(载荷))；载荷 = [标签1B][nonce12B][密文][tag16B]。
func (c *cipherImpl) Encrypt(plaintext string, alg port.Algorithm) (string, error) {
	if alg == 0 && c.defaultAlg != 0 {
		alg = c.defaultAlg
	}
	aead, keyLen, err := c.aead(alg)
	if err != nil {
		return "", err
	}
	_ = keyLen
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("生成随机 nonce 失败：%w", err)
	}
	ct := aead.Seal(nil, nonce, []byte(plaintext), nil)
	payload := make([]byte, 0, 1+12+len(ct))
	payload = append(payload, byte(alg))
	payload = append(payload, nonce...)
	payload = append(payload, ct...)
	return encPrefix + base64.StdEncoding.EncodeToString(payload) + ")", nil
}

// Decrypt 自动识别 ENC() 包裹与算法标签；非 ENC() 按明文返回。
func (c *cipherImpl) Decrypt(stored string) (string, error) {
	if !strings.HasPrefix(stored, encPrefix) {
		return stored, nil // 明文
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(stored, encPrefix), ")")
	payload, err := base64.StdEncoding.DecodeString(inner)
	if err != nil {
		return "", fmt.Errorf("ENC 密文 base64 解码失败：%w", err)
	}
	if len(payload) < 29 {
		return "", errors.New("ENC 密文载荷过短，格式非法")
	}
	alg := port.Algorithm(payload[0])
	aead, _, err := c.aead(alg)
	if err != nil {
		return "", err
	}
	nonce := payload[1:13]
	ct := payload[13:]
	pt, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("密文解密校验失败（密钥错误或数据损坏）：%w", err)
	}
	return string(pt), nil
}

// IsEncrypted 判断字符串是否为 ENC() 包裹。
func (c *cipherImpl) IsEncrypted(s string) bool {
	return strings.HasPrefix(s, encPrefix)
}

// aead 按算法标签返回 AEAD；0x00 用完整 32 字节密钥，0x01 用前 16 字节。
func (c *cipherImpl) aead(alg port.Algorithm) (cipher.AEAD, int, error) {
	switch alg {
	case port.AlgAES256GCM:
		block, err := aes.NewCipher(c.key)
		if err != nil {
			return nil, 0, err
		}
		a, err := cipher.NewGCM(block)
		return a, 32, err
	case port.AlgSM4GCM:
		block, err := sm4.NewCipher(c.key[:16])
		if err != nil {
			return nil, 0, err
		}
		a, err := cipher.NewGCM(block)
		return a, 16, err
	default:
		return nil, 0, fmt.Errorf("未知加密算法标签：0x%02x", byte(alg))
	}
}
