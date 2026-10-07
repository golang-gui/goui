package animation

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/golang-gui/goui/core/signal"
)

type testSource struct {
	frames signal.Signal1[time.Time]
	paints int
	err    error
}

func (s *testSource) ConnectFrame(fn func(time.Time)) signal.Handle { return s.frames.Connect(fn) }
func (s *testSource) RequestPaint() error                           { s.paints++; return s.err }

func fixture() (*Animation[float64], *testSource, time.Time) {
	base := time.Unix(100, 0)
	a := New(float64(0), Float64)
	a.now = func() time.Time { return base }
	return a, new(testSource), base
}

func TestAnimationSamplesAndCompletion(t *testing.T) {
	a, s, base := fixture()
	var values []float64
	finishes := 0
	a.ConnectUpdate(func(v float64) { values = append(values, v) })
	a.ConnectFinished(func(err error) {
		if err != nil {
			t.Fatal(err)
		}
		finishes++
	})
	if err := a.AnimateTo(s, 8, time.Second, nil); err != nil {
		t.Fatal(err)
	}
	if s.paints != 1 || a.CurrentValue() != 0 {
		t.Fatal("start sampled or did not request first frame")
	}
	s.frames.Emit(base.Add(time.Second / 2))
	if a.CurrentValue() != 4 || s.paints != 2 || !a.Running() {
		t.Fatal("incorrect midpoint")
	}
	for range 5 {
		if a.CurrentValue() != 4 {
			t.Fatal("read advanced time")
		}
	}
	s.frames.Emit(base.Add(3 * time.Second))
	if a.CurrentValue() != 8 || a.Running() || finishes != 1 || s.paints != 2 {
		t.Fatal("missed frame did not finish exactly and stop requesting")
	}
	s.frames.Emit(base.Add(4 * time.Second))
	if len(values) != 2 || finishes != 1 {
		t.Fatal("completed callback remained connected")
	}
}

func TestAnimationReentrantOperations(t *testing.T) {
	for _, mode := range []string{"stop", "set", "replace", "finish-replace"} {
		t.Run(mode, func(t *testing.T) {
			a, s, base := fixture()
			finished := 0
			a.ConnectFinished(func(error) { finished++ })
			var h signal.Handle
			h = a.ConnectUpdate(func(float64) {
				h.Disconnect()
				switch mode {
				case "stop":
					a.Stop()
				case "set":
					a.SetValue(42)
				default:
					if err := a.AnimateTo(s, 20, time.Second, nil); err != nil {
						t.Fatal(err)
					}
				}
			})
			if err := a.AnimateTo(s, 10, time.Second, nil); err != nil {
				t.Fatal(err)
			}
			at := base.Add(time.Second / 2)
			if mode == "finish-replace" {
				at = base.Add(time.Second)
			}
			s.frames.Emit(at)
			if finished != 0 {
				t.Fatal("stale Finished")
			}
			if mode == "stop" && (a.Running() || s.paints != 1 || a.CurrentValue() != 5) {
				t.Fatal("stopped frame requested another paint")
			}
			if mode == "set" && (a.Running() || a.CurrentValue() != 42) {
				t.Fatal("SetValue overwritten")
			}
			if mode == "replace" || mode == "finish-replace" {
				if !a.Running() || s.paints != 2 {
					t.Fatal("replacement disconnected or old frame requested paint")
				}
				s.frames.Emit(base.Add(time.Second))
				if a.Running() || a.CurrentValue() != 20 || finished != 1 {
					t.Fatal("replacement did not finish")
				}
			}
		})
	}
}

func TestAnimationErrorsAndImmediate(t *testing.T) {
	a, s, base := fixture()
	failure := errors.New("schedule")
	var reported []error
	a.ConnectFinished(func(err error) { reported = append(reported, err) })
	if err := a.AnimateTo(s, 1, time.Second, nil); err != nil {
		t.Fatal(err)
	}
	if a.AnimateTo(nil, 2, time.Second, nil) == nil || a.AnimateTo(s, 2, -1, nil) == nil || !a.Running() {
		t.Fatal("invalid arguments changed playback")
	}
	s.err = failure
	s.frames.Emit(base.Add(time.Second / 2))
	if a.Running() || len(reported) != 1 || reported[0] != failure {
		t.Fatal("async failure not reported once")
	}
	reported = nil
	if a.AnimateTo(s, 2, time.Second, nil) != failure || a.Running() || len(reported) != 0 {
		t.Fatal("initial error duplicated")
	}
	s.err = nil
	if err := a.AnimateTo(s, 3, 0, nil); err != nil {
		t.Fatal(err)
	}
	if a.CurrentValue() != 3 || a.Running() || len(reported) != 1 || reported[0] != nil {
		t.Fatal("zero-duration transition")
	}
	if err := a.AnimateTo(s, 4, time.Second, func(float64) float64 { return math.NaN() }); err != nil {
		t.Fatal(err)
	}
	s.frames.Emit(base.Add(time.Second / 2))
	if a.Running() || len(reported) != 2 || reported[1] == nil {
		t.Fatal("non-finite curve not rejected")
	}
}

func TestAnimationFinishAndSharedSource(t *testing.T) {
	a, s, base := fixture()
	b := New(float64(0), Float64)
	b.now = a.now
	if err := a.AnimateTo(s, 10, time.Second, EaseOutCubic); err != nil {
		t.Fatal(err)
	}
	if err := b.AnimateTo(s, 10, 2*time.Second, nil); err != nil {
		t.Fatal(err)
	}
	s.frames.Emit(base.Add(time.Second / 2))
	if a.CurrentValue() != 8.75 || b.CurrentValue() != 2.5 {
		t.Fatal("independent animations did not share frame time")
	}
	a.Finish()
	if a.CurrentValue() != 10 || a.Running() || !b.Running() {
		t.Fatal("finish affected peer")
	}
	s.frames.Emit(base.Add(2 * time.Second))
	if b.CurrentValue() != 10 || b.Running() {
		t.Fatal("peer not completed")
	}
}

func TestCurvesAndCompositeInterpolation(t *testing.T) {
	for _, tc := range []struct {
		curve    Curve
		midpoint float64
	}{{Linear, .5}, {EaseInCubic, .125}, {EaseOutCubic, .875}, {EaseInOutCubic, .5}} {
		if tc.curve(0) != 0 || tc.curve(.5) != tc.midpoint || tc.curve(1) != 1 {
			t.Fatal("curve endpoints/midpoint")
		}
	}
	if Float32(2, 6, .25) != 3 || Float64(2, 6, 1.5) != 8 {
		t.Fatal("interpolation/extrapolation")
	}
	type pair struct{ x, y float64 }
	a := New(pair{}, func(f, to pair, p float64) pair { return pair{Float64(f.x, to.x, p), Float64(f.y, to.y, p)} })
	s := new(testSource)
	base := time.Unix(100, 0)
	a.now = func() time.Time { return base }
	if err := a.AnimateTo(s, pair{2, 4}, time.Second, Linear); err != nil {
		t.Fatal(err)
	}
	s.frames.Emit(base.Add(time.Second / 2))
	if a.CurrentValue() != (pair{1, 2}) {
		t.Fatal("composite values")
	}
}
