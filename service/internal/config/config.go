package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

import (
	"github.com/Bailensn/LianT/service/internal/crypto"
)

var (
	configPath     = "config.enc"
	defaultKeyPath = filepath.Join(
		"data",
		"key",
		"master.key",
	)
)

type Config struct {
	Proxy struct {
		Enabled bool   `json:"enabled"`
		URL     string `json:"url"`
	} `json:"proxy"`
	Storage struct {
		Driver     string `json:"driver,omitempty"`
		Database   string `json:"database"`
		Key        string `json:"key"`
		PGHost     string `json:"pg_host,omitempty"`
		PGPort     int    `json:"pg_port,omitempty"`
		PGUser     string `json:"pg_user,omitempty"`
		PGPassword string `json:"pg_password,omitempty"`
		PGDatabase string `json:"pg_database,omitempty"`
		PGSSLMode  string `json:"pg_sslmode,omitempty"`
	} `json:"storage"`
	// Auth 是暴露到公网时的一道前置认证。
	Auth struct {
		BasicUser string `json:"basic_user"`
		BasicPass string `json:"basic_pass"`
	} `json:"auth"`
	// Pair 是配对策略。
	Pair struct {
		// Allow 非空时只允许名字匹配其中任意 glob 的设备配对。
		Allow []string `json:"allow"`
	} `json:"pair"`
	Connect struct {
		Url string `json:"url"`
		// Address 覆盖对外通告的 WSS 地址；留空则由启动时自动探测。
		Address string `json:"address"`
		// Listen 是 WSS 的监听地址，默认 :8443（协议 §1 的固定端口）。
		Listen string `json:"listen"`
	} `json:"connect"`
}

// PostgresDSN 用配置里的账号密码拼 PostgreSQL 连接串。
func (c Config) PostgresDSN() string {
	if c.Storage.Driver != "postgres" {
		return ""
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.Storage.PGUser, c.Storage.PGPassword),
		Host:   c.Storage.PGHost,
		Path:   "/" + c.Storage.PGDatabase,
	}
	if c.Storage.PGPort > 0 {
		u.Host = u.Host + ":" + strconv.Itoa(c.Storage.PGPort)
	}
	q := url.Values{}
	q.Set("sslmode", defaultString(c.Storage.PGSSLMode, "disable"))
	u.RawQuery = q.Encode()
	return u.String()
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func ConfigPath() string {
	return configPath
}

func DefaultKeyPath() string {
	return defaultKeyPath
}

// ==========================
// 保存配置
// ==========================
func SaveConfig(
	cfg Config,
) {
	raw, err := json.MarshalIndent(
		cfg,
		"",
		"    ",
	)
	if err != nil {
		panic(err)
	}
	encrypted := crypto.Encrypt(
		raw,
	)
	err = os.WriteFile(
		configPath,
		encrypted,
		0600,
	)
	if err != nil {
		panic(err)
	}
}

// ==========================
// 读取配置
// ==========================
func LoadConfig() Config {
	var cfg Config
	data, err := os.ReadFile(
		configPath,
	)
	if err != nil {
		panic(err)
	}
	raw := crypto.Decrypt(
		data,
	)
	err = json.Unmarshal(
		raw,
		&cfg,
	)
	if err != nil {
		panic(err)
	}
	// 主密钥只用于经 KDF 派生各用途子密钥；配置文件本身用 config 子密钥加解密。
	// master key 路径由 crypto 包决定（默认 key/master.key，可用
	// LIANT_MASTER_KEY_FILE 覆盖），不依赖 cfg.Storage.Key，避免循环依赖。
	return cfg
}

// ==========================
// config命令
// ==========================
func ConfigCommand(
	args []string,
) {
	cfg := LoadConfig()
	if len(args) < 1 {
		fmt.Println(
			"用法: LianT config get/set",
		)
		return
	}
	switch args[0] {
	case "get":
		raw, _ := json.MarshalIndent(
			cfg,
			"",
			"    ",
		)
		fmt.Println(
			string(raw),
		)
	case "set":
		if len(args) != 3 {
			fmt.Println(
				"LianT config set key value",
			)
			return
		}
		switch args[1] {
		case "proxy.enabled":
			value, err := strconv.ParseBool(
				args[2],
			)
			if err != nil {
				fmt.Println(
					"proxy.enabled 必须是 true 或 false",
				)
				return
			}
			cfg.Proxy.Enabled = value
		case "proxy.url":
			cfg.Proxy.URL = args[2]
		case "storage.driver":
			cfg.Storage.Driver = args[2]
		case "storage.database":
			cfg.Storage.Database = args[2]
		case "storage.key":
			cfg.Storage.Key = args[2]
		case "storage.pg_host":
			cfg.Storage.PGHost = args[2]
		case "storage.pg_port":
			value, err := strconv.Atoi(args[2])
			if err != nil || value < 1 || value > 65535 {
				fmt.Println("storage.pg_port 必须是 1-65535 的整数")
				return
			}
			cfg.Storage.PGPort = value
		case "storage.pg_user":
			cfg.Storage.PGUser = args[2]
		case "storage.pg_password":
			cfg.Storage.PGPassword = args[2]
		case "storage.pg_database":
			cfg.Storage.PGDatabase = args[2]
		case "storage.pg_sslmode":
			cfg.Storage.PGSSLMode = args[2]
		case "connect.url":
			cfg.Connect.Url = args[2]
		case "connect.address":
			cfg.Connect.Address = args[2]
		case "connect.listen":
			cfg.Connect.Listen = args[2]
		case "auth.user":
			cfg.Auth.BasicUser = args[2]
		case "auth.pass":
			cfg.Auth.BasicPass = args[2]
		case "pair.allow":
			// 逗号分隔的设备名 glob；留空表示不限制。
			var allow []string
			for _, part := range strings.Split(args[2], ",") {
				if v := strings.TrimSpace(part); v != "" {
					allow = append(allow, v)
				}
			}
			cfg.Pair.Allow = allow
		default:
			fmt.Println(
				"未知配置",
			)
			return
		}
		SaveConfig(
			cfg,
		)
		fmt.Println(
			"保存成功",
		)
	default:
		fmt.Println(
			"未知命令",
		)
	}
}
