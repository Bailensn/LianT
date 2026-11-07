package main

import (
	"fmt"
	"os"

	"github.com/Bailensn/LianT/service/internal/config"
)

func main() {
	args := os.Args
	if len(args) < 2 {
		fmt.Println("请输入命令")
		return
	}
	switch args[1] {
	case "init":
		initCommand()
	case "version":
		versionCommand()
	case "config":
		config.ConfigCommand(args[2:])
	case "serve":
		serveCommand(args[2:])
	case "pair":
		pairCommand(args[2:])
	case "bot":
		botCommand(args[2:])
	default:
		fmt.Println("未知命令")
	}
}
