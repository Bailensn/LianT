package platform_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Bailensn/LianT/service/internal/platform"
)

type fakeAdapter struct {
	name string
}

func (f fakeAdapter) Name() string { return f.name }

func (fakeAdapter) Validate(context.Context, platform.Credentials) (platform.Account, error) {
	return platform.Account{ID: "10001", Username: "demo_bot", Caps: []string{platform.CapText}}, nil
}

func (fakeAdapter) Run(context.Context, platform.Account, platform.Credentials, platform.Hooks) error {
	return nil
}

func (fakeAdapter) SendText(
	context.Context,
	platform.Account,
	platform.Credentials,
	platform.Peer,
	string,
) (platform.MessageID, error) {
	return "m-1", nil
}

func (fakeAdapter) Format() string { return "qq-plain" }

var _ platform.Adapter = fakeAdapter{}

func TestIDRoundTrip(t *testing.T) {
	cases := []struct {
		id       string
		platform string
		external string
	}{
		{"telegram:123456", "telegram", "123456"},
		{"qqbot:ABC_def-1", "qqbot", "ABC_def-1"},
		{"discord:123456789012345678", "discord", "123456789012345678"},
		{"qqbot:x", "qqbot", "x"},
	}

	for _, c := range cases {
		gotPlatform, gotExternal, err := platform.SplitID(c.id)
		if err != nil {
			t.Fatalf("SplitID(%q) 返回错误: %v", c.id, err)
		}
		if gotPlatform != c.platform || gotExternal != c.external {
			t.Fatalf("SplitID(%q) = (%q, %q)，期望 (%q, %q)",
				c.id, gotPlatform, gotExternal, c.platform, c.external)
		}
		if rebuilt := platform.MakeID(gotPlatform, gotExternal); rebuilt != c.id {
			t.Fatalf("MakeID 重建得到 %q，期望 %q", rebuilt, c.id)
		}
		if !platform.IsValidID(c.id) {
			t.Fatalf("IsValidID(%q) = false", c.id)
		}
	}
}

func TestIDInvalid(t *testing.T) {
	cases := []string{
		"",
		"telegram",
		":1",
		"telegram:",
		"Telegram:1",
		"t:1",
		"telegram:abc def",
		"telegram:" + strings.Repeat("a", 65),
	}

	for _, id := range cases {
		if platform.IsValidID(id) {
			t.Errorf("IsValidID(%q) = true，期望 false", id)
		}
		if _, _, err := platform.SplitID(id); err == nil {
			t.Errorf("SplitID(%q) 没报错", id)
		} else if !errors.Is(err, platform.ErrInvalidID) {
			t.Errorf("SplitID(%q) 的错误没有包装 ErrInvalidID: %v", id, err)
		}
	}
}

func TestRegistryRegisterAndGet(t *testing.T) {
	platform.Register(fakeAdapter{name: "tstreg"})

	got, ok := platform.Get("tstreg")
	if !ok {
		t.Fatal("注册后 Get 返回 ok=false")
	}
	if got.Name() != "tstreg" {
		t.Fatalf("Get 返回的适配器名为 %q", got.Name())
	}

	if _, ok := platform.Get("missing"); ok {
		t.Fatal("未注册的平台 Get 返回 ok=true")
	}

	if names := platform.Names(); !slices.Contains(names, "tstreg") {
		t.Fatalf("Names() 里没有 tstreg: %v", names)
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	platform.Register(fakeAdapter{name: "tstdup"})

	defer func() {
		if recover() == nil {
			t.Fatal("重复注册没有 panic")
		}
	}()
	platform.Register(fakeAdapter{name: "tstdup"})
}

func TestRegisterInvalidNamePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("非法平台名没有 panic")
		}
	}()
	platform.Register(fakeAdapter{name: "Qq"})
}
