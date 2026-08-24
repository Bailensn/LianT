package service

import (
	"fmt"
	"net"
	"os"
)

import (
	"github.com/LensnTeam/LianT/service/wss"
	"github.com/LensnTeam/LianT/service/botruntime"
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
	// 每个 bot 进程独立启动一份 WSS 服务
	ws:=wss.New()
	wsURL,wsToken,err:=ws.Start()
	if err!=nil{
		fmt.Fprintln(
			os.Stderr,
			"Bot",
			id,
			"启动WSS失败:",
			err,
		)
	}else{
		fmt.Fprintf(
			os.Stderr,
			"WSS:\nwss://%s:%d/ws\n\nTOKEN:\n%s\n\n",
			ws.Address,
			ws.Port,
			wsToken,
		)
		fmt.Fprintln(
			os.Stderr,
			"WSS_READY",
		)
		_ = wsURL
	}
	go botruntime.Run(
		id,
		ws,
	)
	for{
		conn,err:=listener.Accept()
		if err!=nil{
			continue
		}
		buf:=make([]byte,32)
		n,_:=conn.Read(buf)
		if string(buf[:n])=="stop"{
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

func cleanup(){
	fmt.Println(
		"保存数据...",
	)
}