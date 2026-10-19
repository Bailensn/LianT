package ipc

import (
	"encoding/json"
	"fmt"
	"net"
	"qbot/internal/cfg"
	"time"
)

// ============================================================================
// 主进程 <-> 任务进程 的指令协议
//
// 一条 TCP 短连接 = 一条指令：客户端写一个 JSON 请求，服务端回一个 JSON 响应。
// 端口只绑 127.0.0.1，仅本机可连；协议很土但足够稳，不引入任何依赖。
// ============================================================================

type Request struct {
	V    int               `json:"v"`
	Cmd  string            `json:"cmd"`
	Args map[string]string `json:"args,omitempty"`
}

type Response struct {
	OK   bool              `json:"ok"`
	Msg  string            `json:"msg,omitempty"`
	Data map[string]string `json:"data,omitempty"`
}

// CallAt 向指定控制地址发一条指令，timeout 内拿到回复。
func CallAt(addr, cmd string, args map[string]string, timeout time.Duration) (*Response, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	if err := json.NewEncoder(conn).Encode(Request{V: cfg.ProtocolVersion, Cmd: cmd, Args: args}); err != nil {
		return nil, err
	}
	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// FindDaemonAddr 找到任务进程的控制地址。
// 先用当前配置的端口探测，失败再看 runtime 文件里记录的端口（主进程与任务进程
// 的 QBOT_PORT 不一致时也能找到）。
func FindDaemonAddr() (string, bool) {
	if resp, err := CallAt(cfg.ControlAddr(), "ping", nil, 1200*time.Millisecond); err == nil && resp.OK {
		return cfg.ControlAddr(), true
	}
	if info, ok := cfg.ReadRuntimeInfo(); ok {
		addr := fmt.Sprintf("127.0.0.1:%d", info.Port)
		if addr != cfg.ControlAddr() {
			if resp, err := CallAt(addr, "ping", nil, 1200*time.Millisecond); err == nil && resp.OK {
				return addr, true
			}
		}
	}
	return cfg.ControlAddr(), false
}

// DaemonStatus 任务进程是否在跑；在跑则返回它的状态数据。
func DaemonStatus() (bool, string, map[string]string) {
	addr, ok := FindDaemonAddr()
	if !ok {
		return false, cfg.ControlAddr(), nil
	}
	resp, err := CallAt(addr, "status", nil, 5*time.Second)
	if err != nil || resp == nil {
		return true, addr, nil
	}
	return true, addr, resp.Data
}
