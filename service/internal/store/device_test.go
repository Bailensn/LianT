package store

import (
	"context"
	"errors"
	"testing"
)

func TestDeviceLifecycle(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, err := s.Device(ctx, "dev-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("空库应返回 ErrNotFound，得到 %v", err)
	}

	pub := []byte{0x30, 0x59, 0x30, 0x13}
	if err := s.PutDevice(ctx, Device{ID: "dev-1", Name: "Pixel", PublicKey: pub}); err != nil {
		t.Fatalf("登记设备: %v", err)
	}
	if err := s.PutDevice(ctx, Device{ID: "dev-1", Name: "Pixel 9", PublicKey: pub}); err != nil {
		t.Fatalf("更新设备: %v", err)
	}

	d, err := s.Device(ctx, "dev-1")
	if err != nil {
		t.Fatalf("读取设备: %v", err)
	}
	if d.Name != "Pixel 9" || string(d.PublicKey) != string(pub) {
		t.Fatalf("设备字段不符: %+v", d)
	}
	if d.CreatedAt.IsZero() {
		t.Fatal("created_at 未写入")
	}

	if n, err := s.CountDevices(ctx); err != nil || n != 1 {
		t.Fatalf("设备数不符: %d err=%v", n, err)
	}

	if err := s.TouchDevice(ctx, "dev-1"); err != nil {
		t.Fatalf("更新最后在线: %v", err)
	}
	d, _ = s.Device(ctx, "dev-1")
	if d.LastSeenAt.IsZero() {
		t.Fatal("last_seen_at 未写入")
	}

	list, err := s.Devices(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("列设备: %v len=%d", err, len(list))
	}

	if err := s.DeleteDevice(ctx, "dev-1"); err != nil {
		t.Fatalf("删除设备: %v", err)
	}
	if _, err := s.Device(ctx, "dev-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("删除后仍能读到: %v", err)
	}
	if err := s.DeleteDevice(ctx, "dev-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("重复删除应返回 ErrNotFound: %v", err)
	}
}

func TestPutDeviceRejectsBadInput(t *testing.T) {
	s := newTestStore(t)
	if err := s.PutDevice(context.Background(), Device{ID: "", PublicKey: []byte("x")}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("空 device_id 未被拒绝: %v", err)
	}
	if err := s.PutDevice(context.Background(), Device{ID: "dev-1"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("空公钥未被拒绝: %v", err)
	}
}
