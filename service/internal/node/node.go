// Package node 管理 Service（节点）自己的身份：ECDSA P-256 私钥、自签证书与指纹。
//
// 指纹是客户端钉住的唯一信任锚（docs/PROTOCOL.md §2/§10）：证书由私钥签发，
// 客户端不校验域名，只比对指纹。私钥只用于 TLS 与挑战签名，不出 node.key。
package node

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base32"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	keyFile  = "node.key"
	certFile = "node.crt"

	// 证书有效期 1 年；剩余不足 30 天时自动换新（§10 第 9 条）。
	certValidity = 365 * 24 * time.Hour
	renewBefore  = 30 * 24 * time.Hour
)

var (
	// ErrNoKey 表示私钥文件缺失或损坏。
	ErrNoKey = errors.New("node: 节点私钥不可用")
	// ErrBadSignature 表示签名校验失败。
	ErrBadSignature = errors.New("node: 签名校验失败")
)

// Identity 是一个已加载的节点身份。
type Identity struct {
	dir  string
	key  *ecdsa.PrivateKey
	cert tls.Certificate
	fp   string
}

// LoadOrCreate 读取（缺失时生成）节点身份。
//
// 私钥文件权限 0600；证书缺失或临近过期时用同一私钥重新签发。
func LoadOrCreate(dir string) (*Identity, error) {
	if dir == "" {
		return nil, fmt.Errorf("%w: 目录为空", ErrNoKey)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("node: 创建目录: %w", err)
	}

	key, err := loadOrCreateKey(filepath.Join(dir, keyFile))
	if err != nil {
		return nil, err
	}
	cert, err := loadOrCreateCert(filepath.Join(dir, certFile), key)
	if err != nil {
		return nil, err
	}
	return &Identity{
		dir:  dir,
		key:  key,
		cert: cert,
		fp:   fingerprint(&key.PublicKey),
	}, nil
}

// Fingerprint 返回节点指纹，形如 ABCD-EFGH-…。
func (i *Identity) Fingerprint() string { return i.fp }

// Certificate 返回 TLS 证书。
func (i *Identity) Certificate() tls.Certificate { return i.cert }

// PublicKey 返回公钥，供测试与诊断使用。
func (i *Identity) PublicKey() *ecdsa.PublicKey { return &i.key.PublicKey }

// Sign 用节点私钥对 msg 做 ECDSA-SHA256 签名（ASN.1 编码）。
func (i *Identity) Sign(msg []byte) ([]byte, error) {
	sum := sha256.Sum256(msg)
	sig, err := ecdsa.SignASN1(rand.Reader, i.key, sum[:])
	if err != nil {
		return nil, fmt.Errorf("node: 签名: %w", err)
	}
	return sig, nil
}

// Rotate 立即换一对新密钥并重签证书。
//
// 客户端钉的是指纹，轮换后需要重新配对——这是有意为之的安全边界。
func (i *Identity) Rotate() error {
	key, err := generateKey()
	if err != nil {
		return err
	}
	if err := writeKey(filepath.Join(i.dir, keyFile), key); err != nil {
		return err
	}
	cert, err := issueCert(filepath.Join(i.dir, certFile), key)
	if err != nil {
		return err
	}
	i.key = key
	i.cert = cert
	i.fp = fingerprint(&key.PublicKey)
	return nil
}

// Verify 校验 pub 对 msg 的签名。
func Verify(pub *ecdsa.PublicKey, msg, sig []byte) error {
	if pub == nil {
		return fmt.Errorf("%w: 空公钥", ErrBadSignature)
	}
	sum := sha256.Sum256(msg)
	if !ecdsa.VerifyASN1(pub, sum[:], sig) {
		return ErrBadSignature
	}
	return nil
}

// AuthMessage 是认证签名覆盖的字节：nonce、服务端指纹与 ts。
//
// 编码固定为 nonce + "\n" + fingerprint + "\n" + 十进制 ts，
// 避免拼接歧义（docs/PROTOCOL.md §4 的 nonce||fp||ts）。
func AuthMessage(nonce, fingerprint string, ts int64) []byte {
	return []byte(nonce + "\n" + fingerprint + "\n" + strconv.FormatInt(ts, 10))
}

// PinnedFingerprint 返回 base32(SHA-256(DER 公钥))，每 4 位一组。
func PinnedFingerprint(pub *ecdsa.PublicKey) string { return fingerprint(pub) }

func fingerprint(pub *ecdsa.PublicKey) string {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		// 公钥来自本进程生成或已解析的 key，序列化失败说明代码有 bug。
		panic("node: 序列化公钥: " + err.Error())
	}
	sum := sha256.Sum256(der)
	enc := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:])
	groups := make([]string, 0, len(enc)/4+1)
	for len(enc) > 4 {
		groups = append(groups, enc[:4])
		enc = enc[4:]
	}
	groups = append(groups, enc)
	return strings.Join(groups, "-")
}

func generateKey() (*ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("node: 生成密钥: %w", err)
	}
	return key, nil
}

func writeKey(path string, key *ecdsa.PrivateKey) error {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("node: 编码私钥: %w", err)
	}
	block := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(path, block, 0o600); err != nil {
		return fmt.Errorf("node: 写入私钥: %w", err)
	}
	return nil
}

func loadOrCreateKey(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		key, err := generateKey()
		if err != nil {
			return nil, err
		}
		if err := writeKey(path, key); err != nil {
			return nil, err
		}
		return key, nil
	}
	if err != nil {
		return nil, fmt.Errorf("node: 读取私钥: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "EC PRIVATE KEY" {
		return nil, fmt.Errorf("%w: 私钥文件格式不对", ErrNoKey)
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoKey, err)
	}
	if key.Curve != elliptic.P256() {
		return nil, fmt.Errorf("%w: 私钥不是 P-256", ErrNoKey)
	}
	return key, nil
}

func loadOrCreateCert(path string, key *ecdsa.PrivateKey) (tls.Certificate, error) {
	if data, err := os.ReadFile(path); err == nil {
		if cert, ok := parseCert(data, key); ok {
			if leaf, err := x509.ParseCertificate(cert.Certificate[0]); err == nil {
				if time.Now().Add(renewBefore).Before(leaf.NotAfter) {
					return cert, nil
				}
			}
			// 临近过期或解析失败：继续往下走，重签。
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return tls.Certificate{}, fmt.Errorf("node: 读取证书: %w", err)
	}
	return issueCert(path, key)
}

func parseCert(data []byte, key *ecdsa.PrivateKey) (tls.Certificate, bool) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return tls.Certificate{}, false
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return tls.Certificate{}, false
	}
	pub, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok || !pub.Equal(&key.PublicKey) {
		// 证书与私钥不配对（例如手动换了 key），必须重签。
		return tls.Certificate{}, false
	}
	return tls.Certificate{
		Certificate: [][]byte{block.Bytes},
		PrivateKey:  key,
		Leaf:        leaf,
	}, true
}

func issueCert(path string, key *ecdsa.PrivateKey) (tls.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("node: 生成序列号: %w", err)
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "LianT Service", Organization: []string{"LianT"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(certValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{"liant"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("node: 签发证书: %w", err)
	}
	block := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(path, block, 0o600); err != nil {
		return tls.Certificate{}, fmt.Errorf("node: 写入证书: %w", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("node: 解析新证书: %w", err)
	}
	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  key,
		Leaf:        leaf,
	}, nil
}

// ParsePublicKey 解析 Manager 以 base64 提交的公钥（PKIX DER）。
func ParsePublicKey(b64 string) (*ecdsa.PublicKey, error) {
	der, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("node: 公钥不是合法 base64: %w", err)
	}
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("node: 解析公钥: %w", err)
	}
	ec, ok := pub.(*ecdsa.PublicKey)
	if !ok || ec.Curve != elliptic.P256() {
		return nil, fmt.Errorf("node: 公钥不是 P-256")
	}
	return ec, nil
}
