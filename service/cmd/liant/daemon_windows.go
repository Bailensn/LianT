//go:build windows

package main

import "errors"

func startDaemon([]string) error {
	return errors.New("Windows 暂不支持 -d，请用服务管理器或计划任务")
}

func stopDaemon() error {
	return errors.New("Windows 暂不支持 stop")
}
