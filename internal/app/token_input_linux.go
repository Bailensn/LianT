//go:build linux || android

package app

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// readSecretLine 在终端里关闭回显读取 token；管道输入时退回普通读取。
func readSecretLine(prompt string) (string, error) {
	fd := int(os.Stdin.Fd())
	term, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err == nil {
		old := *term
		term.Lflag &^= unix.ECHO
		if err := unix.IoctlSetTermios(fd, unix.TCSETS, term); err == nil {
			defer func() { _ = unix.IoctlSetTermios(fd, unix.TCSETS, &old) }()
			fmt.Fprint(os.Stdout, prompt)
			line, rerr := bufio.NewReader(os.Stdin).ReadString('\n')
			fmt.Fprintln(os.Stdout)
			return strings.TrimSpace(line), rerr
		}
	}
	fmt.Fprint(os.Stdout, prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(line), err
}
