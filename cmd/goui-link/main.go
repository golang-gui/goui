// Command goui-link adapts Go external-link arguments to the embedded Go linker
// or a verified LLVM linker.
// Components are selected through process-local environment variables.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	mobilebase "github.com/golang-gui/goui/platform/mobile/base"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	inputPath, toolPath := os.Getenv("GOUI_LINK_INPUTS"), os.Getenv("GOUI_LINK_TOOLS")
	var err error
	if len(os.Args) > 1 && os.Args[1] == "-q" {
		err = mobilebase.RunNativeArchive(ctx, os.Getenv("GOUI_GO"), os.Args[1:], os.Stdout, os.Stderr)
	} else if inputPath == "" || toolPath == "" {
		err = fmt.Errorf("GOUI_LINK_INPUTS and GOUI_LINK_TOOLS are required")
	} else {
		err = mobilebase.RunNativeLink(ctx, inputPath, toolPath, os.Args[1:], os.Stdout, os.Stderr)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "goui-link:", err)
		os.Exit(1)
	}
}
