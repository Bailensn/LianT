package server_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sync/atomic"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Bailensn/LianT/service/internal/node"
	"github.com/Bailensn/LianT/service/internal/pairing"
	"github.com/Bailensn/LianT/service/internal/protocol"
	"github.com/Bailensn/LianT/service/internal/server"
	"github.com/Bailensn/LianT/service/internal/store"
)

type testEnv struct {
	id   *node.Identity
	db   *store.Store
	pair *pairing.Pairer
	http *httptest.Server
	srv  *server.Server
	echo atomic.Int32
}

func newEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	id, err := node.LoadOrCreate(dir + "/node")
	if err != nil {
		t.Fatalf("节点身份: %v", err)
	}
	db, err := store.Open(dir + "/db.sqlite")
	if err != nil {
		t.Fatalf("数据库: %v", err)
	}
	p := pairing.New()
	srv, err := server.New(server.Options{
		Identity: id,
		Devices:  db,
		Pairer:   p,
		Confirm: func(context.Context, string, string, *ecdsa.PublicKey) bool {
			return true
		},
		Retention: protocol.Retention{Days: 7, MediaMaxBytes: 3 << 30},
	})
	if err != nil {
		t.Fatalf("服务端: %v", err)
	}
	env := &testEnv{id: id, db: db, pair: p, srv: srv}
	srv.Register("test.echo", func(context.Context, server.Session, json.RawMessage) (any, *protocol.Error) {
		n := env.echo.Add(1)
		return map[string]any{"n": n}, nil
	})
	ts := httptest.NewTLSServer(srv.Handler())
	env.http = ts
	t.Cleanup(func() {
		ts.Close()
		db.Close()
	})
	return env
}

func (e *testEnv) dial(t *testing.T) *websocket.Conn {
	t.Helper()
	dialer := websocket.Dialer{
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true}, // 测试用：真实客户端必须钉指纹
		HandshakeTimeout: 5 * time.Second,
	}
	url := "wss" + strings.TrimPrefix(e.http.URL, "https") + "/v1/ws"
	c, _, err := dialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("连接: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func readEnv(t *testing.T, c *websocket.Conn) protocol.Envelope {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("读取: %v", err)
	}
	var env protocol.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("解析信封: %v", err)
	}
	return env
}

func writeEnv(t *testing.T, c *websocket.Conn, id, typ string, payload any) {
	t.Helper()
	env, err := protocol.New(id, typ, time.Now().UnixMilli(), payload)
	if err != nil {
		t.Fatalf("构造信封: %v", err)
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("编码: %v", err)
	}
	c.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("写入: %v", err)
	}
}

func newManagerKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("生成设备密钥: %v", err)
	}
	return key
}

func sign(t *testing.T, key *ecdsa.PrivateKey, msg []byte) []byte {
	t.Helper()
	sum := sha256.Sum256(msg)
	sig, err := ecdsa.SignASN1(rand.Reader, key, sum[:])
	if err != nil {
		t.Fatalf("签名: %v", err)
	}
	return sig
}

func pubDER(t *testing.T, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("编码公钥: %v", err)
	}
	return der
}

func authChallenge(t *testing.T, c *websocket.Conn) protocol.AuthChallenge {
	t.Helper()
	env := readEnv(t, c)
	if env.Type != protocol.TypeAuthChallenge {
		t.Fatalf("期望 auth.challenge，得到 %s", env.Type)
	}
	var ch protocol.AuthChallenge
	if err := env.DecodePayload(&ch); err != nil {
		t.Fatalf("解析挑战: %v", err)
	}
	return ch
}

func TestAuthPingAndUnknownType(t *testing.T) {
	e := newEnv(t)
	key := newManagerKey(t)
	if err := e.db.PutDevice(context.Background(), store.Device{
		ID: "dev-1", Name: "测试机", PublicKey: pubDER(t, key),
	}); err != nil {
		t.Fatalf("登记设备: %v", err)
	}

	c := e.dial(t)
	ch := authChallenge(t, c)
	sig := sign(t, key, node.AuthMessage(ch.Nonce, e.id.Fingerprint(), ch.TS))
	writeEnv(t, c, "auth-1", protocol.TypeAuthResponse, protocol.AuthResponse{
		ManagerID: "dev-1",
		Sig:       base64.StdEncoding.EncodeToString(sig),
	})

	ok := readEnv(t, c)
	if ok.Type != protocol.TypeAuthOK || ok.ReplyTo != "auth-1" {
		t.Fatalf("期望 auth.ok，得到 %s reply_to=%s", ok.Type, ok.ReplyTo)
	}
	var authOK protocol.AuthOK
	if err := ok.DecodePayload(&authOK); err != nil {
		t.Fatalf("解析 auth.ok: %v", err)
	}
	if len(authOK.Caps) == 0 || !contains(authOK.Caps, protocol.CapMultiBot) {
		t.Fatalf("caps 不符: %v", authOK.Caps)
	}

	writeEnv(t, c, "ping-1", protocol.TypePing, struct{}{})
	pong := readEnv(t, c)
	if pong.Type != protocol.TypePong || pong.ReplyTo != "ping-1" {
		t.Fatalf("期望 pong，得到 %s", pong.Type)
	}

	writeEnv(t, c, "x-1", "no.such.method", struct{}{})
	errEnv := readEnv(t, c)
	if errEnv.Type != protocol.TypeError || errEnv.ReplyTo != "x-1" {
		t.Fatalf("期望 error，得到 %s", errEnv.Type)
	}
	var perr protocol.Error
	if err := errEnv.DecodePayload(&perr); err != nil {
		t.Fatalf("解析 error: %v", err)
	}
	if perr.Code != protocol.CodeUnknownType {
		t.Fatalf("错误码不符: %s", perr.Code)
	}
}

func TestAuthRejectsBadSignature(t *testing.T) {
	e := newEnv(t)
	key := newManagerKey(t)
	other := newManagerKey(t)
	if err := e.db.PutDevice(context.Background(), store.Device{
		ID: "dev-1", Name: "测试机", PublicKey: pubDER(t, key),
	}); err != nil {
		t.Fatalf("登记设备: %v", err)
	}

	c := e.dial(t)
	ch := authChallenge(t, c)
	sig := sign(t, other, node.AuthMessage(ch.Nonce, e.id.Fingerprint(), ch.TS))
	writeEnv(t, c, "auth-1", protocol.TypeAuthResponse, protocol.AuthResponse{
		ManagerID: "dev-1",
		Sig:       base64.StdEncoding.EncodeToString(sig),
	})

	env := readEnv(t, c)
	if env.Type != protocol.TypeError {
		t.Fatalf("期望 error，得到 %s", env.Type)
	}
	var perr protocol.Error
	if err := env.DecodePayload(&perr); err != nil {
		t.Fatalf("解析 error: %v", err)
	}
	if perr.Code != protocol.CodeUnauthorized {
		t.Fatalf("错误码不符: %s", perr.Code)
	}
}

func TestPairThenAuth(t *testing.T) {
	e := newEnv(t)
	code, err := e.pair.Issue()
	if err != nil {
		t.Fatalf("生成配对码: %v", err)
	}
	key := newManagerKey(t)

	c := e.dial(t)
	ch := authChallenge(t, c)

	sig := sign(t, key, []byte(code.Nonce))
	writeEnv(t, c, "pair-1", protocol.TypePairRequest, protocol.PairRequest{
		Ticket:      code.Value,
		ManagerName: "新手机",
		ManagerPub:  base64.StdEncoding.EncodeToString(pubDER(t, key)),
		Sig:         base64.StdEncoding.EncodeToString(sig),
	})

	ok := readEnv(t, c)
	if ok.Type != protocol.TypePairOK {
		t.Fatalf("期望 pair.ok，得到 %s: %s", ok.Type, string(ok.Payload))
	}
	var pairOK protocol.PairOK
	if err := ok.DecodePayload(&pairOK); err != nil {
		t.Fatalf("解析 pair.ok: %v", err)
	}
	if pairOK.DeviceID == "" {
		t.Fatal("pair.ok 缺少 device_id")
	}
	if pairOK.DevicesCount != 1 {
		t.Fatalf("设备数不符: %d", pairOK.DevicesCount)
	}

	// 用刚拿到的设备 ID 完成认证。
	authSig := sign(t, key, node.AuthMessage(ch.Nonce, e.id.Fingerprint(), ch.TS))
	writeEnv(t, c, "auth-1", protocol.TypeAuthResponse, protocol.AuthResponse{
		ManagerID: pairOK.DeviceID,
		Sig:       base64.StdEncoding.EncodeToString(authSig),
	})
	if env := readEnv(t, c); env.Type != protocol.TypeAuthOK {
		t.Fatalf("配对后认证失败: %s", env.Type)
	}
}

func TestRejectsUnsupportedVersion(t *testing.T) {
	e := newEnv(t)
	c := e.dial(t)
	_ = authChallenge(t, c)

	raw := `{"v":2,"id":"x","type":"ping"}`
	c.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := c.WriteMessage(websocket.TextMessage, []byte(raw)); err != nil {
		t.Fatalf("写入: %v", err)
	}
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err := c.ReadMessage()
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) {
		t.Fatalf("期望关闭帧，得到 %v", err)
	}
	if closeErr.Code != 4002 {
		t.Fatalf("关闭码应为 4002，得到 %d", closeErr.Code)
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func TestRequestIdempotency(t *testing.T) {
	e := newEnv(t)
	key := newManagerKey(t)
	if err := e.db.PutDevice(context.Background(), store.Device{
		ID: "dev-1", Name: "测试机", PublicKey: pubDER(t, key),
	}); err != nil {
		t.Fatalf("登记设备: %v", err)
	}

	c := e.dial(t)
	ch := authChallenge(t, c)
	sig := sign(t, key, node.AuthMessage(ch.Nonce, e.id.Fingerprint(), ch.TS))
	writeEnv(t, c, "auth-1", protocol.TypeAuthResponse, protocol.AuthResponse{
		ManagerID: "dev-1",
		Sig:       base64.StdEncoding.EncodeToString(sig),
	})
	if env := readEnv(t, c); env.Type != protocol.TypeAuthOK {
		t.Fatalf("认证失败: %s", env.Type)
	}

	writeEnv(t, c, "dup-1", "test.echo", struct{}{})
	first := readEnv(t, c)
	writeEnv(t, c, "dup-1", "test.echo", struct{}{})
	second := readEnv(t, c)

	if first.ReplyTo != "dup-1" || second.ReplyTo != "dup-1" {
		t.Fatalf("reply_to 不符: %s / %s", first.ReplyTo, second.ReplyTo)
	}
	if string(first.Payload) != string(second.Payload) {
		t.Fatalf("重试响应不一致: %s vs %s", first.Payload, second.Payload)
	}
	if got := e.echo.Load(); got != 1 {
		t.Fatalf("同 id 只应执行一次，实际 %d", got)
	}
}
