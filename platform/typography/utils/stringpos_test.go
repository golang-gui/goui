package utils

import "testing"

func TestStringPositionEmptyAndEnd(t *testing.T) {
	for _, tc := range []struct {
		text  string
		units int
	}{{"", 0}, {"a中😀", 4}} {
		p := CalcStringPosition(tc.text)
		if got := p.ToUtf8(tc.units); got != len(tc.text) {
			t.Errorf("%q end UTF-8=%d want %d", tc.text, got, len(tc.text))
		}
		if got := p.ToUtf16(len(tc.text)); got != tc.units {
			t.Errorf("%q end UTF-16=%d want %d", tc.text, got, tc.units)
		}
		if p.ToUtf8(tc.units+1) != -1 || p.ToUtf16(len(tc.text)+1) != -1 {
			t.Errorf("%q accepted out-of-range position", tc.text)
		}
	}
}

func Test_StringPos(t *testing.T) {
	us := CalcStringPosition("abc这是一段中文")
	t.Log(us.ToUtf16(1))
	t.Log(us.ToUtf16(4))
	t.Log(us.ToUtf16(5))
	t.Log(us.ToUtf16(6))
	t.Log(us.ToUtf16(999))
	t.Log(us.ToUtf8(3))
	t.Log(us.ToUtf8(5))
	t.Log(us.ToUtf8(66))
	t.Log(us.ToUtf8(2))
	t.Log(us.ToUtf8(5))
	t.Log(us.ToUtf8(8))
	t.Log(us.ToUtf8(999))
}
