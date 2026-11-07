package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/Bailensn/LianT/service/internal/config"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	args := os.Args
	if len(args) < 2 {
		fmt.Println("用法：LianT <init|config|serve|pair|stop|version>")
		fmt.Println("机器人管理在网页端：打开服务地址 → 机器人 → 添加")
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
	case "stop":
		stopCommand()
	default:
		fmt.Println("未知命令")
	}
}
