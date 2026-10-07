// Package animation provides frame-driven value transitions. It depends on a
// small frame source, allowing GUI roots and deterministic test sources alike.
package animation

import (
	"errors"
	"math"
	"time"

	"github.com/golang-gui/goui/core/signal"
)

// FrameSource emits a shared sampling time before layout and paint. Connecting
// is passive; RequestPaint schedules the next frame and may coalesce requests.
// Its methods and notifications must run on the same owning thread.
type FrameSource interface {
	ConnectFrame(func(time.Time)) signal.Handle
	RequestPaint() error
}

// Interpolator calculates a value at the curve-adjusted progress. It should be
// pure and must support extrapolation if the supplied Curve overshoots.
type Interpolator[T any] func(from, to T, progress float64) T

// Curve maps elapsed progress to interpolation progress. Nil means Linear.
// Overshoot is allowed; NaN and infinity terminate that playback with an error.
type Curve func(progress float64) float64

// Animation holds the last sampled value. All operations and callbacks belong
// to the frame source's thread. Stop when the owner unmounts or is destroyed;
// a connected source retains the animation. Do not copy after use.
type Animation[T any] struct {
	value, from, target T
	interpolate         Interpolator[T]
	curve               Curve
	source              FrameSource
	handle              signal.Handle
	started             time.Time
	duration            time.Duration
	running             bool
	version             uint64
	now                 func() time.Time
	update              signal.Signal1[T]
	finished            signal.Signal1[error]
}

// New creates an idle animation without connecting or requesting frames.
// A nil interpolator is a programming error and panics.
func New[T any](initial T, interpolate Interpolator[T]) *Animation[T] {
	if interpolate == nil {
		panic("animation: nil interpolator")
	}
	return &Animation[T]{value: initial, interpolate: interpolate, now: time.Now}
}

// CurrentValue reads the cached sample without advancing time or notifying.
func (a *Animation[T]) CurrentValue() T { return a.value }

// Running reports playback intent, including while a hidden source is quiet.
func (a *Animation[T]) Running() bool { return a.running }

func (a *Animation[T]) ConnectUpdate(fn func(T)) signal.Handle { return a.update.Connect(fn) }

// ConnectFinished reports arrival at the target (nil) or a subsequent frame
// request/curve failure. Stop and replacement do not emit this signal. Success
// describes the value transition, not actual presentation on a display.
func (a *Animation[T]) ConnectFinished(fn func(error)) signal.Handle { return a.finished.Connect(fn) }

// AnimateTo replaces playback from the cached value. Invalid arguments leave
// playback intact. Zero duration jumps to target; nil curve means Linear.
// An initial scheduling failure is returned, never also emitted as Finished.
func (a *Animation[T]) AnimateTo(source FrameSource, target T, duration time.Duration, curve Curve) error {
	if source == nil || duration < 0 {
		return errors.New("animation: source must be non-nil and duration non-negative")
	}
	a.Stop()
	a.from, a.target, a.source = a.value, target, source
	a.duration, a.started, a.curve = duration, a.now(), curve
	if a.curve == nil {
		a.curve = Linear
	}
	version := a.version
	if duration == 0 {
		a.source = nil
		a.value = target
		a.update.Emit(target)
		if a.version != version {
			return nil
		}
		if err := source.RequestPaint(); err != nil {
			return err
		}
		if a.version == version {
			a.finished.Emit(nil)
		}
		return nil
	}
	a.running = true
	a.handle = source.ConnectFrame(func(now time.Time) { a.advance(version, now) })
	if err := source.RequestPaint(); err != nil {
		if a.version == version {
			a.Stop()
		}
		return err
	}
	return nil
}

// SetValue interrupts playback, assigns the value and emits Update. Consumers
// apply it through their usual setters/state and invalidation mechanisms.
func (a *Animation[T]) SetValue(value T) {
	a.Stop()
	a.value = value
	a.update.Emit(value)
}

// Stop preserves the last sample and disconnects. It is safe from callbacks.
func (a *Animation[T]) Stop() {
	a.version++
	a.disconnect()
}

func (a *Animation[T]) disconnect() {
	a.running = false
	if a.handle != nil {
		a.handle.Disconnect()
		a.handle = nil
	}
	a.source = nil
}

// Finish jumps a running animation to its target, requests a final paint and
// emits Finished. It does nothing while idle.
func (a *Animation[T]) Finish() {
	if !a.running {
		return
	}
	source, target := a.source, a.target
	a.version++
	version := a.version
	a.disconnect()
	a.value = target
	a.update.Emit(target)
	if a.version == version {
		err := source.RequestPaint()
		if a.version == version {
			a.finished.Emit(err)
		}
	}
}

func (a *Animation[T]) advance(version uint64, now time.Time) {
	if !a.running || a.version != version {
		return
	}
	progress := min(1, max(0, float64(now.Sub(a.started))/float64(a.duration)))
	if progress >= 1 {
		target := a.target
		a.disconnect()
		a.value = target
		a.update.Emit(target)
		if a.version == version {
			a.finished.Emit(nil)
		}
		return
	}
	progress = a.curve(progress)
	if !a.running || a.version != version {
		return
	}
	if math.IsNaN(progress) || math.IsInf(progress, 0) {
		a.disconnect()
		a.finished.Emit(errors.New("animation: curve returned non-finite progress"))
		return
	}
	value := a.interpolate(a.from, a.target, progress)
	if !a.running || a.version != version {
		return
	}
	a.value = value
	a.update.Emit(value)
	if !a.running || a.version != version {
		return
	}
	if err := a.source.RequestPaint(); err != nil && a.version == version {
		a.disconnect()
		a.finished.Emit(err)
	}
}
