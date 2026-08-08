package main

import (
	"fmt"
)

func serviceCommand(args []string) {
	switch args[0]{
		case "start":
			fmt.Println(
				"start",
			)
		case "stop":
			fmt.Println(
				"stop",
			)
		default:
			fmt.Println(
				"未知命令",
			)
	}
}