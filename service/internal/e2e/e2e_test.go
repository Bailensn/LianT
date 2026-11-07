package e2e_test

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
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Bailensn/LianT/service/internal/core"
	"github.com/Bailensn/LianT/service/internal/node"
	"github.com/Bailensn/LianT/service/internal/pairing"
	"github.com/Bailensn/LianT/service/internal/platform"
	"github.com/Bailensn/LianT/service/internal/protocol"
	"github.com/Bailensn/LianT/service/internal/server"
	"github.com/Bailensn/LianT/service/internal/store"
)

// fakeAdapter 记录投递内容；不碰网络。
type fakeAdapter struct {
	name string
	sent []platform.SendRequest
}

func (f *fakeAdapter) Name() string { return f.name }

func (f *fakeAdapter) Validate(context.Context, platform.Credentials) (platform.Account, error) {
	return platform.Account{
		ID:          "42",
		DisplayName: "端到端测试号",
		Caps:        []string{platform.CapText},
		Limits:      map[string]any{"max_text_len": float64(2000)},
		Constraints: map[string]any{
			"can_initiate":           false,
			"reply_window_s":         float64(300),
			"max_replies_per_inbound": float64(5),
		},
	}, nil
}

func (f *fakeAdapter) Run(context.Context, platform.Account, platform.Credentials, platform.Hooks) error {
	return nil
}

func (f *fakeAdapter) SendText(_ context.Context, _ platform.Account, _ platform.Credentials, req platform.SendRequest) (platform.MessageID, error) {
	f.sent = append(f.sent, req)
	return "out-1", nil
}

func (f *fakeAdapter) Format() string { return "qq-plain" }

func sign(t *testing.T, key *ecdsa.PrivateKey, msg []byte) []byte {
	t.Helper()
	sum := sha256.Sum256(msg)
	sig, err := ecdsa.SignASN1(rand.Reader, key, sum[:])
	if err != nil {
		t.Fatalf("签名: %v", err)
	}
	return sig
}

func readEnvelope(t *testing.T, c *websocket.Conn) protocol.Envelope {
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

func writeEnvelope(t *testing.T, c *websocket.Conn, id, typ string, payload any) {
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

// readReply 跳过事件帧，直到拿到指定 reply_to 的响应。
func readReply(t *testing.T, c *websocket.Conn, replyTo string) protocol.Envelope {
	t.Helper()
	for i := 0; i < 10; i++ {
		env := readEnvelope(t, c)
		if env.ReplyTo == replyTo {
			return env
		}
	}
	t.Fatalf("没有等到 %s 的响应", replyTo)
	return protocol.Envelope{}
}

func TestFakeClientRunsVerticalSlice(t *testing.T) {
	dir := t.TempDir()
	identity, err := node.LoadOrCreate(dir + "/node")
	if err != nil {
		t.Fatalf("节点身份: %v", err)
	}
	db, err := store.Open(dir + "/db.sqlite")
	if err != nil {
		t.Fatalf("数据库: %v", err)
	}
	defer db.Close()

	pairer := pairing.New()
	adapter := &fakeAdapter{name: "qqbot"}

	srv, err := server.New(server.Options{
		Identity: identity,
		Devices:  db,
		Pairer:   pairer,
		Confirm: func(context.Context, string, string, *ecdsa.PublicKey) bool { return true },
		Retention: protocol.Retention{
			Days: 7,
		},
	})
	if err != nil {
		t.Fatalf("服务端: %v", err)
	}
	svc, err := core.New(core.Options{
		Store: db,
		Adapter: func(name string) (platform.Adapter, bool) {
			if name == adapter.Name() {
				return adapter, true
			}
			return nil, false
		},
		Broadcaster: srv,
		Encrypt:     func(raw []byte) (string, error) { return string(raw), nil },
		DecodeCredentials: func(b store.Bot) (platform.Credentials, error) {
			var creds platform.Credentials
			if err := json.Unmarshal(b.Credentials, &creds); err != nil {
				return nil, err
			}
			return creds, nil
		},
	})
	if err != nil {
		t.Fatalf("核心: %v", err)
	}
	svc.Register(srv)

	httpSrv := httptest.NewTLSServer(srv.Handler())
	defer httpSrv.Close()

	managerKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("设备密钥: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&managerKey.PublicKey)
	if err != nil {
		t.Fatalf("编码公钥: %v", err)
	}

	code, err := pairer.Issue()
	if err != nil {
		t.Fatalf("配对码: %v", err)
	}

	dialer := websocket.Dialer{
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true}, // 测试固定用自签证书
		HandshakeTimeout: 5 * time.Second,
	}
	url := "wss" + strings.TrimPrefix(httpSrv.URL, "https") + "/v1/ws"
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("连接: %v", err)
	}
	defer conn.Close()

	challengeEnv := readEnvelope(t, conn)
	if challengeEnv.Type != protocol.TypeAuthChallenge {
		t.Fatalf("期望 auth.challenge，得到 %s", challengeEnv.Type)
	}
	var challenge protocol.AuthChallenge
	if err := challengeEnv.DecodePayload(&challenge); err != nil {
		t.Fatalf("解析挑战: %v", err)
	}

	// 1. 配对
	pairSig := sign(t, managerKey, []byte(code.Nonce))
	writeEnvelope(t, conn, "pair-1", protocol.TypePairRequest, protocol.PairRequest{
		Ticket:      code.Value,
		ManagerName: "假客户端",
		ManagerPub:  base64.StdEncoding.EncodeToString(pubDER),
		Sig:         base64.StdEncoding.EncodeToString(pairSig),
	})
	pairEnv := readReply(t, conn, "pair-1")
	if pairEnv.Type != protocol.TypePairOK {
		t.Fatalf("期望 pair.ok，得到 %s", pairEnv.Type)
	}
	var pairOK protocol.PairOK
	if err := pairEnv.DecodePayload(&pairOK); err != nil {
		t.Fatalf("解析 pair.ok: %v", err)
	}

	// 2. 认证
	authSig := sign(t, managerKey, node.AuthMessage(challenge.Nonce, identity.Fingerprint(), challenge.TS))
	writeEnvelope(t, conn, "auth-1", protocol.TypeAuthResponse, protocol.AuthResponse{
		ManagerID: pairOK.DeviceID,
		Sig:       base64.StdEncoding.EncodeToString(authSig),
	})
	if env := readReply(t, conn, "auth-1"); env.Type != protocol.TypeAuthOK {
		t.Fatalf("期望 auth.ok，得到 %s", env.Type)
	}

	// 3. 登记 Bot
	writeEnvelope(t, conn, "add-1", protocol.TypeBotAdd, protocol.BotAdd{
		Platform:    "qqbot",
		Credentials: map[string]string{"app_id": "42", "app_secret": "s"},
		Label:       "e2e",
	})
	var botRef protocol.BotRef
	if err := readReply(t, conn, "add-1").DecodePayload(&botRef); err != nil {
		t.Fatalf("解析 bot.add: %v", err)
	}
	if botRef.BotID != "qqbot:42" {
		t.Fatalf("bot_id 不符: %s", botRef.BotID)
	}

	// 4. 入站消息：落库并推给客户端
	inserted, err := svc.Ingest(context.Background(), botRef.BotID, platform.Inbound{
		MessageID: "in-1",
		Peer:      platform.Peer{ID: "qqbot:peer-1", Kind: platform.PeerGroup},
		SenderID:  "qqbot:user-1",
		Kind:      "text",
		Text:      "在吗",
		At:        time.Now(),
	})
	if err != nil || !inserted {
		t.Fatalf("入站: inserted=%v err=%v", inserted, err)
	}
	push := readEnvelope(t, conn)
	if push.Type != protocol.TypeMsgInbound {
		t.Fatalf("期望 msg.inbound 推送，得到 %s", push.Type)
	}
	var pushed protocol.MsgInbound
	if err := push.DecodePayload(&pushed); err != nil {
		t.Fatalf("解析推送: %v", err)
	}
	if pushed.MsgID != "in-1" || pushed.Body.Plain != "在吗" {
		t.Fatalf("推送载荷不符: %+v", pushed)
	}

	// 5. 拉历史
	writeEnvelope(t, conn, "list-1", protocol.TypeMsgList, protocol.MsgList{
		BotID: botRef.BotID, PeerID: "qqbot:peer-1", Limit: 10,
	})
	var list protocol.MsgListOK
	if err := readReply(t, conn, "list-1").DecodePayload(&list); err != nil {
		t.Fatalf("解析 msg.list: %v", err)
	}
	if len(list.Messages) != 1 || list.Messages[0].MsgID != "in-1" {
		t.Fatalf("历史不符: %+v", list)
	}

	// 6. 回复
	writeEnvelope(t, conn, "send-1", protocol.TypeMsgSend, protocol.MsgSend{
		BotID:     botRef.BotID,
		PeerID:    "qqbot:peer-1",
		Body:      protocol.Body{Plain: "在的"},
		InReplyTo: "in-1",
	})
	var sent protocol.MsgSendOK
	if err := readReply(t, conn, "send-1").DecodePayload(&sent); err != nil {
		t.Fatalf("解析 msg.send: %v", err)
	}
	if sent.MsgID != "out-1" {
		t.Fatalf("发送结果不符: %+v", sent)
	}
	if len(adapter.sent) != 1 || adapter.sent[0].Text != "在的" || adapter.sent[0].InReplyTo != "in-1" {
		t.Fatalf("平台调用不符: %+v", adapter.sent)
	}
}
