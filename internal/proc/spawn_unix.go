//go:build !windows

package proc

import (
	"os"
	"os/signal"
	"syscall"
)

// SpawnDetached 以"独立会话 + 后台"的方式拉起任务进程：
//   - setsid：脱离当前终端的会话/进程组，主进程或终端退出都不会带走它；
//   - stdout/stderr 追加到日志文件，保证启动早期的输出（含 panic）也不会丢；
//   - 继承环境变量，所以 QBOT_APP_ID 之类的配置能传下去。
func SpawnDetached(exe string, args []string, logFile string) (int, error) {
	logF, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, err
	}
	defer logF.Close()

	devNull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		return 0, err
	}
	defer devNull.Close()

	proc, err := os.StartProcess(exe, append([]string{exe}, args...), &os.ProcAttr{
		Dir:   WorkDir(),
		Env:   os.Environ(),
		Files: []*os.File{devNull, logF, logF},
		Sys:   &syscall.SysProcAttr{Setsid: true},
	})
	if err != nil {
		return 0, err
	}
	pid := proc.Pid
	_ = proc.Release()
	return pid, nil
}

// IgnoreHangup 让任务进程忽略终端挂断信号（关终端 = 不影响任务进程）。
func IgnoreHangup() {
	signal.Ignore(syscall.SIGHUP)
}

func WorkDir() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}
