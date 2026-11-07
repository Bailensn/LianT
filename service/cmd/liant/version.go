package main

import (
	"fmt"
	"runtime"
)

// version 由发布流水线注入：
//   go build -ldflags "-X main.version=0.1.3" ./cmd/liant
// 本地直接 go build 时保持 dev。
var version = "dev"

func versionCommand() {
	fmt.Printf("LianT Service %s (%s %s/%s)\n", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
