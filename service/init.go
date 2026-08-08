package main

import (
	"fmt"
	"os"
)

import (
	"LianT/bot"
	"LianT/config"
)

func initCommand() {
	/*==config.go==*/

	// 1. 生成 master.key
	config.InitMasterKey()

	// 2. 创建默认配置
	_,err:=os.ReadFile(
		config.ConfigPath(),
	)
	if os.IsNotExist(err){
		cfg:=config.Config{}
		cfg.Proxy.Enabled=false
		cfg.Proxy.URL=""
		cfg.Storage.Database=
			"data/bots.db"
		cfg.Storage.Key=
			"data/master.key"
		config.SaveConfig(
			cfg,
		)
	}

	/*==botmanager.go==*/

	// 3. 初始化数据库
	bot.InitDatabase()

	fmt.Println(
		"初始化完成",
	)
}