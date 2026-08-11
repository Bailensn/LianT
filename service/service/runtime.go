package service

import (
	"os"
	"path/filepath"
)

const runtimeDir = "data/runtime"

func initRuntime() error {
	return os.MkdirAll(
		runtimeDir,
		0755,
	)
}

func serviceAddrPath() string {
	return filepath.Join(
		runtimeDir,
		"service.addr",
	)
}

func saveServiceAddr(addr string) error {
	err:=initRuntime()
	if err!=nil{
		return err
	}
	return os.WriteFile(
		serviceAddrPath(),
		[]byte(addr),
		0644,
	)
}

func readServiceAddr() (string,error){
	data,err:=os.ReadFile(
		serviceAddrPath(),
	)
	if err!=nil{
		return "",
		err
	}
	return string(data),nil
}

func removeServiceAddr(){
	os.Remove(
		serviceAddrPath(),
	)
}