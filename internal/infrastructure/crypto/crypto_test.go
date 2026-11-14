package crypto

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"dbdlp/internal/domain/port"
	"github.com/emmansun/gmsm/sm4"
)

func TestSM4OfficialVector(t *testing.T) {
	// GB/T 32907 官方测试向量：验证底层 SM4 分块正确性
	key := []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF, 0xFE, 0xDC, 0xBA, 0x98, 0x76, 0x54, 0x32, 0x10}
	plain := []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF, 0xFE, 0xDC, 0xBA, 0x98, 0x76, 0x54, 0x32, 0x10}
	want := []byte{0x68, 0x1E, 0xDF, 0x34, 0xD2, 0x06, 0x96, 0x5E, 0x86, 0xB3, 0xE9, 0x4F, 0x53, 0x6E, 0x42, 0x46}
	c, err := sm4.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	dst := make([]byte, 16)
	c.Encrypt(dst, plain)
	if !bytes.Equal(dst, want) {
		t.Fatalf("SM4 加密向量不匹配：\n got %x\nwant %x", dst, want)
	}
	back := make([]byte, 16)
	c.Decrypt(back, dst)
	if !bytes.Equal(back, plain) {
		t.Fatalf("SM4 解密回环失败")
	}
}

func TestAES256GCMRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	c, err := NewCipher(key, port.AlgAES256GCM)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := c.Encrypt("敏感密码123", port.AlgAES256GCM)
	if err != nil {
		t.Fatal(err)
	}
	if !c.IsEncrypted(enc) {
		t.Fatal("应识别为 ENC() 密文")
	}
	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatal(err)
	}
	if dec != "敏感密码123" {
		t.Fatalf("解密结果错误：%q", dec)
	}
	// 明文原样返回
	if p, _ := c.Decrypt("明文"); p != "明文" {
		t.Fatalf("非 ENC 应原样返回")
	}
}

func TestSM4GCMRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(255 - i)
	}
	c, _ := NewCipher(key, port.AlgAES256GCM)
	enc, err := c.Encrypt("192.168.10.3", port.AlgSM4GCM)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatal(err)
	}
	if dec != "192.168.10.3" {
		t.Fatalf("SM4-GCM 解密错误：%q", dec)
	}
}

func TestCipherWrongKeyFails(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 1
	c1, _ := NewCipher(key1, port.AlgAES256GCM)
	c2, _ := NewCipher(key2, port.AlgAES256GCM)
	enc, _ := c1.Encrypt("secret", port.AlgAES256GCM)
	if _, err := c2.Decrypt(enc); err == nil {
		t.Fatal("错误密钥应解密失败")
	}
}

func TestKeyStoreGenerateAndReuse(t *testing.T) {
	dir := t.TempDir()
	ks := NewKeyStore(dir)
	k1, err := ks.LoadOrInit()
	if err != nil {
		t.Fatal(err)
	}
	if len(k1) != 32 {
		t.Fatalf("密钥长度应为 32，got %d", len(k1))
	}
	p := ks.Path()
	if got, _ := os.Stat(p); got == nil {
		t.Fatal("密钥文件应已生成")
	}
	// 二次调用返回同一密钥
	k2, err := ks.LoadOrInit()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(k1, k2) {
		t.Fatal("二次加载应返回同一密钥")
	}
	// 损坏文件应报错
	bad := filepath.Join(dir, ".dbdlp", "key")
	_ = os.WriteFile(bad, []byte("short"), 0o600)
	if _, err := ks.LoadOrInit(); err == nil {
		t.Fatal("损坏密钥应报错")
	}
}
