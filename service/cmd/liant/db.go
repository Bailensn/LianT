package main

import (
	"fmt"
	"os"

	"github.com/Bailensn/LianT/service/internal/store"
)

func dbCommand(args []string) {
	if len(args) == 0 || args[0] != "wipe" {
		fmt.Println("用法: LianT db wipe")
		return
	}
	cfg := loadConfigOrDie()
	if cfg.Storage.Driver == "postgres" {
		if err := store.WipePostgres(cfg.PostgresDSN()); err != nil {
			fmt.Fprintln(os.Stderr, "清理 PostgreSQL 失败:", err)
			os.Exit(1)
		}
	} else if cfg.Storage.Database != "" {
		_ = os.Remove(cfg.Storage.Database)
	}
	fmt.Println("数据库已清理")
}
