package bits

import "testing"

func TestBitmapZeroValueIsEmpty(t *testing.T) {
	var b Bitmap[uint8]
	if !b.Empty() {
		t.Fatal("zero-value bitmap should be empty")
	}
	if b.Check(0) {
		t.Fatal("zero-value bitmap should have no bits set")
	}
}

func TestBitmapSetAndCheck(t *testing.T) {
	var b Bitmap[uint8]

	b.Set(0, true)
	b.Set(3, true)
	b.Set(7, true)

	for _, i := range []int{0, 3, 7} {
		if !b.Check(i) {
			t.Fatalf("bit %d should be set", i)
		}
	}
	for _, i := range []int{1, 2, 4, 5, 6} {
		if b.Check(i) {
			t.Fatalf("bit %d should not be set", i)
		}
	}
	if b.Empty() {
		t.Fatal("bitmap with bits set should not be empty")
	}
}

func TestBitmapUnset(t *testing.T) {
	var b Bitmap[uint8]
	b.Set(5, true)
	if !b.Check(5) {
		t.Fatal("bit 5 should be set")
	}

	b.Set(5, false)
	if b.Check(5) {
		t.Fatal("bit 5 should be cleared after Set(5, false)")
	}
	if !b.Empty() {
		t.Fatal("bitmap should be empty after clearing its only bit")
	}
}

func TestBitmapClear(t *testing.T) {
	var b Bitmap[uint16]
	b.Set(1, true)
	b.Set(9, true)
	b.Clear()
	if !b.Empty() {
		t.Fatal("Clear should empty the bitmap")
	}
	if b.Check(1) || b.Check(9) {
		t.Fatal("no bits should remain after Clear")
	}
}

// TestBitmapHighestBit guards the unsafe.Sizeof-in-bits fix: the top bit of each
// width must be addressable (index = bits-1).
func TestBitmapHighestBit(t *testing.T) {
	var b8 Bitmap[uint8]
	b8.Set(7, true)
	if !b8.Check(7) {
		t.Fatal("uint8 bit 7 (highest) should be settable")
	}

	var b64 Bitmap[uint64]
	b64.Set(63, true)
	if !b64.Check(63) {
		t.Fatal("uint64 bit 63 (highest) should be settable")
	}
}

// TestBitmapOutOfRange verifies that out-of-range indices are ignored rather
// than panicking or corrupting neighboring bits.
func TestBitmapOutOfRange(t *testing.T) {
	var b Bitmap[uint8] // 8 bits: valid indices 0..7

	b.Set(8, true) // just past the top bit
	b.Set(100, true)
	b.Set(-1, true)
	if !b.Empty() {
		t.Fatal("out-of-range Set calls must not set any bit")
	}
	if b.Check(8) || b.Check(-1) || b.Check(100) {
		t.Fatal("out-of-range Check must report false")
	}
}

func TestBitmapWidths(t *testing.T) {
	// Each width should expose exactly its bit count as valid indices.
	var b16 Bitmap[uint16]
	b16.Set(15, true)
	b16.Set(16, true) // out of range for 16 bits
	if !b16.Check(15) {
		t.Fatal("uint16 bit 15 should be set")
	}
	if b16.Check(16) {
		t.Fatal("uint16 has no bit 16")
	}

	var b32 Bitmap[int32]
	b32.Set(31, true)
	if !b32.Check(31) {
		t.Fatal("int32 bit 31 should be set")
	}
}
