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
		cfg.Storage.Driver = "sqlite"
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
	cfg := config.LoadConfig()
	db, err := store.OpenDatabase(cfg.Storage.Driver, cfg.PostgresDSN(), cfg.Storage.Database)
	if err != nil {
		fmt.Fprintln(os.Stderr, "初始化数据库失败：", err)
		if cfg.Storage.Driver == "postgres" {
			fmt.Fprintln(os.Stderr, "请确认 PostgreSQL 已创建角色和数据库，且 storage.pg_user / storage.pg_password 正确。")
			fmt.Fprintln(os.Stderr, "示例：CREATE ROLE \"用户名\" LOGIN PASSWORD '密码'; CREATE DATABASE 数据库名 OWNER \"用户名\";")
		}
		os.Exit(1)
	}
	_ = db.Close()

	fmt.Println(
		"初始化完成",
	)
}
