package service

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"encoding/json"
	"strings"
	"time"
)

type BotProcess struct {
	ID int64
	Pid int
	Addr string
	Process *exec.Cmd
	Stopping bool
	RestartCount int
}

var bots = make(
	map[int64]*BotProcess,
)

func waitBot(bot *BotProcess){
	err:=bot.Process.Wait()
	delete(
		bots,
		bot.ID,
	)
	if bot.Stopping {
		fmt.Println(
			"Bot正常停止:",
			bot.ID,
		)
		return
	}
	fmt.Println(
		"Bot异常退出:",
		bot.ID,
		err,
	)
	time.Sleep(
		time.Second*3,
	)
	startBot(
		bot.ID,
	)
}

func sendBotList(conn net.Conn){
	list:=make(
		[]BotInfo,
		0,
	)
	for _,bot:=range bots{
		list=append(
			list,
			BotInfo{
				ID:bot.ID,
				PID:bot.Pid,
				Addr:bot.Addr,
			},
		)
	}
	json.NewEncoder(conn).Encode(
		list,
	)
}

func startBot(id int64) {
	cmd := exec.Command(
		os.Args[0],
		"botstart",
		strconv.FormatInt(id, 10),
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println(err)
		return
	}
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		fmt.Println(err)
		return
	}
	reader := bufio.NewReader(stdout)
	addr, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println(err)
		return
	}
	addr = strings.TrimSpace(addr)
	bot := &BotProcess{
		ID: id,
		Pid: cmd.Process.Pid,
		Addr: addr,
		Process: cmd,
		Stopping: false,

	}
	bots[id] = bot
	fmt.Println(
		"Bot启动:",
		id,
		"PID:",
		bot.Pid,
		"IPC:",
		bot.Addr,
	)
	go waitBot(bot)
}

func stopBot(id int64) {
	bot, ok := bots[id]
	if !ok {
		fmt.Println(
			"Bot不存在",
		)
		return
	}
	bot.Stopping = true
	conn, err := net.Dial(
		"tcp",
		bot.Addr,
	)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	_, err = conn.Write(
		[]byte("stop"),
	)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(
		"停止请求发送:",
		id,
	)
}