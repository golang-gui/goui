package utils

import (
	"unicode/utf16"
	"unicode/utf8"
)

type StringPosition struct {
	u8to16 []int
	u16to8 []int
}

func CalcStringPosition(text string) (p StringPosition) {
	if len(text) != 0 {
		p.u8to16 = make([]int, 0, len(text))
		p.u16to8 = make([]int, 0, len(text)/2)
		u8pos := 0
		u16pos := 0
		for _, r := range text {
			u8len := utf8.RuneLen(r)
			u16len := utf16.RuneLen(r)

			for i := 0; i < u8len; i++ {
				p.u8to16 = append(p.u8to16, u16pos)
			}
			for i := 0; i < u16len; i++ {
				p.u16to8 = append(p.u16to8, u8pos)
			}

			u8pos += u8len
			u16pos += u16len
		}
		p.u8to16 = append(p.u8to16, u16pos)
		p.u16to8 = append(p.u16to8, u8pos)
	}
	return
}

func (p StringPosition) ToUtf8(u16 int) int {
	if 0 <= u16 && u16 < len(p.u16to8) {
		return p.u16to8[u16]
	}
	return -1
}

func (p StringPosition) ToUtf16(u8 int) int {
	if 0 <= u8 && u8 < len(p.u8to16) {
		return p.u8to16[u8]
	}
	return -1
}
