//go:build unix

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// startDaemon 以新会话重新拉起自己，脱离当前终端；日志写 data/liant.log。
func startDaemon(args []string) error {
	logPath := filepath.Join("data", "liant.log")
	if err := os.MkdirAll("data", 0o700); err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer logFile.Close()

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), daemonEnv+"=1")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := writePidFile(cmd.Process.Pid); err != nil {
		return err
	}
	fmt.Printf("已转入后台，pid %d，日志 %s\n", cmd.Process.Pid, logPath)
	return nil
}

func stopDaemon() error {
	pid, err := readPidFile()
	if err != nil {
		return fmt.Errorf("没有找到运行中的服务: %w", err)
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		return err
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err != nil {
			removePidFile()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("进程 %d 未在 5 秒内退出", pid)
}
