// 仅验证构建编排、环境和失败发布；不执行 Go 编译或验证原生 ABI。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if path := os.Getenv("GOUI_TEST_BUILD_LOG"); path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			panic(err)
		}
		values := map[string]any{"args": os.Args[1:]}
		for _, arg := range os.Args[1:] {
			if !strings.HasPrefix(arg, "-overlay=") {
				continue
			}
			data, err := os.ReadFile(strings.TrimPrefix(arg, "-overlay="))
			if err != nil {
				panic(err)
			}
			var overlay struct{ Replace map[string]string }
			if err := json.Unmarshal(data, &overlay); err != nil {
				panic(err)
			}
			for target, source := range overlay.Replace {
				data, err := os.ReadFile(source)
				if err != nil {
					panic(err)
				}
				values["entry"], values["entryTarget"] = string(data), target
			}
		}
		for _, key := range []string{"GOOS", "GOARCH", "GOARM64", "GOEXPERIMENT", "GOROOT", "GOTOOLCHAIN", "GOWORK", "GOCACHE", "GOPROXY", "GOFLAGS", "CGO_ENABLED", "CC", "CXX", "GOUI_BASE", "GOUI_LINK_INPUTS", "GOUI_LINK_TOOLS", "GOUI_GO"} {
			values[key] = os.Getenv(key)
		}
		if err = json.NewEncoder(f).Encode(values); err != nil {
			panic(err)
		}
		f.Close()
	}
	if os.Args[1] == "env" {
		version := "go1.25.8"
		if os.Getenv("GOUI_TEST_BUILD_BADGO") == "1" {
			version = "go1.25.9"
		}
		if len(os.Args) == 3 {
			fmt.Println(version)
			return
		}
		values := map[string]string{"GOVERSION": version, "GOROOT": "fixture-goroot", "GOFLAGS": os.Getenv("GOFLAGS"), "CGO_CFLAGS": "-O2 -g", "CGO_CPPFLAGS": "", "CGO_CXXFLAGS": "-O2 -g", "CGO_FFLAGS": "-O2 -g", "CGO_LDFLAGS": "-O2 -g"}
		if err := json.NewEncoder(os.Stdout).Encode(values); err != nil {
			panic(err)
		}
		return
	}
	if os.Args[1] == "list" {
		dir, err := filepath.Abs("main")
		if err != nil {
			panic(err)
		}
		if err = json.NewEncoder(os.Stdout).Encode(map[string]string{"Name": "main", "Dir": dir}); err != nil {
			panic(err)
		}
		return
	}
	if os.Args[1] != "build" {
		panic("unexpected operation")
	}
	if marker := os.Getenv("GOUI_TEST_BUILD_WAIT"); marker != "" {
		if err := os.WriteFile(marker, []byte("started"), 0600); err != nil {
			panic(err)
		}
		time.Sleep(time.Hour)
	}
	for i, arg := range os.Args {
		if arg != "-o" {
			continue
		}
		data, err := os.ReadFile(os.Getenv("GOUI_TEST_LLD_COPY"))
		if err != nil {
			panic(err)
		}
		if err = os.WriteFile(os.Args[i+1], data, 0755); err != nil {
			panic(err)
		}
		if os.Getenv("GOUI_TEST_BUILD_FAIL") == "1" {
			os.Exit(19)
		}
		return
	}
	panic("missing output")
}
