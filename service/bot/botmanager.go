package bot

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"os"
	"golang.org/x/term"
	_ "github.com/glebarez/go-sqlite"
	"gopkg.in/telebot.v4"
)

import (
	"LianT/config"
	"LianT/crypto"
)

type BotInfo struct {
	ID int64 `json:"id"`
	Username string `json:"username"`
	FirstName string `json:"first_name"`
}

type TelegramResponse struct {
	OK bool `json:"ok"`
	Result BotInfo `json:"result"`
}

// ==========================
// 数据库
// ==========================
func openBotDB()*sql.DB{
	cfg:=config.LoadConfig()
	db,err:=sql.Open(
		"sqlite",
		cfg.Storage.Database,
	)
	if err!=nil{
		panic(err)
	}
	return db
}
func InitDatabase(){
	db:=openBotDB()
	defer db.Close()
	_,err:=db.Exec(
		`
CREATE TABLE IF NOT EXISTS bots(
bot_id INTEGER PRIMARY KEY,
token TEXT NOT NULL,
username TEXT,
first_name TEXT
)
`,
	)
	if err!=nil{
		panic(err)
	}
}

// ==========================
// Telegram
// ==========================
func checkBot(
	token string,
)(BotInfo,error){
	cfg:=config.LoadConfig()
	settings:=telebot.Settings{
		Token: token,
	}
	if cfg.Proxy.Enabled {
		proxy,err:=url.Parse(
			cfg.Proxy.URL,
		)
		if err!=nil{
			return BotInfo{},err
		}
		settings.Client=&http.Client{
			Transport:&http.Transport{
				Proxy:http.ProxyURL(proxy),
			},
		}
		fmt.Println(
			"使用代理:",
			cfg.Proxy.URL,
		)
	}
	bot,err:=telebot.NewBot(
		settings,
	)
	if err!=nil{
		return BotInfo{},err
	}
	me:=bot.Me
	return BotInfo{
		ID: me.ID,
		Username: me.Username,
		FirstName: me.FirstName,
	},nil
}

// ==========================
// 保存
// ==========================
func saveBot(
	id int64,
	token string,
	info BotInfo,
)error{
	db:=openBotDB()
	defer db.Close()
	_,err:=db.Exec(
		`
INSERT INTO bots
(bot_id,token,username,first_name)
VALUES(?,?,?,?)
`,
		id,
		crypto.EncryptToken(token),
		info.Username,
		info.FirstName,
	)
	return err
}

// ==========================
// 删除
// ==========================
func removeBot(
	id int64,
){
	db:=openBotDB()
	defer db.Close()
	res,err:=db.Exec(
		`
DELETE FROM bots
WHERE bot_id=?
`,
		id,
	)
	if err!=nil{
		fmt.Println(err)
		return
	}
	n,_:=res.RowsAffected()
	if n==0{
		fmt.Println(
			"Bot不存在",
		)
	}else{
		fmt.Println(
			"删除成功",
		)
	}
}

// ==========================
// 隐式输入
// ==========================
func inputHidden(prompt string) string {
	fmt.Print(prompt)
	byteToken, err := term.ReadPassword(
		int(os.Stdin.Fd()),
	)
	fmt.Println()
	if err != nil {
		panic(err)
	}
	return string(byteToken)
}


// ==========================
// bot命令
// ==========================
func BotmanagerCommand(
	args []string,
){
	if len(args)<1{
		fmt.Println(
			"用法: LianT bot add/remove",
		)
		return
	}
	switch args[0]{
	case "add":
		if len(args) != 2 {
			fmt.Println(
				"LianT bot add <bot_id>",
			)
			return
		}
		id, err := strconv.ParseInt(
			args[1],
			10,
			64,
		)
		if err != nil {
			fmt.Println(
				"Bot ID必须是数字",
			)
			return
		}
		fmt.Print(
			"请输入Token: ",
		)
		tokenBytes, err := term.ReadPassword(
			int(os.Stdin.Fd()),
		)
		fmt.Println()
		if err != nil {
			fmt.Println(
				"读取Token失败:",
				err,
			)
			return
		}
		token := string(tokenBytes)
		fmt.Println(
			"正在验证Token...",
		)
		info, err := checkBot(
			token,
		)
		if err != nil {
			fmt.Println(
				err,
			)
			return
		}
		if info.ID != id {
			fmt.Println(
				"Bot ID与Token不匹配",
			)
			return
		}
		fmt.Println()
		fmt.Println(
			"验证成功",
		)
		fmt.Printf(
			"用户名: @%s\n",
			info.Username,
		)
		fmt.Printf(
			"名称: %s\n",
			info.FirstName,
		)
		err = saveBot(
			id,
			token,
			info,
		)
		if err != nil {
			fmt.Println(
				"该Bot已经存在",
			)
			return
		}
		fmt.Println()
		fmt.Println(
			"保存成功",
		)
	case "remove":
		if len(args)!=2{
			fmt.Println(
				"LianT bot remove <bot_id>",
			)
			return
		}
		id, err := strconv.ParseInt(
			args[1],
			10,
			64,
		)
		if err!=nil{
			fmt.Println(err)
			return
		}
		removeBot(id)
	default:
		fmt.Println(
			"未知bot命令",
		)
	}
}