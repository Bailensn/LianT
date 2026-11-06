package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Bailensn/LianT/service/internal/core"
	"github.com/Bailensn/LianT/service/internal/protocol"
	"github.com/Bailensn/LianT/service/internal/server"
)

// botCommand 是本地管理命令：只做登记/查询/删除。
// 启停走协议方法（客户端连上来调用 bot.start/stop），因为要落在那台常驻进程里。
func botCommand(args []string) {
	if len(args) == 0 {
		botUsage()
		return
	}
	switch args[0] {
	case "add":
		botAdd(args[1:])
	case "list":
		botList()
	case "remove":
		botRemove(args[1:])
	default:
		botUsage()
	}
}

func botUsage() {
	fmt.Println(`用法:
  LianT bot add --platform qqbot --cred app_id=... --cred app_secret=... [--label 名字]
  LianT bot list
  LianT bot remove <bot_id>`)
}

// credFlags 收集可重复的 --cred key=value。
type credFlags []string

func (c *credFlags) String() string { return strings.Join(*c, ",") }

func (c *credFlags) Set(v string) error {
	*c = append(*c, v)
	return nil
}

func botAdd(args []string) {
	fs := flag.NewFlagSet("bot add", flag.ExitOnError)
	platformName := fs.String("platform", "qqbot", "平台名，例如 qqbot")
	label := fs.String("label", "", "显示名，可选")
	var creds credFlags
	fs.Var(&creds, "cred", "凭据键值对 key=value，可重复")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if *platformName == "" {
		fmt.Fprintln(os.Stderr, "缺少 --platform")
		os.Exit(1)
	}
	if len(creds) == 0 {
		fmt.Fprintln(os.Stderr, "缺少 --cred")
		os.Exit(1)
	}
	m := map[string]string{}
	for _, kv := range creds {
		key, value, ok := strings.Cut(kv, "=")
		if !ok || key == "" {
			fmt.Fprintf(os.Stderr, "凭据格式应为 key=value：%q\n", kv)
			os.Exit(1)
		}
		m[key] = value
	}
	raw, err := json.Marshal(protocol.BotAdd{
		Platform:    *platformName,
		Credentials: m,
		Label:       *label,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "编码请求失败：", err)
		os.Exit(1)
	}

	db := mustOpenStore()
	defer db.Close()
	svc, err := core.New(core.Options{Store: db})
	if err != nil {
		fmt.Fprintln(os.Stderr, "初始化失败：", err)
		os.Exit(1)
	}
	got, perr := svc.AddBot(context.Background(), server.Session{}, raw)
	if perr != nil {
		fmt.Fprintf(os.Stderr, "登记失败：%s %s\n", perr.Code, perr.Message)
		os.Exit(1)
	}
	fmt.Println("已登记", got.(protocol.BotRef).BotID)
}

func botList() {
	db := mustOpenStore()
	defer db.Close()
	bots, err := db.Bots(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "读取失败：", err)
		os.Exit(1)
	}
	if len(bots) == 0 {
		fmt.Println("还没有登记任何 Bot。")
		return
	}
	fmt.Printf("%-28s %-8s %-12s %-10s %s\n", "BOT_ID", "平台", "名称", "状态", "标签")
	for _, b := range bots {
		fmt.Printf("%-28s %-8s %-12s %-10s %s\n",
			b.ID, b.Platform, b.DisplayName, b.Status.State, b.Label)
	}
}

func botRemove(args []string) {
	if len(args) != 1 {
		fmt.Println("用法: LianT bot remove <bot_id>")
		os.Exit(1)
	}
	db := mustOpenStore()
	defer db.Close()
	if err := db.DeleteBot(context.Background(), args[0]); err != nil {
		fmt.Fprintln(os.Stderr, "删除失败：", err)
		os.Exit(1)
	}
	fmt.Println("已删除", args[0], "（消息与媒体记录一并删除）")
}
