package main

import (
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
)

func TestParseControlsPosition(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  *geometry.Point
	}{
		{"", nil},
		{"0,0", &geometry.Point{}},
		{"16.5, 18", &geometry.Point{X: 16.5, Y: 18}},
	} {
		got, err := parseControlsPosition(tc.value)
		if err != nil || (got == nil) != (tc.want == nil) || got != nil && *got != *tc.want {
			t.Errorf("parse %q: got %v, %v; want %v", tc.value, got, err, tc.want)
		}
	}
	for _, value := range []string{"16", "16,", "a,18", "16,18,20"} {
		if _, err := parseControlsPosition(value); err == nil {
			t.Errorf("accepted malformed position %q", value)
		}
	}
}

func TestHeaderMoveRegions(t *testing.T) {
	d := demo{size: events.SizeEvent{Width: 800, Height: 560}, controls: geometry.Rect(16, 18, 54, 14), allowMove: true}
	for _, point := range []geometry.Point{{X: 40, Y: 20}, {X: 100, Y: 20}} {
		if hit := d.hitTest(point); hit != platform.WindowHitClient {
			t.Errorf("interactive header point %v returned %v", point, hit)
		}
	}
	blank := geometry.Point{X: 300, Y: 20}
	if hit := d.hitTest(blank); hit != platform.WindowHitCaption {
		t.Errorf("blank header returned %v", hit)
	}
	d.allowMove = false
	if hit := d.hitTest(blank); hit == platform.WindowHitCaption {
		t.Fatal("disabled sample dragging still exposes a caption role")
	}
}
