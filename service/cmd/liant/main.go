package main

import (
	"fmt"
	"os"

	"github.com/Bailensn/LianT/service/internal/config"
	"github.com/Bailensn/LianT/service/internal/supervisor"
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
	case "config":
		config.ConfigCommand(args[2:])
	case "service":
		supervisor.ServiceCommand(args[2:])
	case "bot":
		botCommand(args[2:])
	case "botstart":
		supervisor.BotCommand(args[2:])
	case "service-daemon":
		supervisor.StartDaemon()
	default:
		fmt.Println("未知命令")
	}
}
