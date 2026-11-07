// Package pairing 管理首次配对的一次性配对码。
//
// 配对码在 Service 终端显示（并编码进二维码），60 秒过期、只用一次；
// 二维码里同时带一个 nonce，客户端必须对 nonce 签名，绑定的是待授权公钥
// 而不是码本身（docs/PROTOCOL.md §3/§10）。
package pairing

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// 配对码有效期。
const DefaultTTL = 60 * time.Second

var (
	// ErrNoTicket 表示当前没有待处理的配对码。
	ErrNoTicket = errors.New("pairing: 没有待处理的配对码")
	// ErrExpired 表示配对码已过期。
	ErrExpired = errors.New("pairing: 配对码已过期")
	// ErrUsed 表示配对码已被使用。
	ErrUsed = errors.New("pairing: 配对码已被使用")
	// ErrMismatch 表示配对码或 nonce 不匹配。
	ErrMismatch = errors.New("pairing: 配对码或 nonce 不匹配")
)

// Code 是终端展示给用户的一次性配对信息。
type Code struct {
	Value   string
	Nonce   string
	Expires time.Time
}

type session struct {
	code    string
	nonce   string
	expires time.Time
	used    bool
}

// Store 是配对码的持久化后端。
//
// 有了它，配对码可以跨进程共享：`pair` 在一个容器里生成，
// 正在 `serve` 的另一个容器照样能兑付——Docker 部署必须这样才能配对。
type Store interface {
	SavePendingPair(ctx context.Context, code, nonce string, expires time.Time) error
	LoadPendingPair(ctx context.Context) (code, nonce string, expires time.Time, err error)
	ClearPendingPair(ctx context.Context) error
}

// Pairer 维护当前唯一一个待处理的配对会话。
type Pairer struct {
	mu      sync.Mutex
	now     func() time.Time
	ttl     time.Duration
	current *session
	store   Store
}

// SetStore 让配对码落库，从而在多个进程之间共享。
func (p *Pairer) SetStore(s Store) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.store = s
}

// New 返回一个使用默认有效期（60 秒）的 Pairer。
func New() *Pairer {
	return &Pairer{now: time.Now, ttl: DefaultTTL}
}

// SetClock 替换时钟，仅供测试使用。
func (p *Pairer) SetClock(now func() time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.now = now
}

// Issue 生成新的配对码；已有未过期的码会被替换。
func (p *Pairer) Issue() (Code, error) {
	code, err := randomDigits(6)
	if err != nil {
		return Code{}, err
	}
	nonce, err := randomToken()
	if err != nil {
		return Code{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	expires := p.now().Add(p.ttl)
	p.current = &session{code: code, nonce: nonce, expires: expires}
	if p.store != nil {
		if err := p.store.SavePendingPair(context.Background(), code, nonce, expires); err != nil {
			return Code{}, fmt.Errorf("pairing: 保存配对码: %w", err)
		}
	}
	return Code{Value: code, Nonce: nonce, Expires: expires}, nil
}

// Redeem 校验并消费配对码。成功后再也不会成功第二次。
func (p *Pairer) Redeem(code, nonce string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.store != nil {
		return p.redeemFromStore(code, nonce)
	}
	if p.current == nil {
		return ErrNoTicket
	}
	switch {
	case p.current.used:
		return ErrUsed
	case !p.now().Before(p.current.expires):
		return ErrExpired
	case subtle.ConstantTimeCompare([]byte(code), []byte(p.current.code)) != 1,
		subtle.ConstantTimeCompare([]byte(nonce), []byte(p.current.nonce)) != 1:
		return ErrMismatch
	}
	p.current.used = true
	return nil
}

func (p *Pairer) redeemFromStore(code, nonce string) error {
	gotCode, gotNonce, expires, err := p.store.LoadPendingPair(context.Background())
	if err != nil {
		return ErrNoTicket
	}
	if !p.now().Before(expires) {
		_ = p.store.ClearPendingPair(context.Background())
		return ErrExpired
	}
	if subtle.ConstantTimeCompare([]byte(code), []byte(gotCode)) != 1 ||
		subtle.ConstantTimeCompare([]byte(nonce), []byte(gotNonce)) != 1 {
		return ErrMismatch
	}
	if err := p.store.ClearPendingPair(context.Background()); err != nil {
		return ErrUsed
	}
	return nil
}

// Pending 返回当前配对码的副本（已使用或过期时 ok=false），供终端展示。
func (p *Pairer) Pending() (Code, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.store != nil {
		code, nonce, expires, err := p.store.LoadPendingPair(context.Background())
		if err != nil || !p.now().Before(expires) {
			return Code{}, false
		}
		return Code{Value: code, Nonce: nonce, Expires: expires}, true
	}
	if p.current == nil || p.current.used || !p.now().Before(p.current.expires) {
		return Code{}, false
	}
	return Code{
		Value:   p.current.code,
		Nonce:   p.current.nonce,
		Expires: p.current.expires,
	}, true
}

func randomDigits(n int) (string, error) {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
	v, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("pairing: 生成配对码: %w", err)
	}
	s := v.String()
	for len(s) < n {
		s = "0" + s
	}
	return s, nil
}

func randomToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("pairing: 生成 nonce: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
