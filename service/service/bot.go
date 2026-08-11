package service

import (
	"fmt"
	"net"
)

func BotCommand(args []string){
	id:=args[0]
	listener,err:=net.Listen(
		"tcp",
		"127.0.0.1:0",
	)
	if err!=nil{
		panic(err)
	}
	defer listener.Close()
	fmt.Println(
		listener.Addr().String(),
	)
	go messageLoop(id)
	for{
		conn,err:=listener.Accept()
		if err!=nil{
			continue
		}
		buf:=make([]byte,32)
		n,_:=conn.Read(buf)
		if string(buf[:n])=="stop"{
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

func messageLoop(id string){
	for{
		// Telegram消息处理
	}
}

func cleanup(){
	fmt.Println(
		"保存数据...",
	)
}