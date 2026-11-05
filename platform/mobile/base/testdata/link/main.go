// 本测试驱动只用于 Linux 控制组，通过系统 GNU ld 链接预编译对象。
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	args := []string{"--eh-frame-hdr"}
	for _, arg := range os.Args[1:] {
		if strings.HasSuffix(arg, ".c") {
			os.Exit(72)
		}
		switch {
		case arg == "-O2" || arg == "-g" || arg == "-m64" || arg == "-Qunused-arguments" || arg == "-pthread":
		case strings.HasPrefix(arg, "-Wl,"):
			args = append(args, strings.Split(arg[4:], ",")...)
		case arg == "-rdynamic":
			args = append(args, "--export-dynamic")
		case arg == "-shared" || arg == "-o" || !strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "-l"):
			args = append(args, arg)
		default:
			fmt.Fprintln(os.Stderr, "unsupported link argument:", arg)
			os.Exit(73)
		}
	}
	args = append(args, "-lc", "-lpthread", "-ldl")
	cmd := exec.Command("/usr/bin/ld", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
