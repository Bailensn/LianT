package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
)

func StartDaemon() {
	// 启动 daemon 的单一安全入口：在 50000-60000 随机选一个可用端口。
	base, err := startWSSProxy()
	if err != nil {
		panic(err)
	}
	publicBase = base
	fmt.Println(
		"安全入口:",
		base,
	)
	listener, err := net.Listen(
		"tcp",
		"127.0.0.1:0",
	)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	err = saveServiceAddr(
		listener.Addr().String(),
	)
	if err != nil {
		panic(err)
	}
	fmt.Println(
		"Service running:",
		listener.Addr(),
	)
	running := true
	for running {
		conn, err := listener.Accept()
		if err != nil {
			break
		}
		go func() {
			stop := handle(conn)
			if stop {
				running = false
				listener.Close()
			}
		}()
	}
	removeServiceAddr()
	fmt.Println(
		"Service stopped",
	)
}

type Request struct {
	Action string `json:"action"`
	ID     int64  `json:"id"`
}

func handle(conn net.Conn) bool {
	defer conn.Close()
	var req Request
	err := json.NewDecoder(conn).Decode(&req)
	if err != nil {
		return false
	}
	switch req.Action {
	case "start":
		startBotEcho(req.ID, conn)
		// 保持连接，直到 bot 的 WSS 信息回显完成并关闭连接
		io.Copy(io.Discard, conn)
	case "stop":
		stopBot(req.ID)
	case "list":
		sendBotList(conn)
	case "daemon-stop":
		botsMu.RLock()
		hasBots := len(bots) > 0
		botsMu.RUnlock()
		if hasBots {
			sendBotList(conn)
			return false
		}
		fmt.Println(
			"收到daemon停止请求",
		)
		return true
	}
	return false
}
