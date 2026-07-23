package optional

import "testing"

func TestZeroValueIsEmpty(t *testing.T) {
	var o Optional[int]
	if o.HasValue() {
		t.Fatal("zero-value Optional should be empty")
	}
	if o.Value() != 0 {
		t.Fatalf("empty Optional[int].Value() should be 0, got %d", o.Value())
	}
}

func TestEmpty(t *testing.T) {
	o := Empty[string]()
	if o.HasValue() {
		t.Fatal("Empty() should not have a value")
	}
	if o.Value() != "" {
		t.Fatalf("Empty[string]().Value() should be \"\", got %q", o.Value())
	}
}

func TestValue(t *testing.T) {
	o := Value(42)
	if !o.HasValue() {
		t.Fatal("Value(42) should have a value")
	}
	if o.Value() != 42 {
		t.Fatalf("Value() should be 42, got %d", o.Value())
	}
}

// TestZeroValueIsPresent is the whole point of Optional: a stored zero value is
// distinguishable from an empty Optional.
func TestZeroValueIsPresent(t *testing.T) {
	o := Value(0)
	if !o.HasValue() {
		t.Fatal("Value(0) must be present, not confused with empty")
	}
	if o.Value() != 0 {
		t.Fatalf("Value(0).Value() should be 0, got %d", o.Value())
	}

	empty := Empty[int]()
	if empty.HasValue() {
		t.Fatal("Empty[int]() must be absent even though its value is also 0")
	}
}

func TestSetValue(t *testing.T) {
	var o Optional[string]
	o.SetValue("hello")
	if !o.HasValue() {
		t.Fatal("SetValue should mark the Optional present")
	}
	if o.Value() != "hello" {
		t.Fatalf("Value() should be \"hello\", got %q", o.Value())
	}

	o.SetValue("")
	if !o.HasValue() {
		t.Fatal("SetValue(\"\") should keep the Optional present")
	}
	if o.Value() != "" {
		t.Fatalf("Value() should be \"\" after SetValue(\"\"), got %q", o.Value())
	}
}

func TestValueOr(t *testing.T) {
	empty := Empty[int]()
	if got := empty.ValueOr(99); got != 99 {
		t.Fatalf("empty.ValueOr(99) should be 99, got %d", got)
	}

	present := Value(7)
	if got := present.ValueOr(99); got != 7 {
		t.Fatalf("Value(7).ValueOr(99) should be 7, got %d", got)
	}

	// A present zero value must win over the default.
	zero := Value(0)
	if got := zero.ValueOr(99); got != 0 {
		t.Fatalf("Value(0).ValueOr(99) should be 0, got %d", got)
	}
}

func TestPointerType(t *testing.T) {
	// Optional over a pointer: a present nil pointer differs from empty.
	o := Value[*int](nil)
	if !o.HasValue() {
		t.Fatal("Value[*int](nil) should be present")
	}
	if o.Value() != nil {
		t.Fatal("stored value should be nil")
	}

	empty := Empty[*int]()
	if empty.HasValue() {
		t.Fatal("Empty[*int]() should be absent")
	}
}
