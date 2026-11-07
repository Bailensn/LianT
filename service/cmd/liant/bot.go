package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/Bailensn/LianT/service/internal/core"
	"github.com/Bailensn/LianT/service/internal/platform"
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
  LianT bot add --platform qqbot --app-id 123456 [--label 名字]
      密钥（如 AppSecret）会提示隐藏输入；脚本可用 --cred-file creds.json
  LianT bot list
  LianT bot remove <bot_id>

凭据里密钥不要写在命令行上：会进 shell history，也会出现在 ps 里。`)
}

// credFlags 收集可重复的 --cred key=value（只允许非密钥字段）。
type credFlags []string

func (c *credFlags) String() string { return strings.Join(*c, ",") }

func (c *credFlags) Set(v string) error {
	*c = append(*c, v)
	return nil
}

// prescanPlatform 先扫出 --platform，才能在注册表里查适配器的凭据字段。
func prescanPlatform(args []string) string {
	name := "qqbot"
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--platform" && i+1 < len(args):
			name = args[i+1]
		case strings.HasPrefix(args[i], "--platform="):
			name = strings.TrimPrefix(args[i], "--platform=")
		}
	}
	return name
}

func botAdd(args []string) {
	platformName := prescanPlatform(args)
	adapter, ok := platform.Get(platformName)
	if !ok {
		fmt.Fprintf(os.Stderr, "没有注册的平台 %q（已注册：%s）\n", platformName, strings.Join(platform.Names(), ", "))
		os.Exit(1)
	}

	fs := flag.NewFlagSet("bot add", flag.ExitOnError)
	fs.String("platform", platformName, "平台名")
	label := fs.String("label", "", "显示名，可选")
	credFile := fs.String("cred-file", "", "凭据 JSON 文件（建议 chmod 600）；脚本里用它，别把密钥放命令行")
	var extra credFlags
	fs.Var(&extra, "cred", "非密钥字段 key=value；密钥请交互输入或用 --cred-file")

	var fields []platform.CredentialField
	fieldValues := map[string]*string{}
	if spec, ok := adapter.(platform.CredentialSpec); ok {
		fields = spec.CredentialFields()
		for _, f := range fields {
			flagName := strings.ReplaceAll(f.Key, "_", "-")
			hint := f.Label
			if !f.Secret {
				hint += "（不要放密钥）"
			}
			fieldValues[f.Key] = fs.String(flagName, "", hint)
		}
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	values := map[string]string{}
	if *credFile != "" {
		loaded, err := loadCredFile(*credFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "读取凭据文件失败：", err)
			os.Exit(1)
		}
		for k, v := range loaded {
			values[k] = v
		}
	}
	for _, kv := range extra {
		key, value, ok := strings.Cut(kv, "=")
		if !ok || key == "" {
			fmt.Fprintf(os.Stderr, "凭据格式应为 key=value：%q\n", kv)
			os.Exit(1)
		}
		if isSecretField(fields, key) {
			fmt.Fprintln(os.Stderr, "密钥不要写在命令行上（会进 shell history / ps），请去掉这一项：")
			fmt.Fprintln(os.Stderr, "  交互输入：直接回车让它提示；或 --cred-file 指定 0600 的 JSON 文件")
			os.Exit(1)
		}
		values[key] = value
	}
	for key, ptr := range fieldValues {
		if *ptr != "" {
			values[key] = *ptr
		}
	}

	// 缺的字段：密钥交互输入，非密钥报错。
	for _, f := range fields {
		if values[f.Key] != "" {
			continue
		}
		if !f.Secret {
			fmt.Fprintf(os.Stderr, "缺少 --%s\n", strings.ReplaceAll(f.Key, "_", "-"))
			os.Exit(1)
		}
		secret, err := promptSecret(f.Label)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取 %s 失败：%v（脚本请用 --cred-file）\n", f.Label, err)
			os.Exit(1)
		}
		values[f.Key] = secret
	}
	if len(values) == 0 {
		fmt.Fprintln(os.Stderr, "没有提供任何凭据")
		os.Exit(1)
	}

	raw, err := json.Marshal(protocol.BotAdd{
		Platform:    platformName,
		Credentials: values,
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

func isSecretField(fields []platform.CredentialField, key string) bool {
	for _, f := range fields {
		if f.Key == key {
			return f.Secret
		}
	}
	return false
}

func loadCredFile(path string) (map[string]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode().Perm()&0o077 != 0 {
		fmt.Fprintf(os.Stderr, "警告：%s 权限是 %o，建议 chmod 600\n", path, info.Mode().Perm())
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out map[string]string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func promptSecret(label string) (string, error) {
	fmt.Fprintf(os.Stderr, "%s（输入不回显）：", label)
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("当前不是终端")
	}
	raw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
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
