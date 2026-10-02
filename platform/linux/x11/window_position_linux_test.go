package x11

import (
	"errors"
	"math"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

func TestPositionMessage(t *testing.T) {
	m := positionMessage(42, 71, -200, 350)
	e := m.ClientMessageEvent()
	if e.Type != xlib.ClientMessage || e.Window != 42 || e.MessageType != 71 || e.Format != 32 {
		t.Fatalf("invalid message header: %+v", e)
	}
	// EWMH: Static gravity (10), x/y bits (8/9), source (12), no size.
	if e.L != [5]int64{0x130a, -200, 350, 0, 0} {
		t.Fatalf("invalid positioning payload: %v", e.L)
	}
}

func TestPositionHintsSurviveMinimumChanges(t *testing.T) {
	w := &Window{initialPosition: &[2]int32{-80, 120}, minW: 200, minH: 100}
	h := w.normalHints(2)
	if h.Flags != xlib.PMinSize|xlib.PPosition|xlib.PWinGravity || h.MinWidth != 400 || h.MinHeight != 200 {
		t.Fatalf("missing hints: %+v", h)
	}
	w.minW, w.minH = 0, 0
	h = w.normalHints(2)
	if h.Flags != xlib.PPosition|xlib.PWinGravity || h.X != -80 || h.Y != 120 || h.WinGravity != 10 {
		t.Fatalf("clearing minimum lost placement: %+v", h)
	}
	w.initialPosition = nil
	if w.normalHints(2).Flags != 0 {
		t.Fatal("empty hints not cleared")
	}
}

func TestInitialClientPositionUsesStaticGravityAndSurvivesMinimumChanges(t *testing.T) {
	w := &Window{initialPosition: &[2]int32{120, 250}, minW: 160}
	for _, minimum := range []float32{160, 0, 240} {
		w.minW = minimum
		h := w.normalHints(2)
		if h.WinGravity != 10 || h.X != 120 || h.Y != 250 || h.Flags&xlib.PPosition == 0 {
			t.Fatalf("client placement lost across minimum update: %+v", h)
		}
	}
}

func TestPositionUnavailableAndInvalid(t *testing.T) {
	w := &Window{}
	if _, err := w.Position(nil); !errors.Is(err, common.ErrUnavailable) {
		t.Fatal(err)
	}
	if err := w.SetPosition(nil, geometry.Point{}); !errors.Is(err, common.ErrUnavailable) {
		t.Fatal(err)
	}
	for _, v := range []float32{float32(math.NaN()), float32(math.Inf(1))} {
		if err := w.SetPosition(nil, geometry.Point{X: v}); err == nil || errors.Is(err, common.ErrUnavailable) {
			t.Fatalf("invalid point was not rejected: %v", err)
		}
	}
}
