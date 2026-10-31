// GOUI widget and feature gallery. Run: go run ./example/demo.
// See README.md for capabilities, desktop requirements and explicit system actions.
package main

import (
	"log"

	"github.com/golang-gui/goui/mlang"
	"github.com/golang-gui/goui/ui"
)

func main() {
	// Demo startup language is an application choice, not an implicit Settings policy.
	if err := mlang.SetLanguage("zh-CN"); err != nil {
		log.Fatal(err)
	}
	d, err := newDemo()
	if err != nil {
		log.Fatal(err)
	}
	defer d.stopActivities()
	languageChanged := mlang.ConnectChanged(d.translateUI)
	defer languageChanged.Disconnect()
	if err := ui.Run("org.golanggui.Demo", d.build); err != nil {
		log.Fatal(err)
	}
}
