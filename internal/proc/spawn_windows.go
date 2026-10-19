//go:build windows

package proc

import (
	"os"
	"syscall"
)

// Windows 下的"后台起来"：DETACHED_PROCESS + 新进程组，日志同样写文件。
// （目标环境是 Android/Linux，这里只是让代码在 Windows 上也能编译通过。）
const (
	CreateNewProcessGroup = 0x00000200
	DetachedProcess       = 0x00000008
)

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
		Sys:   &syscall.SysProcAttr{CreationFlags: DetachedProcess | CreateNewProcessGroup},
	})
	if err != nil {
		return 0, err
	}
	pid := pid
	_ = release()
	return pid, nil
}

// IgnoreHangup：Windows 没有 SIGHUP 语义，无需处理。
func IgnoreHangup() {}

func WorkDir() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}
