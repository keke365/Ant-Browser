package siteaccount

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const encryptedValuePrefix = "enc:v1:"

type secretKeeper struct {
	keyPath string
	mu      sync.Mutex
	key     []byte
}

func newSecretKeeper(keyPath string) *secretKeeper {
	return &secretKeeper{keyPath: strings.TrimSpace(keyPath)}
}

func (s *secretKeeper) encrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	key, err := s.loadKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("生成敏感字段随机数失败: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return encryptedValuePrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

func (s *secretKeeper) decrypt(stored string) (string, error) {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return "", nil
	}
	if !strings.HasPrefix(stored, encryptedValuePrefix) {
		return stored, nil
	}
	key, err := s.loadKey()
	if err != nil {
		return "", err
	}
	payloadText := strings.TrimPrefix(stored, encryptedValuePrefix)
	payload, err := base64.StdEncoding.DecodeString(payloadText)
	if err != nil {
		return "", fmt.Errorf("敏感字段解码失败: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) < gcm.NonceSize() {
		return "", fmt.Errorf("敏感字段密文无效")
	}
	nonce := payload[:gcm.NonceSize()]
	ciphertext := payload[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("敏感字段解密失败: %w", err)
	}
	return string(plain), nil
}

func (s *secretKeeper) loadKey() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.key) == 32 {
		return append([]byte{}, s.key...), nil
	}
	if s.keyPath == "" {
		return nil, fmt.Errorf("敏感字段密钥路径为空")
	}
	if data, err := os.ReadFile(s.keyPath); err == nil {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, fmt.Errorf("读取敏感字段密钥失败: %w", err)
		}
		if len(decoded) != 32 {
			return nil, fmt.Errorf("敏感字段密钥长度无效")
		}
		s.key = decoded
		return append([]byte{}, s.key...), nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("读取敏感字段密钥失败: %w", err)
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("生成敏感字段密钥失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.keyPath), 0o700); err != nil {
		return nil, fmt.Errorf("创建敏感字段密钥目录失败: %w", err)
	}
	content := []byte(base64.StdEncoding.EncodeToString(key))
	if err := os.WriteFile(s.keyPath, content, 0o600); err != nil {
		return nil, fmt.Errorf("保存敏感字段密钥失败: %w", err)
	}
	s.key = key
	return append([]byte{}, s.key...), nil
}
