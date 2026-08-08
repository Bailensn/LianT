package crypto

import (
	"os"
	"github.com/fernet/fernet-go"
)

var keyPath = "data/master.key"


func GetKey() *fernet.Key {
	os.MkdirAll(
		"data",
		0700,
	)
	_,err:=os.Stat(keyPath)
	if os.IsNotExist(err){
		key:=fernet.Key{}
		err:=key.Generate()
		if err!=nil{
			panic(err)
		}
		err=os.WriteFile(
			keyPath,
			[]byte(key.Encode()),
			0600,
		)
		if err!=nil{
			panic(err)
		}
	}
	data,err:=os.ReadFile(
		keyPath,
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

func Encrypt(
	data []byte,
)[]byte{
	token,err:=fernet.EncryptAndSign(
		data,
		GetKey(),
	)
	if err!=nil{
		panic(err)
	}
	return []byte(token)
}



func Decrypt(
	data []byte,
)[]byte{
	result:=fernet.VerifyAndDecrypt(
		data,
		0,
		[]*fernet.Key{
			GetKey(),
		},
	)
	if result==nil{
		panic(
			"配置解密失败",
		)
	}
	return result
}

func EncryptToken(
	token string,
)string{
	return string(
		Encrypt(
			[]byte(token),
		),
	)
}