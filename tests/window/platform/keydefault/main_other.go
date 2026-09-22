//go:build !windows

package main

import "fmt"

func main() {
	fmt.Println("Not applicable: this probe verifies the Windows native key/WM_CHAR message queue.")
}
