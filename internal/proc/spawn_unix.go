//go:build !windows

package proc

import (
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

// SpawnDetached 以"独立会话 + 后台"的方式拉起任务进程：
//   - 优先直接 os.StartProcess（Linux / Termux 上用这条）；
//   - Android 某些系统（targetSdk 较新 / linker 限制）不允许 Go 直接 exec
//     应用数据目录里的 PIE 程序，这时回退到 `/system/bin/sh -c 'exec ...'`。
//     shell 是系统分区程序，可以正常 exec；它再 exec 我们的二进制时，
//     会多传一份自身路径，main.normalizeArgs 会剥掉。
//   - setsid：脱离当前终端的会话/进程组，主进程或终端退出都不会带走它；
//   - stdout/stderr 追加到日志文件，保证启动早期的输出（含 panic）也不会丢；
//   - 继承环境变量，所以 LIANT_BOT_TOKEN 之类的配置能传下去。
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
	if err == nil {
		pid := proc.Pid
		_ = proc.Release()
		return pid, nil
	}

	// 回退：Android 系统 shell 代劳 exec（仅当 /system/bin/sh 存在）。
	if _, statErr := os.Stat("/system/bin/sh"); statErr != nil {
		return 0, err
	}
	cmdline := "exec " + shellQuote(exe)
	for _, a := range args {
		cmdline += " " + shellQuote(a)
	}
	cmd := exec.Command("/system/bin/sh", "-c", cmdline)
	cmd.Dir = WorkDir()
	cmd.Env = os.Environ()
	cmd.Stdin = devNull
	cmd.Stdout = logF
	cmd.Stderr = logF
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if startErr := cmd.Start(); startErr != nil {
		// 两条路都失败时，优先把最初（更常见）的错误给用户。
		return 0, err
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	return pid, nil
}

// shellQuote 把任意字符串安全地放进 POSIX shell 单引号里。
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
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
