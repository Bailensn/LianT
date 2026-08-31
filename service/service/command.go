package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
)

func ServiceCommand(args []string) {
	if len(args) < 1 {
		fmt.Println(
			"service start|add <id>|stop <id>|list|daemon-stop",
		)
		return
	}

	switch args[0] {
	case "start":
		daemon := exec.Command(
			os.Args[0],
			"service-daemon",
		)
		if err := daemon.Start(); err != nil {
			fmt.Println("启动 Service 失败:", err)
			return
		}
		// 后台回收子进程，避免产生僵尸进程
		go func() {
			_ = daemon.Wait()
		}()
		fmt.Println("Service started")
	case "add":
		id, ok := parseID(args)
		if !ok {
			return
		}
		send(
			Request{
				Action: "start",
				ID:     id,
			},
		)
	case "stop":
		id, ok := parseID(args)
		if !ok {
			return
		}
		send(
			Request{
				Action: "stop",
				ID:     id,
			},
		)
	case "daemon-stop":
		send(
			Request{
				Action: "daemon-stop",
			},
		)
	case "list":
		send(
			Request{
				Action: "list",
			},
		)
	default:
		fmt.Println("未知命令")
	}
}

// parseID 解析 args[1] 为 bot id，并校验参数数量与格式。
func parseID(args []string) (int64, bool) {
	if len(args) != 2 {
		fmt.Println(
			"需要指定 Bot ID",
		)
		return 0, false
	}
	id, err := strconv.ParseInt(
		args[1],
		10,
		64,
	)
	if err != nil {
		fmt.Println(
			"Bot ID 必须是数字:",
			args[1],
		)
		return 0, false
	}
	return id, true
}

func send(req Request) {
	addr, err := readServiceAddr()
	if err != nil {
		fmt.Println(
			"service未启动",
		)
		return
	}
	conn, err := net.Dial(
		"tcp",
		addr,
	)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	json.NewEncoder(conn).Encode(req)
	if req.Action == "start" {
		// 回显 bot 的 WSS 信息，直到 daemon 关闭连接（WSS_READY 之后）
		reader := bufio.NewReader(conn)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			fmt.Print(line)
		}
	}
	if req.Action == "daemon-stop" {
		var bots []BotInfo
		err := json.NewDecoder(conn).Decode(
			&bots,
		)
		if err == nil && len(bots) > 0 {
			fmt.Println(
				"无法关闭 Service，仍有运行中的 Bot:",
			)
			for _, bot := range bots {
				fmt.Println(
					"ID:",
					bot.ID,
					"PID:",
					bot.PID,
					"IPC:",
					bot.Addr,
				)
			}
			return
		}
		fmt.Println(
			"Service已关闭",
		)
	}
}
