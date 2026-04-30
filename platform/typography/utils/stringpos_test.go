package utils

import "testing"

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
