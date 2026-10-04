package identity

import "testing"

func TestSamePointer(t *testing.T) {
	type value struct{ data any }
	a, b := &value{42}, &value{42}
	sliceValue := value{[]int{1}}
	mapValue := value{map[string]int{"one": 1}}
	funcValue := value{func() {}}
	var typedNil *value
	for _, tc := range []struct {
		name string
		a, b any
		want bool
	}{
		{"nil", nil, nil, true},
		{"one nil", a, nil, false},
		{"same pointer", a, a, true},
		{"equal fields distinct instances", a, b, false},
		{"different pointer types", a, new(int), false},
		{"equal values", *a, *b, false},
		{"slice in interface field", sliceValue, sliceValue, false},
		{"map in interface field", mapValue, mapValue, false},
		{"function in interface field", funcValue, funcValue, false},
		{"pointer to non-comparable value", &sliceValue, &sliceValue, true},
		{"slice", []int{1}, []int{1}, false},
		{"scalar", 42, 42, false},
		{"typed nil", typedNil, typedNil, false},
		{"typed nil versus nil", typedNil, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SamePointer(tc.a, tc.b); got != tc.want {
				t.Fatalf("SamePointer(%T, %T)=%v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
