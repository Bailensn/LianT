package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const daemonEnv = "LIANT_DAEMON"

func isDaemonChild() bool { return os.Getenv(daemonEnv) == "1" }

func pidFilePath() string { return filepath.Join("data", "liant.pid") }

func writePidFile(pid int) error {
	if err := os.MkdirAll("data", 0o700); err != nil {
		return err
	}
	return os.WriteFile(pidFilePath(), []byte(strconv.Itoa(pid)+"\n"), 0o600)
}

func readPidFile() (int, error) {
	raw, err := os.ReadFile(pidFilePath())
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return 0, fmt.Errorf("pid 文件损坏: %w", err)
	}
	return pid, nil
}

func removePidFile() { _ = os.Remove(pidFilePath()) }

func stopCommand() {
	if err := stopDaemon(); err != nil {
		fmt.Fprintln(os.Stderr, "停止失败：", err)
		os.Exit(1)
	}
	fmt.Println("已停止")
}
