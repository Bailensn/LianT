package service

import (
	"fmt"
)

func Start() {
	fmt.Println(
		"start",
	)
}

func Stop() {
	fmt.Println(
		"stop",
	)
}

func ReStart() {
	fmt.Println(
		"restart",
	)
}

func ServiceCommand(args []string) {
	switch args[0]{
		case "start":
			Start()
		case "stop":
			Stop()
		case "restart":
			ReStart()
		default:
			fmt.Println(
				"未知命令",
			)
	}
}