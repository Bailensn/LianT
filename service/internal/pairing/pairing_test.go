package pairing_test

import (
	"errors"
	"testing"
	"time"

	"github.com/Bailensn/LianT/service/internal/pairing"
)

func TestIssueAndRedeem(t *testing.T) {
	p := pairing.New()
	code, err := p.Issue()
	if err != nil {
		t.Fatalf("生成配对码: %v", err)
	}
	if len(code.Value) != 6 {
		t.Fatalf("配对码应为 6 位: %q", code.Value)
	}
	if code.Nonce == "" {
		t.Fatal("nonce 为空")
	}
	if d := time.Until(code.Expires); d <= 0 || d > pairing.DefaultTTL+time.Second {
		t.Fatalf("有效期不符: %s", d)
	}
	if pending, ok := p.Pending(); !ok || pending.Value != code.Value {
		t.Fatalf("Pending 不符: %+v ok=%v", pending, ok)
	}

	if err := p.Redeem(code.Value, code.Nonce); err != nil {
		t.Fatalf("首次兑换失败: %v", err)
	}
	if err := p.Redeem(code.Value, code.Nonce); !errors.Is(err, pairing.ErrUsed) {
		t.Fatalf("二次兑换应返回 ErrUsed，得到 %v", err)
	}
}

func TestRedeemRejectsMismatch(t *testing.T) {
	p := pairing.New()
	code, _ := p.Issue()

	if err := p.Redeem("000000", code.Nonce); !errors.Is(err, pairing.ErrMismatch) {
		t.Fatalf("错误配对码应返回 ErrMismatch，得到 %v", err)
	}
	if err := p.Redeem(code.Value, "wrong-nonce"); !errors.Is(err, pairing.ErrMismatch) {
		t.Fatalf("错误 nonce 应返回 ErrMismatch，得到 %v", err)
	}
	// 校验失败不应消费掉码。
	if err := p.Redeem(code.Value, code.Nonce); err != nil {
		t.Fatalf("正确的码应仍可兑换: %v", err)
	}
}

func TestRedeemExpired(t *testing.T) {
	p := pairing.New()
	now := time.Now()
	p.SetClock(func() time.Time { return now })
	code, _ := p.Issue()

	now = now.Add(pairing.DefaultTTL + time.Second)
	if err := p.Redeem(code.Value, code.Nonce); !errors.Is(err, pairing.ErrExpired) {
		t.Fatalf("过期应返回 ErrExpired，得到 %v", err)
	}
	if _, ok := p.Pending(); ok {
		t.Fatal("过期后不应再有 Pending")
	}
}

func TestRedeemWithoutTicket(t *testing.T) {
	p := pairing.New()
	if err := p.Redeem("123456", "n"); !errors.Is(err, pairing.ErrNoTicket) {
		t.Fatalf("无码时应返回 ErrNoTicket，得到 %v", err)
	}
}

func TestIssueReplacesOldCode(t *testing.T) {
	p := pairing.New()
	first, _ := p.Issue()
	second, _ := p.Issue()

	if err := p.Redeem(first.Value, first.Nonce); !errors.Is(err, pairing.ErrMismatch) {
		t.Fatalf("旧码应失效，得到 %v", err)
	}
	if err := p.Redeem(second.Value, second.Nonce); err != nil {
		t.Fatalf("新码应可兑换: %v", err)
	}
}
