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
	err := mobilebase.RunLinkDriver(ctx, os.Args[1:], os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "goui-link:", err)
		os.Exit(1)
	}
}
