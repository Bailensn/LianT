package service

import (
	"encoding/json"
	"fmt"
	"net"
)

func StartDaemon() {
	listener,err:=net.Listen(
		"tcp",
		"127.0.0.1:0",
	)
	if err!=nil{
		panic(err)
	}
	defer listener.Close()
	err=saveServiceAddr(
		listener.Addr().String(),
	)
	if err!=nil{
		panic(err)
	}
	fmt.Println(
		"Service running:",
		listener.Addr(),
	)
	running:=true
	for running{
		conn,err:=listener.Accept()
		if err!=nil{
			break
		}
		go func(){
			stop:=handle(conn)
			if stop{
				running=false
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
	ID int64 `json:"id"`
}

func handle(conn net.Conn) bool {
	defer conn.Close()
	var req Request
	err:=json.NewDecoder(conn).Decode(&req)
	if err!=nil{
		return false
	}
	switch req.Action {
	case "start":
		startBot(req.ID)
	case "stop":
		stopBot(req.ID)
	case "list":
		sendBotList(conn)
	case "daemon-stop":
		if len(bots)>0 {
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