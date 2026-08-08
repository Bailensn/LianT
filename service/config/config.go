package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"path/filepath"
	"github.com/fernet/fernet-go"
)

import (
	"LianT/crypto"
)

var (
	configPath = "config.enc"
	defaultKeyPath = filepath.Join(
		"data",
		"master.key",
	)
)

type Config struct {
	Proxy struct {
		Enabled bool `json:"enabled"`
		URL string `json:"url"`
	} `json:"proxy"`
	Storage struct {
		Database string `json:"database"`
		Key string `json:"key"`
	} `json:"storage"`
}

func ConfigPath() string {
	return configPath
}

func DefaultKeyPath() string {
	return defaultKeyPath
}

// ==========================
// master.key
// ==========================
func InitMasterKey(){
	os.MkdirAll(
		"data",
		0700,
	)
	_,err:=os.Stat(
		defaultKeyPath,
	)
	if os.IsNotExist(err){
		key:=fernet.Key{}
		err:=key.Generate()
		if err!=nil{
			panic(err)
		}
		err=os.WriteFile(
			defaultKeyPath,
			[]byte(key.Encode()),
			0600,
		)
		if err!=nil{
			panic(err)
		}
	}
}

func getFernetKey()*fernet.Key{
	InitMasterKey()
	data,err:=os.ReadFile(
		defaultKeyPath,
	)
	if err!=nil{
		panic(err)
	}
	key,err:=fernet.DecodeKey(
		string(data),
	)
	if err!=nil{
		panic(err)
	}
	return key
}

// ==========================
// 保存配置
// ==========================
func SaveConfig(
	cfg Config,
){
	raw,err:=json.MarshalIndent(
		cfg,
		"",
		"    ",
	)
	if err!=nil{
		panic(err)
	}
	encrypted:=crypto.Encrypt(
		raw,
	)
	err=os.WriteFile(
		configPath,
		encrypted,
		0600,
	)
	if err!=nil{
		panic(err)
	}
}

// ==========================
// 读取配置
// ==========================
func LoadConfig()Config{
	var cfg Config
	data,err:=os.ReadFile(
		configPath,
	)
	if err!=nil{
		panic(err)
	}
	raw:=crypto.Decrypt(
		data,
	)
	err=json.Unmarshal(
		raw,
		&cfg,
	)
	if err!=nil{
		panic(err)
	}
	return cfg
}

// ==========================
// config命令
// ==========================
func ConfigCommand(
	args []string,
){
	cfg:=LoadConfig()
	if len(args)<1{
		fmt.Println(
			"用法: LianT config get/set",
		)
		return
	}
	switch args[0]{
	case "get":
		raw,_:=json.MarshalIndent(
			cfg,
			"",
			"    ",
		)
		fmt.Println(
			string(raw),
		)
	case "set":
		if len(args)!=3{
			fmt.Println(
				"用法: LianT config set key value",
			)
			return
		}
		switch args[1]{
		case "proxy.enabled":
			value,err:=strconv.ParseBool(
				args[2],
			)
			if err!=nil{
				fmt.Println(
					"proxy.enabled 必须是 true 或 false",
				)
				return
			}
			cfg.Proxy.Enabled=value
		case "proxy.url":
			cfg.Proxy.URL=args[2]
		case "storage.database":
			cfg.Storage.Database=args[2]
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