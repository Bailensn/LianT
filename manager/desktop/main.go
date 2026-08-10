package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("LianT Manager")

	connected := false
	statusLabel := widget.NewLabel("未连接")
	statusLabel.Importance = widget.LowImportance

	toggleBtn := widget.NewButton("连接", func() {
		if !connected {
			connected = true
			statusLabel.SetText("已连接")
			statusLabel.Importance = widget.HighImportance
			toggleBtn.SetText("断开")
			fmt.Println("连接到服务端 wss://127.0.0.1:端口")
		} else {
			connected = false
			statusLabel.SetText("未连接")
			statusLabel.Importance = widget.LowImportance
			toggleBtn.SetText("连接")
			fmt.Println("断开连接")
		}
	})

	exitBtn := widget.NewButton("退出", func() {
		myApp.Quit()
	})

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
