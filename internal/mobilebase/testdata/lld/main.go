// 只用于测试输出校验和原子替换的假链接器，不用于验证原生链接能力。
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

func main() {
	if len(os.Args) == 4 && os.Args[3] == "--version" {
		fmt.Println("LLD 0.1.0")
		return
	}
	if os.Getenv("GOUI_TEST_LLD_FAIL") == "1" {
		os.Exit(17)
	}
	file, err := os.Open(os.Args[len(os.Args)-1][1:])
	if err != nil {
		panic(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var previous string
	for scanner.Scan() {
		arg, err := strconv.Unquote(scanner.Text())
		if err != nil {
			panic(err)
		}
		if previous == "-o" {
			if want := os.Getenv("GOUI_TEST_LLD_NAME"); want != "" && filepath.Base(arg) != want {
				fmt.Fprintln(os.Stderr, "unexpected output basename")
				os.Exit(21)
			}
			data, err := os.ReadFile(os.Getenv("GOUI_TEST_LLD_COPY"))
			if err != nil {
				panic(err)
			}
			if err = os.WriteFile(arg, data, 0755); err != nil {
				panic(err)
			}
		}
		previous = arg
	}
	if err = scanner.Err(); err != nil {
		panic(err)
	}
}
