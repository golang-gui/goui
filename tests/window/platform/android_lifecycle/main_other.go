//go:build !android

package main

import "fmt"

func main() {
	fmt.Println("请使用 cmd/goui 构建 Android ARM64 APK；操作说明见 README.md。")
}
