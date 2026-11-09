//go:build !ios

package main

import "fmt"

func main() {
	fmt.Println("本例需要通过 cmd/goui 构建并运行于 iOS ARM64 Simulator/device；见 README.md。")
}
