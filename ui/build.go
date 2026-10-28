package ui

import "slices"

// BuildView is a composition whose content is produced by a function.
// It adds no GUI widget or layout boundary.
type BuildView struct {
	ComposeBase[BuildView]
	build func() View
}

// Build declares a function-built subtree. fn runs on the UI thread at mount,
// when the parent updates this declaration, or when an observed source changes.
// Values must be read inside fn; Go closure capture rules are unchanged.
func Build(fn func() View) *BuildView {
	if fn == nil {
		panic("ui: Build requires a function")
	}
	v := &BuildView{build: fn}
	v.Self = v
	return v
}
func (v *BuildView) Build(BuildContext) View { return v.build() }

// Observe replaces this composition's explicit change sources. Empty clears
// the list. Dependencies belong to each mounted node; IDs are not required.
// Sources are stable, non-nil pointers. Get does not infer dependencies.
func (b *ComposeBase[T]) Observe(sources ...Observable) *T {
	for _, source := range sources {
		validateObservable(source)
	}
	b.observables = slices.Clone(sources)
	if b.Self == nil {
		panic("ui: composition not initialized (ComposeBase.Self is nil)")
	}
	return b.Self
}
