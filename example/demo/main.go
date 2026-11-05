// GOUI widget and feature gallery. Run: go run ./example/demo.
// See README.md for capabilities, desktop requirements and explicit system actions.
package main

import (
	"errors"
	"flag"
	"log"
	"os"
	"runtime/trace"

	"github.com/golang-gui/goui/mlang"
	"github.com/golang-gui/goui/ui"
)

func main() {
	tracePath := flag.String("trace", "", "write a Go execution trace to this file (overwrites existing contents)")
	flag.Parse()
	if err := runDemo(*tracePath); err != nil {
		log.Fatal(err)
	}
}

func runDemo(tracePath string) (err error) {
	if tracePath != "" {
		file, openErr := os.Create(tracePath)
		if openErr != nil {
			return openErr
		}
		if err := trace.Start(file); err != nil {
			return errors.Join(err, file.Close())
		}
		defer func() {
			trace.Stop()
			err = errors.Join(err, file.Close())
		}()
	}
	// Demo startup language is an application choice, not an implicit Settings policy.
	if err := mlang.SetLanguage("zh-CN"); err != nil {
		return err
	}
	d, err := newDemo()
	if err != nil {
		return err
	}
	defer d.stopActivities()
	defer d.windows.closeCapture()
	languageChanged := mlang.ConnectChanged(d.translateUI)
	defer languageChanged.Disconnect()
	return ui.Run("org.golanggui.Demo", d.build)
}
