package main

import (
	"fmt"
	"os"
)

import (
	"github.com/Bailensn/LianT/service/internal/config"
	"github.com/Bailensn/LianT/service/internal/store"
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
			"data/key/master.key"
		cfg.Connect.Url =
			"https://api.telegram.org"
		cfg.Connect.Listen = ":8443"
		config.SaveConfig(
			cfg,
		)
	}

	// 数据库交给 store 迁移，避免这里再维护一份 schema。
	db, err := store.Open(config.LoadConfig().Storage.Database)
	if err != nil {
		panic(err)
	}
	_ = db.Close()

	fmt.Println(
		"初始化完成",
	)
}
