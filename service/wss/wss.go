package wss

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/LensnTeam/LianT/service/crypto"
)

// sessionTTL 是新签发 session 的有效期。
const sessionTTL = 24 * time.Hour

// Server 是可供每个 bot 进程独立持有一份的 WSS 服务。
// 现在它只绑定本机回环，由 daemon 的单一安全入口对外反向代理，
// 避免每个 bot 进程暴露一个公网端口。
type Server struct {
	client *websocket.Conn
	mutex  sync.Mutex

	Token   string
	Address string
	Port    int
	used    bool

	// base 是对外的公开基地址（如 https://host:port/<id>），
	// 用于构造客户端可达的媒体下载/连接地址；为空时回退到本地地址。
	base string

	// Receive 用于接收来自 WSS 客户端的文本消息。
	Receive chan string

	// sessions 存有效 session（随机 id -> 过期时间）。首次用 token 登录后签发，
	// 之后客户端用 ?session= 登录。值的 id 是随机串，对外以加密形式下发。
	sessions map[string]time.Time

	// files 用于媒体下载代理：映射一次性 token 到真实的 Telegram 下载地址。
	// 下发给客户端的是不含 bot token 的代理地址，避免泄露 token。
	filesMu sync.Mutex
	files   map[string]*fileEntry

	handler *http.Server
	done    chan struct{}
}

type fileEntry struct {
	url    string
	client *http.Client
}

// New 创建一个空的 WSS 服务实例。
func New() *Server {
	return &Server{
		Receive:  make(chan string, 64),
		done:     make(chan struct{}),
		files:    make(map[string]*fileEntry),
		sessions: make(map[string]time.Time),
	}
}

// SetPublicBase 设置对外公开的基地址（如 https://host:port/<id>）。
// 用于让媒体下载/客户端连接的 URL 指向 daemon 的单一入口，而不是本机回环地址。
func (s *Server) SetPublicBase(base string) {
	s.base = strings.TrimRight(base, "/")
}

// URL 返回 WSS 连接地址。若设置了公开基地址，则基于它返回。
func (s *Server) URL() string {
	if s.base != "" {
		return strings.Replace(s.base, "https://", "wss://", 1) + "/ws"
	}
	return fmt.Sprintf(
		"wss://%s:%d/ws",
		s.Address,
		s.Port,
	)
}

// RegisterFile 登记一条真实下载地址（可能含 bot token，仅服务端持有），
// 返回一个不含 token 的一次性代理地址，供客户端下载。
// client 用于转发请求（可用代理配置）；为空时使用 http.DefaultClient。
func (s *Server) RegisterFile(
	realURL string,
	client *http.Client,
) (string, error) {
	tok := generateToken()
	s.filesMu.Lock()
	s.files[tok] = &fileEntry{
		url:    realURL,
		client: client,
	}
	s.filesMu.Unlock()
	prefix := s.base
	if prefix == "" {
		prefix = fmt.Sprintf("https://%s:%d", s.Address, s.Port)
	}
	return fmt.Sprintf(
		"%s/dl?token=%s",
		prefix,
		tok,
	), nil
}

func generateToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func randomPort() int {
	b := make([]byte, 2)
	_, _ = rand.Read(b)
	return 50000 + int(b[0])*39 + int(b[1])%39
}

func getIPv6() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP
			if ip.To4() != nil {
				continue
			}
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			return ip.String()
		}
	}
	return ""
}

func getPublicIPv4() string {
	cmd := exec.Command(
		"curl",
		"-4",
		"-s",
		"ifconfig.me",
	)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(
		string(out),
	)
}

func getLocalIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil || ip.IsLoopback() {
				continue
			}
			return ip.String()
		}
	}
	return ""
}

// ResolveAddress 决定对外通告的地址：优先 IPv6，其次公网 IPv4，
// 再退回到本机 IPv4；全部失败时使用 localhost。
func ResolveAddress() string {
	if ipv6 := getIPv6(); ipv6 != "" {
		return "[" + ipv6 + "]"
	}
	if ipv4 := getPublicIPv4(); ipv4 != "" {
		return ipv4
	}
	if ipv4 := getLocalIPv4(); ipv4 != "" {
		return ipv4
	}
	return "localhost"
}

func generateCert() (tls.Certificate, error) {
	key, err := rsa.GenerateKey(
		rand.Reader,
		2048,
	)
	if err != nil {
		return tls.Certificate{}, err
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "Temporary WSS",
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&key.PublicKey,
		key,
	)
	if err != nil {
		return tls.Certificate{}, err
	}
	cert := pem.EncodeToMemory(
		&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: der,
		},
	)
	keyPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		},
	)
	return tls.X509KeyPair(
		cert,
		keyPEM,
	)
}

// GenerateCert 生成一个自签名 TLS 证书，供 daemon 的单一安全入口使用。
func GenerateCert() (tls.Certificate, error) {
	return generateCert()
}

// listenPort 在 bindHost 上尝试绑定一个随机端口（bindHost 为空则绑定全部接口），
// 有上限次数，避免无 IPv6 环境下无限重试。
func listenPort(bindHost string) (net.Listener, int, error) {
	for i := 0; i < 20; i++ {
		port := randomPort()
		addr := fmt.Sprintf(":%d", port)
		if bindHost != "" {
			addr = fmt.Sprintf("%s:%d", bindHost, port)
		}
		l, err := net.Listen(
			"tcp",
			addr,
		)
		if err == nil {
			return l, port, nil
		}
	}
	return nil, 0, errors.New("无法分配可用端口")
}

// mediaDownloadHandler 在一次性范围内代理下载真实的 Telegram 媒体文件，
// 对外只暴露不含 bot token 的一次性链接。
func (s *Server) mediaDownloadHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	tok := r.URL.Query().Get("token")
	s.filesMu.Lock()
	e := s.files[tok]
	if e != nil {
		// 一次性：首次访问即销毁
		delete(s.files, tok)
	}
	s.filesMu.Unlock()
	if e == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	cl := e.client
	if cl == nil {
		cl = http.DefaultClient
	}
	resp, err := cl.Get(e.url)
	if err != nil {
		http.Error(w, "download failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for _, k := range []string{
		"Content-Type",
		"Content-Length",
		"Content-Disposition",
	} {
		for _, v := range resp.Header[k] {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// StartLocal 启动仅绑定本机回环的 WSS 服务，返回 (本机地址 host:port, token, error)。
// 该地址仅供同机 daemon 反向代理访问，不直接对外暴露。
func (s *Server) StartLocal() (string, string, error) {
	_, token, err := s.startLoopback()
	if err != nil {
		return "", "", err
	}
	return fmt.Sprintf("127.0.0.1:%d", s.Port), token, nil
}

// startLoopback 绑定本机回环并启动服务。
func (s *Server) startLoopback() (string, string, error) {
	s.Token = generateToken()
	cert, err := generateCert()
	if err != nil {
		return "", "", err
	}

	mux := http.NewServeMux()
	mux.HandleFunc(
		"/ws",
		func(w http.ResponseWriter, r *http.Request) {
			newSession, ok := s.authenticate(r)
			// token 不匹配、已用尽或 session 无效，一律拒绝
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			conn, err := upgrader(w, r)
			if err != nil {
				return
			}
			s.mutex.Lock()
			s.client = conn
			s.mutex.Unlock()
			defer func() {
				s.mutex.Lock()
				s.client = nil
				s.mutex.Unlock()
				conn.Close()
			}()
			if newSession != "" {
				// 首次用 token 登录：把新签发的 session 用 session 子密钥加密后发给客户端
				enc := crypto.EncryptSession(newSession)
				_ = conn.WriteMessage(
					websocket.TextMessage,
					[]byte("SESSION "+enc),
				)
			}
			for {
				_, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				// 缓冲区满时阻塞等待，而不是静默丢弃消息
				select {
				case s.Receive <- string(msg):
				case <-s.done:
					return
				}
			}
		},
	)
	mux.HandleFunc("/dl", s.mediaDownloadHandler)

	listener, port, err := listenPort("127.0.0.1")
	if err != nil {
		return "", "", err
	}
	s.Port = port
	s.Address = "127.0.0.1"

	s.handler = &http.Server{
		Handler:   mux,
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
	}
	go func() {
		_ = s.handler.ServeTLS(listener, "", "")
	}()

	return s.URL(), s.Token, nil
}

// upgrader 创建一个 WebSocket 升级器。
func upgrader(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	u := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	return u.Upgrade(w, r, nil)
}

// authenticate 校验请求：
//   - 首次用 token（?token=）登录：token 匹配且尚未使用则作废 token，并签发新 session；
//   - 之后用 session（?session=）登录：解密并校验 session 是否有效（未过期）。
//
// 返回值 newSession 非空时表示首次 token 登录成功，需把新 session 下发给客户端。
func (s *Server) authenticate(r *http.Request) (newSession string, ok bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	tokenOK := r.URL.Query().Get("token") == s.Token
	if !s.used && tokenOK {
		// 首次 token 登录：作废 token，签发新 session
		s.used = true
		sid := generateToken()
		s.sessions[sid] = time.Now().Add(sessionTTL)
		return sid, true
	}

	// 二次及以后用 session 登录
	enc := r.URL.Query().Get("session")
	if enc == "" {
		return "", false
	}
	sid := s.decryptSession(enc)
	if sid == "" {
		return "", false
	}
	exp, exist := s.sessions[sid]
	if !exist {
		return "", false
	}
	if time.Now().After(exp) {
		// 过期：清除后拒绝
		delete(s.sessions, sid)
		return "", false
	}
	// session 登录成功，不再签发新 session
	return "", true
}

// decryptSession 解密客户端回传的 session。对非法/损坏的输入返回空串，
// 而不是让 crypto.DecryptSession 的 panic 直接崩溃 bot 进程。
func (s *Server) decryptSession(enc string) (sid string) {
	defer func() {
		if recover() != nil {
			sid = ""
		}
	}()
	return crypto.DecryptSession(enc)
}

// Send 向当前已连接的 WSS 客户端发送一条文本消息。
func (s *Server) Send(msg string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.client == nil {
		return
	}
	if err := s.client.WriteMessage(
		websocket.TextMessage,
		[]byte(msg),
	); err != nil {
		fmt.Fprintln(os.Stderr, "发送 WSS 消息失败:", err)
	}
}

// Close 关闭 WSS 服务。
func (s *Server) Close() {
	select {
	case <-s.done:
		return
	default:
		close(s.done)
	}
	if s.handler != nil {
		_ = s.handler.Close()
	}
}
