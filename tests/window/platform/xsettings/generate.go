//go:build ignore

// 生成可独立检查的GTK测试主题；输出限定在源码仓库之外。
package main

import "github.com/golang-gui/goui/internal/nativegen"

func main() {
	nativegen.Main("tests/window/platform/xsettings", themeSources)
}
