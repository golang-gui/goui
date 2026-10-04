// Package identity provides reference checks for declarative adapters.
// It does not define equality for model contents or arbitrary Go values.
package identity

import "reflect"

// SamePointer reports whether both arguments are nil interfaces, or hold the
// same non-nil pointer of the same type. Values without pointer identity are
// never considered the same, even when their fields compare equal. Typed nil
// pointers are not valid model instances and are not normalized to nil.
func SamePointer(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	x, y := reflect.ValueOf(a), reflect.ValueOf(b)
	return x.Kind() == reflect.Pointer && y.Kind() == reflect.Pointer &&
		x.Type() == y.Type() && !x.IsNil() && !y.IsNil() && a == b
}
