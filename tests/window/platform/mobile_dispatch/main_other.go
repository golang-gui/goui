//go:build !android && !ios

package main

import "fmt"

func main() { fmt.Println("本例仅适用于 Android/iOS；请按 README.md 构建安装。") }
