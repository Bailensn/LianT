package core

import (
	"context"
	"encoding/json"

	"github.com/Bailensn/LianT/service/internal/protocol"
	"github.com/Bailensn/LianT/service/internal/server"
	"github.com/Bailensn/LianT/service/internal/store"
)

// ListDevices 返回已配对设备，供管理端吊销。
func (c *Core) ListDevices(ctx context.Context, _ server.Session, _ json.RawMessage) (any, *protocol.Error) {
	devs, err := c.opt.Store.Devices(ctx)
	if err != nil {
		return nil, internal(err)
	}
	out := make([]protocol.DeviceInfo, 0, len(devs))
	for _, d := range devs {
		out = append(out, protocol.DeviceInfo{
			DeviceID:   d.ID,
			Name:       d.Name,
			CreatedAt:  d.CreatedAt.UnixMilli(),
			LastSeenAt: d.LastSeenAt.UnixMilli(),
		})
	}
	return protocol.DeviceListOK{Devices: out}, nil
}

// RevokeDevice 删除一台设备；它当前的连接会在下一次写入时被断开。
func (c *Core) RevokeDevice(ctx context.Context, sess server.Session, raw json.RawMessage) (any, *protocol.Error) {
	var req protocol.DeviceRevoke
	if err := json.Unmarshal(raw, &req); err != nil || req.DeviceID == "" {
		return nil, bad(protocol.CodeInvalidPayload, "device.revoke 需要 device_id")
	}
	if req.DeviceID == sess.ManagerID {
		return nil, bad(protocol.CodeInvalidPayload, "不能吊销当前正在使用的设备")
	}
	if _, err := c.opt.Store.Device(ctx, req.DeviceID); err != nil {
		return nil, bad(protocol.CodeInvalidPayload, "设备不存在")
	}
	if err := c.opt.Store.DeleteDevice(ctx, req.DeviceID); err != nil {
		return nil, internal(err)
	}
	// 吊销本身也是一条安全事件。
	_ = c.opt.Store.AddAudit(ctx, store.Audit{
		Kind:     "device.revoke",
		DeviceID: req.DeviceID,
		Name:     "由 " + sess.ManagerID + " 吊销",
	})
	// 直接把吊销后的列表回给调用方，省一次往返。
	return c.ListDevices(ctx, sess, nil)
}

// ListAudit 返回最近的安全事件。
func (c *Core) ListAudit(ctx context.Context, _ server.Session, raw json.RawMessage) (any, *protocol.Error) {
	var req protocol.AuditList
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &req)
	}
	entries, err := c.opt.Store.RecentAudit(ctx, req.Limit)
	if err != nil {
		return nil, internal(err)
	}
	out := make([]protocol.AuditEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, protocol.AuditEntry{
			TS:       e.TS,
			Kind:     e.Kind,
			DeviceID: e.DeviceID,
			Name:     e.Name,
			IP:       e.IP,
			Reason:   e.Reason,
		})
	}
	return protocol.AuditListOK{Entries: out}, nil
}
