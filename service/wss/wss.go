package wss

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Server 是可供每个 bot 进程独立持有一份的 WSS 服务。
type Server struct {
	client *websocket.Conn
	mutex  sync.Mutex

	Token   string
	Address string
	Port    int
	used    bool

	// Receive 用于接收来自 WSS 客户端的文本消息。
	Receive chan string

	handler *http.Server
	done    chan struct{}
}

// New 创建一个空的 WSS 服务实例。
func New() *Server {
	return &Server{
		Receive: make(chan string, 32),
		done:    make(chan struct{}),
	}
}

// URL 返回 WSS 连接地址，例如 wss://[::1]:50039/ws。
func (s *Server) URL() string {
	return fmt.Sprintf(
		"wss://%s:%d/ws",
		s.Address,
		s.Port,
	)
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

// resolveAddress 决定对外通告的地址：优先 IPv6，其次公网 IPv4，
// 再退回到本机 IPv4；全部失败时使用 localhost。
func resolveAddress() string {
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

// Start 启动 WSS 服务并返回 (地址, token, 错误)。
func (s *Server) Start() (string, string, error) {
	s.Token = generateToken()
	cert, err := generateCert()
	if err != nil {
		return "", "", err
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc(
		"/ws",
		func(w http.ResponseWriter, r *http.Request) {
			s.mutex.Lock()
			used := s.used
			valid := r.URL.Query().Get("token") == s.Token
			s.mutex.Unlock()
			// token 不匹配或已被一次性使用，一律拒绝
			if used || !valid {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			s.mutex.Lock()
			s.client = conn
			// 连接成功后立即作废 token，防止被二次连接
			s.used = true
			s.mutex.Unlock()
			defer func() {
				s.mutex.Lock()
				s.client = nil
				s.mutex.Unlock()
				conn.Close()
			}()
			for {
				_, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				select {
				case s.Receive <- string(msg):
				case <-s.done:
					return
				default:
				}
			}
		},
	)

	var port int
	var listener net.Listener
	for {
		port = randomPort()
		listener, err = net.Listen(
			"tcp",
			fmt.Sprintf("[::]:%d", port),
		)
		if err == nil {
			break
		}
	}
	s.Port = port
	s.Address = resolveAddress()

	s.handler = &http.Server{
		Handler:   mux,
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
	}
	go func() {
		_ = s.handler.ServeTLS(listener, "", "")
	}()

	return s.URL(), s.Token, nil
}

// Send 向当前已连接的 WSS 客户端发送一条文本消息。
func (s *Server) Send(msg string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.client == nil {
		return
	}
	_ = s.client.WriteMessage(
		websocket.TextMessage,
		[]byte(msg),
	)
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