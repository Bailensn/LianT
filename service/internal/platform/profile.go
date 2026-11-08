package platform

import (
	"context"
	"errors"
)

// ErrProfileForbidden 表示平台拒绝读取会话资料，通常是需要白名单授权。
var ErrProfileForbidden = errors.New("platform: 无权读取会话资料")

// PeerProfile 是会话级别的资料，目前只有标题。
type PeerProfile struct {
	Title string
}

// PeerProfiler 是可选能力：按会话 ID 反查平台侧资料。
//
// 群名这类信息要平台额外授权，没有权限时返回 ErrProfileForbidden，
// 由调用方决定怎么提示用户，而不是当成失败吞掉。
type PeerProfiler interface {
	PeerProfile(ctx context.Context, creds Credentials, peerID string) (PeerProfile, error)
}
