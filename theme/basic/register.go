package basic

import (
	"slices"
	"strings"
	"sync"

	"github.com/golang-gui/goui/style"
)

// Register adds a third-party control's rules. Call from package init, before
// the first Rules or Sheet call. name should be a unique module/package path.
// Empty names, nil functions, duplicate names and late registration panic.
// The function receives only built-in styles, and must return deterministic,
// independently owned rules in its own namespace. It must be safe to call
// concurrently and must not call Rules/Sheet or access GUI/native objects.
func Register(name string, rules func(base style.StyleSheet) []style.Rule) {
	registered.add(name, rules)
}

type basicRegistration struct {
	name  string
	rules func(base style.StyleSheet) []style.Rule
}
type basicRegistry struct {
	mu      sync.Mutex
	frozen  bool
	entries []basicRegistration
}

var registered basicRegistry

func (r *basicRegistry) add(name string, rules func(base style.StyleSheet) []style.Rule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if strings.TrimSpace(name) == "" || rules == nil {
		panic("basic: invalid style registration")
	}
	if r.frozen {
		panic("basic: style registration after theme construction: " + name)
	}
	for _, entry := range r.entries {
		if entry.name == name {
			panic("basic: duplicate style registration: " + name)
		}
	}
	r.entries = append(r.entries, basicRegistration{name, rules})
}
func (r *basicRegistry) snapshot() []basicRegistration {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
	entries := slices.Clone(r.entries)
	slices.SortFunc(entries, func(a, b basicRegistration) int { return strings.Compare(a.name, b.name) })
	return entries
}
