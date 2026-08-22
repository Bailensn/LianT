package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"

	"golang.org/x/crypto/chacha20poly1305"
)

const keyPath = "data/master.key"

// ==========================
// 获取Key
// ==========================
func GetKey() []byte {
	dir := filepath.Dir(
		keyPath,
	)
	err := os.MkdirAll(
		dir,
		0700,
	)
	if err != nil {
		panic(err)
	}
	_, err = os.Stat(
		keyPath,
	)
	// 不存在则生成
	if os.IsNotExist(err) {
		key := make(
			[]byte,
			chacha20poly1305.KeySize,
		)
		_, err := rand.Read(
			key,
		)
		if err != nil {
			panic(err)
		}
		err = os.WriteFile(
			keyPath,
			[]byte(
				base64.StdEncoding.EncodeToString(key),
			),
			0600,
		)
		if err != nil {
			panic(err)
		}
	}
	data, err := os.ReadFile(
		keyPath,
	)
	if err != nil {
		panic(err)
	}
	key, err := base64.StdEncoding.DecodeString(
		string(data),
	)
	if err != nil {
		panic(err)
	}
	if len(key) != chacha20poly1305.KeySize {
		panic(
			"master.key长度错误",
		)
	}
	return key
}

// ==========================
// 加密
// ==========================
func Encrypt(
	data []byte,
) []byte {
	aead, err := chacha20poly1305.New(
		GetKey(),
	)
	if err != nil {
		panic(err)
	}
	nonce := make(
		[]byte,
		aead.NonceSize(),
	)
	_, err = rand.Read(
		nonce,
	)
	if err != nil {
		panic(err)
	}
	// nonce + ciphertext + tag
	return aead.Seal(
		nonce,
		nonce,
		data,
		nil,
	)
}

// ==========================
// 解密
// ==========================
func Decrypt(
	data []byte,
) []byte {
	aead, err := chacha20poly1305.New(
		GetKey(),
	)
	if err != nil {
		panic(err)
	}
	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		panic(
			"密文损坏",
		)
	}
	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]
	result, err := aead.Open(
		nil,
		nonce,
		ciphertext,
		nil,
	)
	if err != nil {
		panic(
			"配置解密失败",
		)
	}
	return result
}

// ==========================
// Token加密
// ==========================
func EncryptToken(
	token string,
) string {
	return base64.StdEncoding.EncodeToString(
		Encrypt(
			[]byte(token),
		),
	)
}

// ==========================
// Token解密
// ==========================
func DecryptToken(
	token string,
) string {
	data, err := base64.StdEncoding.DecodeString(
		token,
	)
	if err != nil {
		panic(err)
	}
	return string(
		Decrypt(
			data,
		),
	)
}