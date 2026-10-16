package directwrite

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/platform/typography/utils"
)

func TestFullColorReplacementDropsObsoleteRanges(t *testing.T) {
	l := &TextLayout{text: "A中🙂Z", position: utils.CalcStringPosition("A中🙂Z")}
	for i := 0; i < 1000; i++ {
		l.SetTextColor(0, len(l.text), color.Black)
		l.SetTextColor(1, 3, color.White)
		if len(l.colors) != 2 {
			t.Fatalf("full overwrite retained obsolete ranges at update %d: %d", i, len(l.colors))
		}
		if l.colors[1].Range.StartPosition != 1 || l.colors[1].Range.Length != 1 {
			t.Fatal("UTF-8 range was not converted to UTF-16", l.colors[1].Range)
		}
	}
}
