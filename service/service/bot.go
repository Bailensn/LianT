package service

import (
	"fmt"
	"net"
	"os"
)

import (
	"github.com/LensnTeam/LianT/service/botruntime"
	"github.com/LensnTeam/LianT/service/wss"
)

// BotCommand 启动一个 bot 进程：
//   - stdout 第 1 行：IPC 地址（供 daemon 发送停止指令）
//   - stdout 第 2 行：本机回环 WSS 地址（供 daemon 反向代理）
//   - stderr：WSS_URL <客户端可达入口>、WSS_READY
//
// bot 的 WSS 只绑定本机回环，不直接对外暴露公网端口；
// 对外统一由 daemon 的单一安全入口按 /<id>/ 路由。
func BotCommand(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "缺少 Bot ID")
		return
	}
	id := args[0]
	listener, err := net.Listen(
		"tcp",
		"127.0.0.1:0",
	)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	// stdout 第 1 行：IPC 地址
	fmt.Println(
		listener.Addr().String(),
	)

	ws := wss.New()
	// 由 daemon 注入对外公开基地址（https://host:port/<id>）
	ws.SetPublicBase(
		os.Getenv("LIANT_PUBLIC_BASE"),
	)
	// 仅绑定本机回环：对外不影响公网端口
	localAddr, wsToken, err := ws.StartLocal()
	// stdout 第 2 行：本机 WSS 地址（失败时为空，避免 daemon 读第二行时阻塞）
	fmt.Println(localAddr)
	if err != nil {
		fmt.Fprintln(
			os.Stderr,
			"Bot",
			id,
			"启动WSS失败:",
			err,
		)
	} else {
		fmt.Fprintf(
			os.Stderr,
			"WSS_URL %s?token=%s\n",
			ws.URL(),
			wsToken,
		)
		fmt.Fprintln(
			os.Stderr,
			"WSS_READY",
		)
	}
	go botruntime.Run(
		id,
		ws,
	)
	for {
		conn, err := listener.Accept()
		if err != nil {
			// 监听器关闭（接收到停止请求后会退出），停止忙轮询
			break
		}
		buf := make([]byte, 32)
		n, _ := conn.Read(buf)
		if string(buf[:n]) == "stop" {
			ws.Close()
			fmt.Println(
				"Bot",
				id,
				"退出",
			)
			cleanup()
			conn.Close()
			return
		}
		conn.Close()
	}
}

func cleanup() {
	fmt.Println(
		"保存数据...",
	)
}
