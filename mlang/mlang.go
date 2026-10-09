// Package mlang provides language-aware text resources independent of GUI
// widgets. Define resources once, usually beside icons in an application's
// resource package, and call their getters when building or translating a UI.
//
// The current language is process-wide and initially English. SetLanguage emits
// synchronous change notifications on its caller's goroutine. Once connected to
// a GUI, change language on the GUI thread (use App.Post from other goroutines).
// Resource construction and reads are safe from any goroutine. This package
// does not query system settings, perform I/O, or mirror right-to-left layouts.
package mlang

import (
	"sync"

	"github.com/golang-gui/goui/core/signal"
	"golang.org/x/text/language"
)

var current = struct {
	sync.RWMutex
	tag     language.Tag
	changed signal.Signal0
}{tag: language.English}

// Language returns the canonical BCP 47 tag requested by SetLanguage, not a
// particular resource's matched or fallback language.
func Language() string {
	current.RLock()
	defer current.RUnlock()
	return current.tag.String()
}

// SetLanguage changes the application language. Invalid tags return an error
// without changing state or notifying listeners. Valid but untranslated tags
// are accepted: each resource uses its own fallback. Equivalent canonical tags
// do not notify again. An empty tag is invalid; "und" selects resource fallbacks.
// Notifications are synchronous, outside the state lock, on the caller's thread.
func SetLanguage(tag string) error {
	parsed, err := language.Parse(tag)
	if err != nil {
		return err
	}
	current.Lock()
	if current.tag == parsed {
		current.Unlock()
		return nil
	}
	current.tag = parsed
	current.Unlock()
	current.changed.Emit()
	return nil
}

// ConnectChanged connects a language-change listener without an initial call.
// Disconnect the returned handle when its owner is released. GUI listeners run
// on the GUI thread when SetLanguage follows the package's GUI-thread contract.
func ConnectChanged(fn func()) signal.Handle { return current.changed.Connect(fn) }
