package modern

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
func Register(name string, rules func(options Options, base style.StyleSheet) []style.Rule) {
	registered.add(name, rules)
}

type modernRegistration struct {
	name  string
	rules func(options Options, base style.StyleSheet) []style.Rule
}
type modernRegistry struct {
	mu      sync.Mutex
	frozen  bool
	entries []modernRegistration
}

var registered modernRegistry

func (r *modernRegistry) add(name string, rules func(options Options, base style.StyleSheet) []style.Rule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if strings.TrimSpace(name) == "" || rules == nil {
		panic("modern: invalid style registration")
	}
	if r.frozen {
		panic("modern: style registration after theme construction: " + name)
	}
	for _, entry := range r.entries {
		if entry.name == name {
			panic("modern: duplicate style registration: " + name)
		}
	}
	r.entries = append(r.entries, modernRegistration{name, rules})
}
func (r *modernRegistry) snapshot() []modernRegistration {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
	entries := slices.Clone(r.entries)
	slices.SortFunc(entries, func(a, b modernRegistration) int { return strings.Compare(a.name, b.name) })
	return entries
}
