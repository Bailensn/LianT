// Package server 实现协议 v1 的 WSS 入口：挑战认证、首次配对与消息分发。
//
// 传输细节（TLS、固定端口、帧上限）在这里收口，业务方法通过 Register 挂进来；
// 核心方法（bot.list、msg.send 等）不直接依赖 websocket。
package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Bailensn/LianT/service/internal/node"
	"github.com/Bailensn/LianT/service/internal/pairing"
	"github.com/Bailensn/LianT/service/internal/protocol"
	"github.com/Bailensn/LianT/service/internal/store"
)

const (
	// 握手阶段与普通读写的超时。
	handshakeTimeout = 15 * time.Second
	writeTimeout     = 10 * time.Second
	// 认证后没有收到任何帧（含 ping）超过该时长就断开。
	// 连接空闲上限：心跳会持续刷新，这里只是兜底回收死连接。
	idleTimeout = 5 * time.Minute
	// 请求幂等缓存窗口（协议 §9）。
	dedupeTTL = 5 * time.Minute

	// 每台设备每分钟最多 5 次认证失败（协议 §4）。
	maxAuthFailures = 5
	authWindow      = time.Minute

	// 单连接最多尝试 3 次认证。
	maxAuthAttempts = 3

	closeAuthFailed   = 4001
	closeVersion      = 4002
	closeServerLoaded = 4003
)

// DeviceStore 是服务端需要的设备持久化能力（store.Store 满足）。
type DeviceStore interface {
	Device(ctx context.Context, id string) (store.Device, error)
	PutDevice(ctx context.Context, d store.Device) error
	TouchDevice(ctx context.Context, id string) error
	CountDevices(ctx context.Context) (int, error)
}

// Options 是服务端的依赖。
type Options struct {
	Identity *node.Identity
	Devices  DeviceStore
	Pairer   *pairing.Pairer

	// Confirm 是配对前的本地确认回调：返回 true 才授权。
	// 终端实现里它应当把名称与指纹打到屏幕上并等待输入。
	Confirm func(ctx context.Context, name, fingerprint string, pub *ecdsa.PublicKey) bool

	// AllowNames 非空时，只有名字匹配其中任意一个 glob 的设备才允许配对。
	// 空表示不限制（仍然需要终端确认）。
	AllowNames []string

	// AutoPair 允许已通过 HTTP 基本认证的连接跳过配对码直接登记设备。
	// 只有在公网前面挂了 Basic 认证时才应该打开。
	AutoPair bool

	// Audit 记录安全事件；可为空。
	Audit func(ctx context.Context, ev AuditEvent)

	// Version 会随 auth.ok 下发，客户端用来判断是否需要刷新页面。
	Version string

	Retention  protocol.Retention
	MaxDevices int
	Now        func() time.Time
	Logf       func(format string, args ...any)
}

// AuditEvent 是一次与安全相关的事件。
type AuditEvent struct {
	Kind     string
	DeviceID string
	Name     string
	IP       string
	Reason   string
}

// Method 是一个已认证的方法处理器；返回的 any 会被编码成响应 payload。
type Method func(ctx context.Context, sess Session, payload json.RawMessage) (any, *protocol.Error)

// Session 是已认证连接的身份。
type Session struct {
	ManagerID string
}

// Server 是 WSS 入口。
type Server struct {
	opt     Options
	now     func() time.Time
	limiter *limiter

	mu      sync.RWMutex
	methods map[string]Method

	hubMu sync.Mutex
	hub   map[*conn]struct{}

	dedupeMu sync.Mutex
	dedupe   map[string]cachedResponse

	upgrader websocket.Upgrader
}

// New 校验依赖并构造服务端。
func New(opt Options) (*Server, error) {
	if opt.Identity == nil {
		return nil, errors.New("server: 缺少节点身份")
	}
	if opt.Devices == nil {
		return nil, errors.New("server: 缺少设备存储")
	}
	if opt.Pairer == nil {
		return nil, errors.New("server: 缺少配对器")
	}
	if opt.Confirm == nil {
		return nil, errors.New("server: 缺少配对确认回调")
	}
	if opt.MaxDevices <= 0 {
		opt.MaxDevices = 8
	}
	if opt.Now == nil {
		opt.Now = time.Now
	}
	if opt.Logf == nil {
		opt.Logf = func(string, ...any) {}
	}
	return &Server{
		opt:     opt,
		now:     opt.Now,
		limiter: newLimiter(maxAuthFailures, authWindow, opt.Now),
		methods: map[string]Method{},
		hub:     map[*conn]struct{}{},
		dedupe:  map[string]cachedResponse{},
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			// 网页端与 WSS 同源：只接受同源页面，其余带 Origin 的一律拒绝。
			CheckOrigin:      sameOrigin,
			EnableCompression: true,
		},
	}, nil
}

// Register 挂一个方法。重复注册会 panic——只应发生在启动期。
func (s *Server) Register(typ string, m Method) {
	if m == nil || typ == "" {
		panic("server: 注册空方法")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.methods[typ]; exists {
		panic("server: 重复注册 " + typ)
	}
	s.methods[typ] = m
}

// Handler 返回 /v1/ws 的 http.Handler。
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.serve)
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/ws" {
		http.NotFound(w, r)
		return
	}
	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()
	ws.SetReadLimit(protocol.MaxFrame + 4096)
	// 24h 挂机场景：让内核也帮忙探活，别只靠应用层心跳。
	if tcp, ok := ws.UnderlyingConn().(*net.TCPConn); ok {
		_ = tcp.SetKeepAlive(true)
		_ = tcp.SetKeepAlivePeriod(30 * time.Second)
	}
	// 客户端空闲时不发数据，靠 ping/pong 续命，避免 90s 空闲超时误杀。
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(s.now().Add(idleTimeout))
		return nil
	})

	ip := remoteIP(r)
	if !s.limiter.Allow(ip) {
		s.writeError(ws, "", protocol.CodeRateLimited, "认证失败次数过多，稍后再试", map[string]any{"retry_after_ms": 60000})
		ws.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(closeServerLoaded, "rate limited"), s.now().Add(writeTimeout))
		return
	}

	c := &conn{s: s, ws: ws, ip: ip, autoPair: s.opt.AutoPair}
	defer s.removeConn(c)
	if err := c.handshake(); err != nil {
		s.opt.Logf("握手结束: %v", err)
	}
}

type conn struct {
	s      *Server
	ws     *websocket.Conn
	ip     string
	sess     Session
	authed   bool
	autoPair bool
	failed int

	// 推送与响应可能来自不同 goroutine，写入口必须串行。
	wmu sync.Mutex
}

func (c *conn) audit(kind, deviceID, name, reason string) {
	ev := AuditEvent{Kind: kind, DeviceID: deviceID, Name: name, IP: c.ip, Reason: reason}
	if c.s.opt.Audit != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		c.s.opt.Audit(ctx, ev)
	}
	// 推给在线客户端，设置页不用手动刷新。
	c.s.Broadcast(protocol.TypeEventAudit, protocol.AuditEntry{
		TS:       c.s.now().UnixMilli(),
		Kind:     kind,
		DeviceID: deviceID,
		Name:     name,
		IP:       c.ip,
		Reason:   reason,
	})
}

// sameOrigin 只放行同源页面的 WebSocket 握手。
//
// 浏览器一定会带 Origin；命令行客户端不带，直接放行。
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

func (c *conn) handshake() error {
	challenge, err := c.s.newChallenge()
	if err != nil {
		return err
	}
	if err := c.write(protocol.TypeAuthChallenge, "", challenge); err != nil {
		return err
	}

	for !c.authed {
		env, err := c.read()
		if err != nil {
			return err
		}
		switch env.Type {
		case protocol.TypePing:
			if err := c.write(protocol.TypePong, env.ID, struct{}{}); err != nil {
				return err
			}
		case protocol.TypeAuthResponse:
			if err := c.handleAuth(env, challenge); err != nil {
				return err
			}
		case protocol.TypePairRequest:
			if err := c.handlePair(env); err != nil {
				return err
			}
		default:
			c.writeErrorFor(env, protocol.CodeUnauthorized, "认证前只能 pair.request / auth.response / ping", nil)
		}
	}
	return c.loop()
}

func (c *conn) handleAuth(env protocol.Envelope, challenge protocol.AuthChallenge) error {
	var req protocol.AuthResponse
	if err := env.DecodePayload(&req); err != nil {
		c.writeErrorFor(env, protocol.CodeInvalidPayload, "auth.response 字段不合法", nil)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dev, err := c.s.opt.Devices.Device(ctx, req.ManagerID)
	if err != nil {
		c.failed++
		c.s.limiter.Fail(c.ip)
		c.audit("auth.fail", req.ManagerID, "", "设备未授权")
		c.writeErrorFor(env, protocol.CodeUnauthorized, "设备未授权", nil)
		return c.tooManyAttempts()
	}
	pub, err := node.ParsePublicKey(base64.StdEncoding.EncodeToString(dev.PublicKey))
	if err != nil {
		c.s.limiter.Fail(c.ip)
		c.writeErrorFor(env, protocol.CodeUnauthorized, "设备公钥损坏", nil)
		return c.tooManyAttempts()
	}
	if skew := c.s.now().Sub(time.UnixMilli(challenge.TS)); skew > time.Minute || skew < -time.Minute {
		c.s.limiter.Fail(c.ip)
		c.writeErrorFor(env, protocol.CodeClockSkew, "客户端与服务端时间偏差过大", nil)
		return c.tooManyAttempts()
	}
	msg := node.AuthMessage(challenge.Nonce, c.s.opt.Identity.Fingerprint(), challenge.TS)
	sig, err := base64.StdEncoding.DecodeString(req.Sig)
	if err != nil {
		c.s.limiter.Fail(c.ip)
		c.writeErrorFor(env, protocol.CodeUnauthorized, "签名编码不合法", nil)
		return c.tooManyAttempts()
	}
	if err := node.Verify(pub, msg, sig); err != nil {
		c.failed++
		c.s.limiter.Fail(c.ip)
		c.audit("auth.fail", dev.ID, dev.Name, "签名校验失败")
		c.writeErrorFor(env, protocol.CodeUnauthorized, "签名校验失败", nil)
		return c.tooManyAttempts()
	}

	if err := c.s.opt.Devices.TouchDevice(ctx, dev.ID); err != nil {
		c.s.opt.Logf("更新设备在线时间失败: %v", err)
	}
	c.audit("auth.ok", dev.ID, dev.Name, "")
	c.sess = Session{ManagerID: dev.ID}
	c.authed = true
	c.s.addConn(c)
	c.ws.SetReadDeadline(time.Time{})
	return c.write(protocol.TypeAuthOK, env.ID, protocol.AuthOK{
		Version:     c.s.opt.Version,
		ServiceTime: c.s.now().UnixMilli(),
		Caps:        protocol.ServerCaps(),
		Retention:   c.s.opt.Retention,
	})
}

func (c *conn) handlePair(env protocol.Envelope) error {
	var req protocol.PairRequest
	if err := env.DecodePayload(&req); err != nil {
		c.writeErrorFor(env, protocol.CodeInvalidPayload, "pair.request 字段不合法", nil)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	count, err := c.s.opt.Devices.CountDevices(ctx)
	if err != nil {
		c.s.opt.Logf("统计设备失败: %v", err)
		return c.denyPair(env, "internal")
	}
	if count >= c.s.opt.MaxDevices {
		return c.denyPair(env, "devices_full")
	}

	// 已通过 Basic 认证的网页端可以不带配对码直接登记。
	auto := c.autoPair && req.Ticket == ""

	pending, ok := c.s.opt.Pairer.Pending()
	if !ok && !auto {
		return c.denyPair(env, "no_ticket")
	}
	pub, err := node.ParsePublicKey(req.ManagerPub)
	if err != nil {
		c.s.limiter.Fail(c.ip)
		return c.denyPair(env, "invalid_pub")
	}
	sig, err := base64.StdEncoding.DecodeString(req.Sig)
	if err != nil {
		c.s.limiter.Fail(c.ip)
		return c.denyPair(env, "invalid_sig")
	}
	if err := node.Verify(pub, []byte(pending.Nonce), sig); err != nil {
		c.s.limiter.Fail(c.ip)
		return c.denyPair(env, "invalid_sig")
	}
	// 签名先验，再消费配对码：签名失败不应烧掉码。
	if err := c.s.opt.Pairer.Redeem(req.Ticket, pending.Nonce); err != nil {
		c.s.limiter.Fail(c.ip)
		return c.denyPair(env, pairReason(err))
	}

	name := req.ManagerName
	if name == "" {
		name = "未命名设备"
	}
	if !nameAllowed(name, c.s.opt.AllowNames) {
		c.audit("pair.denied", "", name, "设备名不在白名单")
		return c.denyPair(env, "name_not_allowed")
	}
	fp := node.PinnedFingerprint(pub)
	// 自动登记没有终端可确认，信任已经通过的那道 Basic 认证。
	if !auto && !c.s.opt.Confirm(ctx, name, fp, pub) {
		return c.denyPair(env, "denied")
	}

	der, err := marshalPublicKey(pub)
	if err != nil {
		return c.denyPair(env, "internal")
	}
	deviceID := newDeviceID()
	if err := c.s.opt.Devices.PutDevice(ctx, store.Device{
		ID:        deviceID,
		Name:      name,
		PublicKey: der,
	}); err != nil {
		c.s.opt.Logf("登记设备失败: %v", err)
		return c.denyPair(env, "internal")
	}
	c.audit("pair.ok", deviceID, name, "")
	return c.write(protocol.TypePairOK, env.ID, protocol.PairOK{
		ServiceID:    c.s.opt.Identity.Fingerprint(),
		ServiceName:  "LianT Service",
		DevicesCount: count + 1,
		DeviceID:     deviceID,
	})
}

func (c *conn) denyPair(env protocol.Envelope, reason string) error {
	c.audit("pair.denied", "", "", reason)
	return c.write(protocol.TypePairDenied, env.ID, protocol.PairDenied{Reason: reason})
}

// nameAllowed 用 glob 匹配设备名；白名单为空表示不限制。
func nameAllowed(name string, allow []string) bool {
	if len(allow) == 0 {
		return true
	}
	for _, pattern := range allow {
		if ok, err := path.Match(pattern, name); err == nil && ok {
			return true
		}
	}
	return false
}

func (c *conn) loop() error {
	done := make(chan struct{})
	defer close(done)
	go c.pingLoop(done)
	for {
		env, err := c.read()
		if err != nil {
			return err
		}
		switch env.Type {
		case protocol.TypePing:
			if err := c.write(protocol.TypePong, env.ID, struct{}{}); err != nil {
				return err
			}
			continue
		}

		// 同一个 id 重试直接回放上次的成功响应（协议 §9）。
		key := c.sess.ManagerID + "\x00" + env.ID
		if data, ok := c.s.cachedResponse(key); ok {
			if err := c.writeRaw(data); err != nil {
				return err
			}
			continue
		}

		c.s.mu.RLock()
		method, ok := c.s.methods[env.Type]
		c.s.mu.RUnlock()
		if !ok {
			c.writeErrorFor(env, protocol.CodeUnknownType, "未知的 type: "+env.Type, nil)
			continue
		}
		payload, perr := method(context.Background(), c.sess, env.Payload)
		if perr != nil {
			c.writeErrorFor(env, perr.Code, perr.Message, perr.Details)
			continue
		}
		data, err := c.encode(env.Type, env.ID, payload)
		if err != nil {
			return err
		}
		c.s.remember(key, data)
		if err := c.writeRaw(data); err != nil {
			return err
		}
	}
}

func (c *conn) encode(typ, replyTo string, payload any) ([]byte, error) {
	env, err := protocol.New(newMessageID(), typ, c.s.now().UnixMilli(), payload)
	if err != nil {
		return nil, err
	}
	env.ReplyTo = replyTo
	return json.Marshal(env)
}

func (c *conn) writeRaw(data []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	c.ws.SetWriteDeadline(c.s.now().Add(writeTimeout))
	return c.ws.WriteMessage(websocket.TextMessage, data)
}

func (c *conn) tooManyAttempts() error {
	if c.failed < maxAuthAttempts {
		return nil
	}
	c.ws.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(closeAuthFailed, "too many attempts"), c.s.now().Add(writeTimeout))
	return errors.New("server: 认证失败次数过多")
}

// pingLoop 每 30s 发一次控制帧 ping；pong 会刷新读超时。
func (c *conn) pingLoop(done <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			c.wmu.Lock()
			err := c.ws.WriteControl(websocket.PingMessage, nil, c.s.now().Add(writeTimeout))
			c.wmu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

func (c *conn) read() (protocol.Envelope, error) {
	// 握手阶段 15s；认证后 90s 收不到任何帧（含 ping）就断开。
	timeout := handshakeTimeout
	if c.authed {
		timeout = idleTimeout
	}
	c.ws.SetReadDeadline(c.s.now().Add(timeout))
	var env protocol.Envelope
	_, data, err := c.ws.ReadMessage()
	if err != nil {
		return env, err
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return env, fmt.Errorf("server: 解析信封: %w", err)
	}
	if err := env.Validate(); err != nil {
		if errors.Is(err, protocol.ErrUnsupportedVersion) {
			c.ws.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(closeVersion, "unsupported version"), c.s.now().Add(writeTimeout))
			return env, err
		}
		c.writeErrorFor(env, protocol.CodeInvalidPayload, err.Error(), nil)
		return env, err
	}
	return env, nil
}

func (c *conn) write(typ, replyTo string, payload any) error {
	env, err := protocol.New(newMessageID(), typ, c.s.now().UnixMilli(), payload)
	if err != nil {
		return err
	}
	env.ReplyTo = replyTo
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	c.ws.SetWriteDeadline(c.s.now().Add(writeTimeout))
	return c.ws.WriteMessage(websocket.TextMessage, data)
}

func (c *conn) writeErrorFor(env protocol.Envelope, code, msg string, details map[string]any) {
	_ = c.write(protocol.TypeError, env.ID, protocol.Error{
		Code:      code,
		Message:   msg,
		Retryable: protocol.Retryable(code),
		Details:   details,
	})
}

func (s *Server) writeError(ws *websocket.Conn, replyTo, code, msg string, details map[string]any) {
	env, err := protocol.New(newMessageID(), protocol.TypeError, s.now().UnixMilli(), protocol.Error{
		Code:      code,
		Message:   msg,
		Retryable: protocol.Retryable(code),
		Details:   details,
	})
	if err != nil {
		return
	}
	env.ReplyTo = replyTo
	data, _ := json.Marshal(env)
	ws.SetWriteDeadline(s.now().Add(writeTimeout))
	_ = ws.WriteMessage(websocket.TextMessage, data)
}

// Broadcast 把事件推给所有已认证连接。
//
// 推送失败只记日志：客户端断线后靠游标补拉，不应影响其他连接。
func (s *Server) Broadcast(typ string, payload any) {
	s.hubMu.Lock()
	conns := make([]*conn, 0, len(s.hub))
	for c := range s.hub {
		conns = append(conns, c)
	}
	s.hubMu.Unlock()

	for _, c := range conns {
		if err := c.write(typ, "", payload); err != nil {
			// 写失败说明连接已死，直接摘掉，别留着反复报错。
			s.removeConn(c)
			_ = c.ws.Close()
			s.opt.Logf("推送 %s 失败: %v", typ, err)
		}
	}
}

func (s *Server) addConn(c *conn) {
	s.hubMu.Lock()
	s.hub[c] = struct{}{}
	s.hubMu.Unlock()
}

func (s *Server) removeConn(c *conn) {
	s.hubMu.Lock()
	delete(s.hub, c)
	s.hubMu.Unlock()
}

func (s *Server) newChallenge() (protocol.AuthChallenge, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return protocol.AuthChallenge{}, fmt.Errorf("server: 生成 nonce: %w", err)
	}
	ch := protocol.AuthChallenge{
		Nonce:    base64.RawURLEncoding.EncodeToString(buf),
		TS:       s.now().UnixMilli(),
		AutoPair: s.opt.AutoPair,
	}
	// 服务端身份自证：客户端钉的是指纹，用它校验公钥比 TLS 更可靠。
	if pub, err := s.opt.Identity.PublicKeyB64(); err == nil {
		ch.ServicePub = pub
		msg := node.AuthMessage(ch.Nonce, s.opt.Identity.Fingerprint(), ch.TS)
		if sig, err := s.opt.Identity.Sign(msg); err == nil {
			ch.ServiceSig = base64.StdEncoding.EncodeToString(sig)
		}
	}
	return ch, nil
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func newDeviceID() string { return "dev-" + randomHex(12) }

func newMessageID() string { return randomHex(16) }

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand 失败属于不可恢复的环境错误。
		panic("server: 生成随机数失败: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func marshalPublicKey(pub *ecdsa.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("server: 编码公钥: %w", err)
	}
	return der, nil
}

func pairReason(err error) string {
	switch {
	case errors.Is(err, pairing.ErrExpired):
		return "expired"
	case errors.Is(err, pairing.ErrUsed):
		return "used"
	case errors.Is(err, pairing.ErrNoTicket):
		return "no_ticket"
	default:
		return "invalid_ticket"
	}
}

// limiter 是按 IP 的滑动窗口，只记录认证失败。
type cachedResponse struct {
	data []byte
	exp  time.Time
}

func (s *Server) cachedResponse(key string) ([]byte, bool) {
	s.dedupeMu.Lock()
	defer s.dedupeMu.Unlock()
	entry, ok := s.dedupe[key]
	if !ok || s.now().After(entry.exp) {
		delete(s.dedupe, key)
		return nil, false
	}
	return entry.data, true
}

func (s *Server) remember(key string, data []byte) {
	s.dedupeMu.Lock()
	defer s.dedupeMu.Unlock()
	if len(s.dedupe) > 4096 {
		now := s.now()
		for k, v := range s.dedupe {
			if now.After(v.exp) {
				delete(s.dedupe, k)
			}
		}
	}
	s.dedupe[key] = cachedResponse{data: data, exp: s.now().Add(dedupeTTL)}
}

type limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	now    func() time.Time
	fails  map[string][]time.Time
}

func newLimiter(max int, window time.Duration, now func() time.Time) *limiter {
	return &limiter{max: max, window: window, now: now, fails: map[string][]time.Time{}}
}

func (l *limiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(ip)
	return len(l.fails[ip]) < l.max
}

func (l *limiter) Fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(ip)
	l.fails[ip] = append(l.fails[ip], l.now())
}

func (l *limiter) prune(ip string) {
	cutoff := l.now().Add(-l.window)
	kept := l.fails[ip][:0]
	for _, t := range l.fails[ip] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.fails, ip)
		return
	}
	l.fails[ip] = kept
}
