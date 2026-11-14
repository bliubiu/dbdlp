package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"dbdlp/internal/domain/port"
)

// keyStoreImpl 实现 port.KeyStore：首次生成 32 字节密钥，0600 写入 .dbdlp/key。
type keyStoreImpl struct {
	baseDir string // 配置目录，密钥文件位于 baseDir/.dbdlp/key
}

// NewKeyStore 以配置目录构造密钥存储。
func NewKeyStore(baseDir string) port.KeyStore {
	return &keyStoreImpl{baseDir: baseDir}
}

// Path 返回密钥文件绝对路径。
func (k *keyStoreImpl) Path() string {
	return filepath.Join(k.baseDir, ".dbdlp", "key")
}

// LoadOrInit 读取既有密钥；不存在则生成 32 字节并 0600 写入。任何失败返回错误。
func (k *keyStoreImpl) LoadOrInit() ([]byte, error) {
	p := k.Path()
	if data, err := os.ReadFile(p); err == nil {
		if len(data) != 32 {
			return nil, fmt.Errorf("密钥文件长度错误（应为 32 字节）：%s", p)
		}
		return data, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("读取密钥失败：%w", err)
	}
	// 生成新密钥
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("生成密钥失败：%w", err)
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("创建密钥目录失败：%w", err)
	}
	if err := os.WriteFile(p, key, 0o600); err != nil {
		return nil, fmt.Errorf("写入密钥失败：%w", err)
	}
	// 再次确保权限（Windows 上 chmod 尽力而为）
	_ = os.Chmod(p, 0o600)
	return key, nil
}

// errUnavailable 密钥不可用的统一错误（对应错误码 2003）。
var errUnavailable = errors.New("密钥不可用")
