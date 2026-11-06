package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
)

// masterKeyPath 是主密钥文件的路径。主密钥只用于通过 KDF 派生子密钥，
// 不直接参与任何加解密。可用环境变量 LIANT_MASTER_KEY_FILE 覆盖。
var masterKeyPath = func() string {
	if p := os.Getenv("LIANT_MASTER_KEY_FILE"); p != "" {
		return p
	}
	return "data/key/master.key"
}()

// 各用途子密钥的 KDF 上下文标签。不同用途用不同标签派生出的密钥互不相同，
// 因此某个用途的密文无法用在别的用途上，实现密钥隔离。
const (
	subConfig  = "config"
	subToken   = "token"
	subSession = "session"
)

var (
	keyCacheMu sync.Mutex
	master     []byte
	subkeys    = map[string][]byte{}
)

// readOrCreateMasterKey 读取（缺失时生成）主密钥，文件权限 0600。
// 返回的密钥仅作为 HKDF 的输入密钥材料，绝不直接用于加解密。
func readOrCreateMasterKey() []byte {
	dir := filepath.Dir(masterKeyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		panic(err)
	}
	if _, err := os.Stat(masterKeyPath); os.IsNotExist(err) {
		key := make([]byte, chacha20poly1305.KeySize)
		if _, err := rand.Read(key); err != nil {
			panic(err)
		}
		if err := os.WriteFile(
			masterKeyPath,
			[]byte(base64.StdEncoding.EncodeToString(key)),
			0600,
		); err != nil {
			panic(err)
		}
	}
	data, err := os.ReadFile(masterKeyPath)
	if err != nil {
		panic(err)
	}
	key, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		panic(err)
	}
	if len(key) != chacha20poly1305.KeySize {
		panic("主密钥长度错误")
	}
	return key
}

// loadMasterLocked 返回缓存的主密钥，未缓存则从文件读取。调用方需持有 keyCacheMu。
func loadMasterLocked() []byte {
	if master == nil {
		master = readOrCreateMasterKey()
	}
	return master
}

// deriveSubkey 用 HKDF-SHA256 从主密钥派生出指定用途的子密钥，并缓存。
// 密钥隔离：不同 purpose 得到不同子密钥；即便密文被取出，也无法跨用途解密。
func deriveSubkey(purpose string) []byte {
	keyCacheMu.Lock()
	defer keyCacheMu.Unlock()
	if k, ok := subkeys[purpose]; ok {
		return k
	}
	ikm := loadMasterLocked()
	// salt 不传（nil），info 内嵌用途标签
	hk := hkdf.New(
		sha256.New,
		ikm,
		nil,
		[]byte("LianT/"+purpose),
	)
	key := make([]byte, chacha20poly1305.KeySize)
	if _, err := io.ReadFull(hk, key); err != nil {
		panic(err)
	}
	subkeys[purpose] = key
	return key
}

// ==========================
// 配置文件（config 子密钥）
// ==========================
func Encrypt(data []byte) []byte {
	return encryptWithKey(data, deriveSubkey(subConfig))
}

func Decrypt(data []byte) []byte {
	return decryptWithKey(data, deriveSubkey(subConfig))
}

// ==========================
// Bot token（token 子密钥）
// ==========================
func EncryptToken(token string) string {
	return base64.StdEncoding.EncodeToString(
		encryptWithKey([]byte(token), deriveSubkey(subToken)),
	)
}

func DecryptToken(token string) string {
	data, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		panic(err)
	}
	return string(decryptWithKey(data, deriveSubkey(subToken)))
}

// ==========================
// 会话（session 子密钥）
// ==========================
func EncryptSession(session string) string {
	return base64.StdEncoding.EncodeToString(
		encryptWithKey([]byte(session), deriveSubkey(subSession)),
	)
}

func DecryptSession(session string) string {
	data, err := base64.StdEncoding.DecodeString(session)
	if err != nil {
		panic(err)
	}
	return string(decryptWithKey(data, deriveSubkey(subSession)))
}

func encryptWithKey(data []byte, key []byte) []byte {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		panic(err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	// nonce + ciphertext + tag
	return aead.Seal(nonce, nonce, data, nil)
}

func decryptWithKey(data []byte, key []byte) []byte {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		panic(err)
	}
	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		panic("密文损坏")
	}
	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]
	result, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		panic("解密失败")
	}
	return result
}
