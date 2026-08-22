package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type BotProcess struct {
	ID           int64
	Pid          int
	Addr         string
	Process      *exec.Cmd
	Stopping     bool
	RestartCount int
}

var bots = make(
	map[int64]*BotProcess,
)

func waitBot(bot *BotProcess) {
	err := bot.Process.Wait()
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
		time.Second * 3,
	)
	startBot(
		bot.ID,
	)
}

func sendBotList(conn net.Conn) {
	list := make(
		[]BotInfo,
		0,
	)
	for _, bot := range bots {
		list = append(
			list,
			BotInfo{
				ID:   bot.ID,
				PID:  bot.Pid,
				Addr: bot.Addr,
			},
		)
	}
	json.NewEncoder(conn).Encode(
		list,
	)
}

// botLog 为指定 bot 打开 logs/{id}.log 追加文件（路径相对于进程工作目录）。
func botLog(id int64) (*os.File, error) {
	if err := os.MkdirAll("logs", 0755); err != nil {
		return nil, err
	}
	return os.OpenFile(
		filepath.Join("logs", strconv.FormatInt(id, 10)+".log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
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
	logf, err := botLog(id)
	if err != nil {
		fmt.Println("打开日志失败:", err)
		return
	}
	defer logf.Close() // 子进程在 Start 时已复制 fd，关闭父端引用即可
	cmd.Stderr = logf
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
		ID:       id,
		Pid:      cmd.Process.Pid,
		Addr:     addr,
		Process:  cmd,
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
// 之后的 stderr（如消息日志）写入 logs/{id}.log，不再污染 daemon 输出。
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
		ID:       id,
		Pid:      cmd.Process.Pid,
		Addr:     addr,
		Process:  cmd,
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
	go echoStderr(id, stderr, conn)
}

// echoStderr 将子进程 stderr 转发到 conn，读到 WSS_READY 后关闭 conn；
// 关闭之后继续消费 stderr 剩余内容并写入 logs/{id}.log，避免管道阻塞。
func echoStderr(id int64, stderr io.Reader, conn net.Conn) {
	rd := bufio.NewReader(stderr)
	var logf *os.File
	defer func() {
		if logf != nil {
			logf.Close()
		}
	}()
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			break
		}
		if logf != nil {
			fmt.Fprint(logf, line)
			continue
		}
		_, werr := conn.Write(
			[]byte(line),
		)
		if strings.TrimSpace(line) == "WSS_READY" {
			lf, lerr := botLog(id)
			if lerr != nil {
				lf = os.Stderr // 兜底：日志目录失败时退回 daemon stderr
			}
			logf = lf
			conn.Close()
		}
		if werr != nil && logf == nil {
			// 前端已断开，且尚未进入日志阶段：把后续内容写入日志文件
			lf, lerr := botLog(id)
			if lerr != nil {
				lf = os.Stderr
			}
			logf = lf
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
