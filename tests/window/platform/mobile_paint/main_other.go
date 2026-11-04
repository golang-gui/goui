//go:build !android && !ios

package main

import "fmt"

func main() { fmt.Println("本例需要 Android/iOS 原生宿主，详见 README.md。") }
