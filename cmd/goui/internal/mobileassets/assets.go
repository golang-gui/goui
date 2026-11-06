// Package mobileassets contains the versioned native inputs embedded in goui.
// Maintainers regenerate these inputs; application builds only unpack them.
package mobileassets

//go:generate go run generate.go

type Blob struct {
	Data   string
	SHA256 string
	Files  map[string]string
}

var bases = map[string]map[string]Blob{}
var targets = map[string]Blob{}

// Select returns shared platform data and exact-version Go support archives.
func Select(version, target, variant string) ([]Blob, bool) {
	key := target + "/" + variant
	base, ok := bases[version][key]
	if !ok {
		return nil, false
	}
	shared, ok := targets[key]
	if !ok {
		return nil, false
	}
	return []Blob{shared, base}, true
}
