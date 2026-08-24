package main

import (
	"fmt"
	"os"
)

import (
	"github.com/LensnTeam/LianT/service/config"
)

func initCommand() {
	/*==config.go==*/

	// 创建默认配置
	_, err := os.ReadFile(
		config.ConfigPath(),
	)
	if os.IsNotExist(err) {
		cfg := config.Config{}
		cfg.Proxy.Enabled = false
		cfg.Proxy.URL = ""
		cfg.Storage.Database =
			"data/bots.db"
		cfg.Storage.Key =
			"data/master.key"
		cfg.Connect.Url =
			"https://api.telegram.org"
		config.SaveConfig(
			cfg,
		)
	}

	/*==bot.go==*/

	// 初始化data数据库
	db := openBotDB()
	defer db.Close()
	_, err = db.Exec(
		`
CREATE TABLE IF NOT EXISTS bots(
bot_id INTEGER PRIMARY KEY,
token TEXT NOT NULL,
username TEXT,
first_name TEXT
)
`,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(
		"初始化完成",
	)
}
