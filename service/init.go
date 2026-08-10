package main

import (
	"fmt"
	"os"
)

import (
	"github.com/Bailensn/LianT/service/botmanager"
	"github.com/Bailensn/LianT/service/config"
)

func initCommand() {
	/*==config.go==*/

	// 创建默认配置
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
		cfg.Connect.Url=
			"https://api.telegram.org"
		config.SaveConfig(
			cfg,
		)
	}

	/*==botmanager.go==*/

	// 初始化data数据库
	botmanager.InitDatabase()

	fmt.Println(
		"初始化完成",
	)
}
