// Package bits provides small, allocation-free bit-set helpers built on Go's
// fixed-width integer types.
package bits

import "unsafe"

// Bitmap is a fixed-width bit set backed by a single integer of type T. Each bit
// position is addressed by an index in the range [0, bits-in-T). It holds no
// pointers and needs no initialization: the zero value is an empty bitmap.
type Bitmap[T integer] struct {
	// Mask is the underlying bit storage. It is exported so callers can read or
	// persist the raw value; prefer the methods for index-safe access.
	Mask T
}

// Empty reports whether no bits are set.
func (b *Bitmap[T]) Empty() bool {
	return b.Mask == 0
}

// Clear unsets every bit, returning the bitmap to its zero (empty) state.
func (b *Bitmap[T]) Clear() {
	b.Mask = 0
}

// Set marks the bit at index on (mark true) or off (mark false). Out-of-range
// indices are ignored.
func (b *Bitmap[T]) Set(index int, mark bool) {
	if b.valid(index) {
		if mark {
			b.Mask |= 1 << index
		} else {
			b.Mask &^= 1 << index
		}
	}
}

// Check reports whether the bit at index is set. An out-of-range index reports
// false.
func (b *Bitmap[T]) Check(index int) (mark bool) {
	if b.valid(index) {
		return (b.Mask & (1 << index)) != 0
	}
	return false
}

// valid reports whether index addresses a real bit of T. The number of bits is
// the byte size of the mask times 8 (unsafe.Sizeof returns bytes, not bits).
func (b *Bitmap[T]) valid(index int) bool {
	return 0 <= index && index < int(unsafe.Sizeof(b.Mask)*8)
}

// integer lists the fixed-width integer types a Bitmap may be backed by.
type integer interface {
	int8 | int16 | int32 | int64 | uint8 | uint16 | uint32 | uint64
}
