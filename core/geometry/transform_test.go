package geometry

import (
	"math"
	"testing"
)

func TestIdentity(t *testing.T) {
	id := Identity()
	p := Point{X: 3, Y: 4}
	got := id.TransformPoint(p)
	if got.X != p.X || got.Y != p.Y {
		t.Errorf("Identity.TransformPoint(%v) = %v, want %v", p, got, p)
	}
}

func TestTranslate(t *testing.T) {
	tr := Translate(10, 20)
	p := Point{X: 3, Y: 4}
	got := tr.TransformPoint(p)
	want := Point{X: 13, Y: 24}
	if got.X != want.X || got.Y != want.Y {
		t.Errorf("Translate(10,20).TransformPoint(%v) = %v, want %v", p, got, want)
	}
}

func TestScale(t *testing.T) {
	sc := Scale(2, 3)
	p := Point{X: 3, Y: 4}
	got := sc.TransformPoint(p)
	want := Point{X: 6, Y: 12}
	if got.X != want.X || got.Y != want.Y {
		t.Errorf("Scale(2,3).TransformPoint(%v) = %v, want %v", p, got, want)
	}
}

func TestRotate90(t *testing.T) {
	// Clockwise 90 degrees in Y-down: (1,0) -> (0,-1)
	rot := Rotate(90)
	p := Point{X: 1, Y: 0}
	got := rot.TransformPoint(p)
	want := Point{X: 0, Y: -1}
	eps := float32(1e-6)
	if math.Abs(float64(got.X-want.X)) > math.Abs(float64(eps)) || math.Abs(float64(got.Y-want.Y)) > math.Abs(float64(eps)) {
		t.Errorf("Rotate(90).TransformPoint(%v) = %v, want %v", p, got, want)
	}
}

func TestRotate180(t *testing.T) {
	rot := Rotate(180)
	p := Point{X: 3, Y: 4}
	got := rot.TransformPoint(p)
	want := Point{X: -3, Y: -4}
	eps := float32(1e-6)
	if math.Abs(float64(got.X-want.X)) > math.Abs(float64(eps)) || math.Abs(float64(got.Y-want.Y)) > math.Abs(float64(eps)) {
		t.Errorf("Rotate(180).TransformPoint(%v) = %v, want %v", p, got, want)
	}
}

func TestMultiply(t *testing.T) {
	// Multiply is standard matrix multiplication: a.Multiply(b) = a * b,
	// i.e. apply b first, then a.
	// Translate(10,0).Multiply(Rotate(90)) = rotate 90 clockwise first, then translate (10,0)
	// (1,0) -> rotate 90 -> (0,-1) -> translate -> (10,-1)
	tr := Translate(10, 0).Multiply(Rotate(90))
	p := Point{X: 1, Y: 0}
	got := tr.TransformPoint(p)
	want := Point{X: 10, Y: -1}
	eps := float32(1e-6)
	if math.Abs(float64(got.X-want.X)) > math.Abs(float64(eps)) || math.Abs(float64(got.Y-want.Y)) > math.Abs(float64(eps)) {
		t.Errorf("Translate(10,0).Multiply(Rotate(90)).TransformPoint(%v) = %v, want %v", p, got, want)
	}
}

func TestChainMethods(t *testing.T) {
	// Chain methods apply in local space: Translate(10,0).Rotate(90) means
	// rotate first, then translate.
	// (1,0) -> rotate 90 clockwise -> (0,-1) -> translate (10,0) -> (10,-1)
	tr := Translate(10, 0).Rotate(90)
	p := Point{X: 1, Y: 0}
	got := tr.TransformPoint(p)
	want := Point{X: 10, Y: -1}
	eps := float32(1e-6)
	if math.Abs(float64(got.X-want.X)) > math.Abs(float64(eps)) || math.Abs(float64(got.Y-want.Y)) > math.Abs(float64(eps)) {
		t.Errorf("chain TransformPoint(%v) = %v, want %v", p, got, want)
	}
}
