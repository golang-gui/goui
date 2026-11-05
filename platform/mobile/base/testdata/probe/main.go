package main

import (
	"example.com/baseprobe/bridge"
	"example.com/baseprobe/logic"
)

func init() { bridge.Register(logic.Compute) }
func main() {}
