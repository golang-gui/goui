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
var objectPools = map[string]Blob{}

// Target returns platform resources independently of any Go support version.
func Target(target, variant string) (Blob, bool) {
	b, ok := targets[target+"/"+variant]
	return b, ok
}

// Select returns target inputs, a shared content-addressed object pool and the
// selected exact-version recipe. Legacy embedded archive data remains readable.
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
	if objects, ok := objectPools[key]; ok {
		return []Blob{shared, objects, base}, true
	}
	return []Blob{shared, base}, true
}
