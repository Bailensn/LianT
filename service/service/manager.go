package service

import (
	"bufio"
	"fmt"
	"io"
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

// startBotEcho 启动一个 bot 进程，并把它的 stderr（含 WSS 地址与 TOKEN）
// 逐行转发到请求连接 conn，用于回显给前端。
// 读到 WSS_READY 标记后关闭 conn，通知前端回显结束；
// 之后的 stderr（如消息日志）转写到 daemon 自己的 stderr，防止管道阻塞。
func startBotEcho(id int64, conn net.Conn) {
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
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Println(err)
		return
	}
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
	go echoStderr(stderr, conn)
}

// echoStderr 将子进程 stderr 转发到 conn，读到 WSS_READY 后关闭 conn；
// 关闭之后继续消费 stderr 剩余内容并写到 daemon 标准错误。
func echoStderr(stderr io.Reader, conn net.Conn) {
	rd := bufio.NewReader(stderr)
	done := false
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			break
		}
		if done {
			fmt.Fprint(os.Stderr, line)
			continue
		}
		_, werr := conn.Write(
			[]byte(line),
		)
		if strings.TrimSpace(line) == "WSS_READY" {
			done = true
			conn.Close()
		}
		if werr != nil {
			// 前端已断开，之后的 stderr 转写自身标准错误
			done = true
		}
	}
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