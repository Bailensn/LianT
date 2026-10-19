//go:build !linux && !android

package app

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func readSecretLine(prompt string) (string, error) {
	fmt.Fprint(os.Stdout, prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(line), err
}
