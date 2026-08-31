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
	"sync"
	"time"
)

type BotProcess struct {
	ID           int64
	Pid          int
	Addr         string
	WSSAddr      string
	Process      *exec.Cmd
	Stopping     bool
	RestartCount int
	// done 在进程结束后由 waitBot 关闭，用于 stopBot 等待进程退出。
	done chan struct{}
}

var (
	botsMu sync.RWMutex
	bots   = make(
		map[int64]*BotProcess,
	)
	// publicBase 是 daemon 单一安全入口的公开基地址（https://host:port），
	// 由 StartDaemon 在启动时设置，用于给每个 bot 注入 /<id>/ 前缀。
	publicBase string
)

func waitBot(bot *BotProcess) {
	err := bot.Process.Wait()
	botsMu.Lock()
	delete(
		bots,
		bot.ID,
	)
	stopping := bot.Stopping
	botsMu.Unlock()
	close(bot.done)
	if stopping {
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
	botsMu.RLock()
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
	botsMu.RUnlock()
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

// stopExisting 若同一 id 已有运行中的 bot，先尝试停止它，避免重复进程。
func stopExisting(id int64) {
	botsMu.RLock()
	_, ok := bots[id]
	botsMu.RUnlock()
	if ok {
		fmt.Println(
			"该 Bot 已在运行，先停止旧进程:",
			id,
		)
		stopBot(id)
	}
}

// registerBot 在 bots 表中登记一个 bot 并启动 waitBot。
func registerBot(
	id int64,
	pid int,
	addr string,
	wssAddr string,
	cmd *exec.Cmd,
) {
	bot := &BotProcess{
		ID:       id,
		Pid:      pid,
		Addr:     addr,
		WSSAddr:  wssAddr,
		Process:  cmd,
		Stopping: false,
		done:     make(chan struct{}),
	}
	botsMu.Lock()
	bots[id] = bot
	botsMu.Unlock()
	fmt.Println(
		"Bot启动:",
		id,
		"PID:",
		pid,
		"IPC:",
		addr,
		"WSS:",
		wssAddr,
	)
	go waitBot(bot)
}

// readBotAddrs 从子进程 stdout 读取前两行：
// 第 1 行为 IPC 地址，第 2 行为本机回环 WSS 地址。
func readBotAddrs(stdout io.Reader) (string, string, error) {
	reader := bufio.NewReader(
		stdout,
	)
	ipc, err := reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	wssLine, err := reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(ipc), strings.TrimSpace(wssLine), nil
}

// publicBaseFor 组装某个 bot 的公开基地址：https://host:port/<id>。
func publicBaseFor(id int64) string {
	if publicBase == "" {
		return ""
	}
	return publicBase + "/" + strconv.FormatInt(id, 10)
}

func startBot(id int64) {
	cmd := exec.Command(
		os.Args[0],
		"botstart",
		strconv.FormatInt(id, 10),
	)
	cmd.Env = append(
		os.Environ(),
		"LIANT_PUBLIC_BASE="+publicBaseFor(id),
	)
	stopExisting(id)

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
	ipcAddr, wssAddr, err := readBotAddrs(stdout)
	if err != nil {
		fmt.Println("读取 Bot 地址失败，结束进程:", err)
		_ = cmd.Process.Kill()
		return
	}
	registerBot(id, cmd.Process.Pid, ipcAddr, wssAddr, cmd)
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
	cmd.Env = append(
		os.Environ(),
		"LIANT_PUBLIC_BASE="+publicBaseFor(id),
	)
	stopExisting(id)

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
	ipcAddr, wssAddr, err := readBotAddrs(stdout)
	if err != nil {
		fmt.Println("读取 Bot 地址失败，结束进程:", err)
		_ = cmd.Process.Kill()
		return
	}
	registerBot(id, cmd.Process.Pid, ipcAddr, wssAddr, cmd)
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

// stopBot 向 bot 的 IPC 端口发送停止请求并等待其退出；
// 若 IPC 不可达或超时未退出，则强制结束进程。
func stopBot(id int64) {
	botsMu.RLock()
	bot, ok := bots[id]
	botsMu.RUnlock()
	if !ok {
		fmt.Println(
			"Bot不存在",
		)
		return
	}
	botsMu.Lock()
	bot.Stopping = true
	botsMu.Unlock()
	conn, err := net.Dial(
		"tcp",
		bot.Addr,
	)
	if err != nil {
		fmt.Println("IPC 连接失败，强制结束:", err)
		_ = bot.Process.Process.Kill()
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
	select {
	case <-bot.done:
		fmt.Println(
			"Bot已正常退出:",
			id,
		)
	case <-time.After(5 * time.Second):
		fmt.Println(
			"Bot未在超时内退出，强制结束:",
			id,
		)
		_ = bot.Process.Process.Kill()
	}
}
