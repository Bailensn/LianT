// Package secret 负责 LianT 的本地敏感字段加密。
//
// 密钥优先级：
//  1. 环境变量 LIANT_SECRET（任意口令，内部 SHA-256 成 32 字节）
//  2. <数据目录>/secret.key（首次启动生成 32 字节随机密钥，权限 0600）
//
// 加密算法：AES-256-GCM，随机 nonce，密文格式 enc:v1:<base64>。
// 数据库被拿走但 secret.key 没被拿走时，bot token / admin token 无法解密。
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"LianT/internal/config"
)

const Prefix = "enc:v1:"

type Box struct {
	aead cipher.AEAD
}

var (
	mu    sync.Mutex
	boxes = map[string]*Box{}
)

// Default 兼容旧调用：按默认注册表库定位密钥。
func Default() (*Box, error) { return ForDB(config.DBPath) }

// KeyPathForDB 让密钥跟随数据库目录，而不是当前工作目录。
// 中央库 <data>/LianT.db 与业务库 <data>/bots/<id>.db 共用 <data>/secret.key。
func KeyPathForDB(dbPath string) string {
	dir := filepath.Dir(dbPath)
	if filepath.Base(dir) == "bots" {
		dir = filepath.Dir(dir)
	}
	return filepath.Join(dir, "secret.key")
}

// KeyPath 兼容旧调用。
func KeyPath() string { return KeyPathForDB(config.DBPath) }

// ForDB 返回某个数据库对应的加密器。同一个密钥文件复用同一个 Box。
func ForDB(dbPath string) (*Box, error) {
	if raw := strings.TrimSpace(config.EnvOr("LIANT_SECRET", "")); raw != "" {
		sum := sha256.Sum256([]byte(raw))
		key := sum[:]
		mu.Lock()
		defer mu.Unlock()
		if b, ok := boxes["env"]; ok {
			return b, nil
		}
		b, err := newBox(key)
		if err != nil {
			return nil, err
		}
		boxes["env"] = b
		return b, nil
	}
	kp := KeyPathForDB(dbPath)
	mu.Lock()
	defer mu.Unlock()
	if b, ok := boxes[kp]; ok {
		return b, nil
	}
	b, err := load(kp)
	if err != nil {
		return nil, err
	}
	boxes[kp] = b
	return b, nil
}

func load(keyPath string) (*Box, error) {
	var key []byte
	p := keyPath
	b, err := os.ReadFile(p)
	switch {
	case err == nil && len(b) == 32:
		key = b
	case err == nil:
		return nil, fmt.Errorf("密钥文件 %s 长度不对（应为 32 字节）", p)
	case os.IsNotExist(err):
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("生成密钥失败: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return nil, fmt.Errorf("创建密钥目录失败: %w", err)
		}
		if err := os.WriteFile(p, key, 0o600); err != nil {
			return nil, fmt.Errorf("写密钥文件失败 %s: %w", p, err)
		}
		_ = os.Chmod(p, 0o600)
		log.Printf("[OK] 已生成本地加密密钥: %s（务必备份，丢了无法解密已保存的 token）", p)
	default:
		return nil, fmt.Errorf("读密钥文件失败 %s: %w", p, err)
	}
	return newBox(key)
}

func newBox(key []byte) (*Box, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

func IsEncrypted(v string) bool { return strings.HasPrefix(v, Prefix) }

func (b *Box) Encrypt(plain string) (string, error) {
	if plain == "" || IsEncrypted(plain) {
		return plain, nil
	}
	if b == nil || b.aead == nil {
		return "", fmt.Errorf("加密器未初始化")
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := b.aead.Seal(nonce, nonce, []byte(plain), nil)
	return Prefix + base64.RawStdEncoding.EncodeToString(out), nil
}

func (b *Box) Decrypt(value string) (string, error) {
	if !IsEncrypted(value) {
		return value, nil
	}
	if b == nil || b.aead == nil {
		return "", fmt.Errorf("加密器未初始化")
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, Prefix))
	if err != nil {
		return "", fmt.Errorf("密文格式错误: %w", err)
	}
	ns := b.aead.NonceSize()
	if len(raw) < ns {
		return "", fmt.Errorf("密文长度不足")
	}
	plain, err := b.aead.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("解密失败（secret.key 不匹配或数据损坏）: %w", err)
	}
	return string(plain), nil
}
