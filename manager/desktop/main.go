package main

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func main() {
	// 创建应用
	myApp := app.New()
	myWindow := myApp.NewWindow("LianT Manager")

	// 状态变量（用字符串表示连接状态）
	connected := false
	statusLabel := widget.NewLabel("未连接")
	statusLabel.Importance = widget.LowImportance

	// 连接/断开按钮
	toggleBtn := widget.NewButton("连接", func() {
		if !connected {
			// 模拟连接
			connected = true
			statusLabel.SetText("已连接")
			statusLabel.Importance = widget.HighImportance
			toggleBtn.SetText("断开")
			fmt.Println("连接到服务端 wss://127.0.0.1:端口")
		} else {
			// 模拟断开
			connected = false
			statusLabel.SetText("未连接")
			statusLabel.Importance = widget.LowImportance
			toggleBtn.SetText("连接")
			fmt.Println("断开连接")
		}
	})

	// 退出按钮
	exitBtn := widget.NewButton("退出", func() {
		myApp.Quit()
	})

	// 布局：垂直排列
	content := container.NewVBox(
		widget.NewLabel("LianT Manager"),
		statusLabel,
		toggleBtn,
		exitBtn,
	)

	myWindow.SetContent(content)
	myWindow.Resize(fyne.NewSize(400, 250))
	myWindow.ShowAndRun()
}
