package main

import (
	"fmt"
	"os"
)

import (
	"LianT/bot"
	"LianT/config"
)

func main() {
	args := os.Args
	if len(args) < 2 {
		fmt.Println("请输入命令")
		fmt.Println("例如: LianT init")
		return
	}
	switch args[1] {
	case "init":
		initCommand()
	case "service":
		serviceCommand(args[2:])
	case "botmanager":
		bot.BotmanagerCommand(args[2:])
	case "config":
		config.ConfigCommand(args[2:])
	default:
		fmt.Println("未知命令")
	}
}