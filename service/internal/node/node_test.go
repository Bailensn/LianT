package node_test

import (
	"crypto/x509"
	"encoding/base64"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/Bailensn/LianT/service/internal/node"
)

func TestLoadOrCreateIsStable(t *testing.T) {
	dir := t.TempDir()
	first, err := node.LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("首次加载: %v", err)
	}
	second, err := node.LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("二次加载: %v", err)
	}
	if first.Fingerprint() != second.Fingerprint() {
		t.Fatalf("指纹不稳定: %s != %s", first.Fingerprint(), second.Fingerprint())
	}

	info, err := os.Stat(dir + "/node.key")
	if err != nil {
		t.Fatalf("私钥文件: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("私钥权限应为 0600，得到 %o", perm)
	}

	leaf := first.Certificate().Leaf
	if leaf == nil {
		t.Fatal("证书缺少 Leaf")
	}
	if leaf.NotAfter.Before(time.Now().Add(300 * 24 * time.Hour)) {
		t.Fatalf("证书有效期过短: %s", leaf.NotAfter)
	}
	if _, err := x509.ParseCertificate(first.Certificate().Certificate[0]); err != nil {
		t.Fatalf("证书不可解析: %v", err)
	}
}

func TestFingerprintFormat(t *testing.T) {
	id, err := node.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatalf("加载: %v", err)
	}
	re := regexp.MustCompile(`^[A-Z2-7]{4}(-[A-Z2-7]{4})+$`)
	if !re.MatchString(id.Fingerprint()) {
		t.Fatalf("指纹格式不符: %q", id.Fingerprint())
	}
}

func TestSignAndVerify(t *testing.T) {
	id, err := node.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatalf("加载: %v", err)
	}
	msg := node.AuthMessage("nonce-1", id.Fingerprint(), 1234567890)
	sig, err := id.Sign(msg)
	if err != nil {
		t.Fatalf("签名: %v", err)
	}
	if err := node.Verify(id.PublicKey(), msg, sig); err != nil {
		t.Fatalf("验签失败: %v", err)
	}
	if err := node.Verify(id.PublicKey(), append(msg, 'x'), sig); err == nil {
		t.Fatal("篡改后的消息应验签失败")
	}
	if err := node.Verify(id.PublicKey(), msg, []byte("bad")); err == nil {
		t.Fatal("坏签名应验签失败")
	}
}

func TestRotateInvalidatesOldKey(t *testing.T) {
	id, err := node.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatalf("加载: %v", err)
	}
	oldFP := id.Fingerprint()
	msg := node.AuthMessage("n", oldFP, 1)
	sig, err := id.Sign(msg)
	if err != nil {
		t.Fatalf("签名: %v", err)
	}
	if err := id.Rotate(); err != nil {
		t.Fatalf("轮换: %v", err)
	}
	if id.Fingerprint() == oldFP {
		t.Fatal("轮换后指纹未变")
	}
	if err := node.Verify(id.PublicKey(), msg, sig); err == nil {
		t.Fatal("旧私钥的签名不应通过新公钥校验")
	}
}

func TestParsePublicKey(t *testing.T) {
	id, err := node.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatalf("加载: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(id.PublicKey())
	if err != nil {
		t.Fatalf("编码公钥: %v", err)
	}
	pub, err := node.ParsePublicKey(base64.StdEncoding.EncodeToString(der))
	if err != nil {
		t.Fatalf("解析公钥: %v", err)
	}
	if !pub.Equal(id.PublicKey()) {
		t.Fatal("解析出的公钥与原公钥不一致")
	}
	if _, err := node.ParsePublicKey("not-base64!"); err == nil {
		t.Fatal("非法 base64 应报错")
	}
}

func TestAuthMessageEncoding(t *testing.T) {
	got := string(node.AuthMessage("abc", "ABCD-EFGH", 42))
	want := "abc\nABCD-EFGH\n42"
	if got != want {
		t.Fatalf("认证消息编码不符: %q != %q", got, want)
	}
}
