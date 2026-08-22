package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"os/exec"
)

func ServiceCommand(args []string){
	if len(args)<1{
		fmt.Println(
			"service start|add|stop",
		)
		return
	}

	switch args[0]{
	case "start":
		daemon := exec.Command(
			os.Args[0],
			"service-daemon",
		)
		daemon.Start()
		fmt.Println("Service started")
	case "add":
		id,_:=strconv.ParseInt(
			args[1],
			10,
			64,
		)
		send(
			Request{
				Action:"start",
				ID:id,
			},
		)
	case "stop":
		id,_:=strconv.ParseInt(
			args[1],
			10,
			64,
		)
		send(
			Request{
				Action:"stop",
				ID:id,
			},
		)
	case "daemon-stop":
		send(
			Request{
				Action:"daemon-stop",
			},
		)
	case "list":
		send(
			Request{
				Action:"list",
			},
		)
	}
}

func send(req Request) {
	addr,err:=readServiceAddr()
	if err!=nil {
		fmt.Println(
			"service未启动",
		)
		return
	}
	conn,err:=net.Dial(
		"tcp",
		addr,
	)
	if err!=nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	json.NewEncoder(conn).Encode(req)
	if req.Action=="start" {
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
	if req.Action=="daemon-stop" {
		var bots []BotInfo
		err:=json.NewDecoder(conn).Decode(
			&bots,
		)
		if err==nil && len(bots)>0 {
			fmt.Println(
				"无法关闭 Service，仍有运行中的 Bot:",
			)
			for _,bot:=range bots {
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