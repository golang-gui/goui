// Package optional provides a small generic container that distinguishes "a
// value is present" from "no value", so a zero value (0, "", nil, ...) can be
// told apart from an unset one. This matters in declarative APIs where "field
// not provided" and "field explicitly set to the zero value" must map to
// different behavior.
package optional

// Optional holds either a value of type T or nothing. The zero value is empty
// (no value). Construct a present value with Value and an empty one with Empty.
type Optional[T any] struct {
	value T
	has   bool
}

// Value returns an Optional holding value (present).
func Value[T any](value T) Optional[T] {
	return Optional[T]{value: value, has: true}
}

// Empty returns an Optional holding nothing. It is identical to the zero value.
func Empty[T any]() Optional[T] {
	return Optional[T]{}
}

// HasValue reports whether a value is present.
func (o *Optional[T]) HasValue() bool {
	return o.has
}

// SetValue stores value and marks the Optional present.
func (o *Optional[T]) SetValue(value T) {
	o.value = value
	o.has = true
}

// Value returns the stored value. When empty it returns the zero value of T;
// call HasValue to tell an empty Optional from one holding the zero value.
func (o *Optional[T]) Value() T {
	return o.value
}

// ValueOr returns the stored value when present, otherwise defValue.
func (o *Optional[T]) ValueOr(defValue T) T {
	if o.has {
		return o.value
	}
	return defValue
}
