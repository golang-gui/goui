package diagnostic

import (
	"errors"
	"slices"
	"sync/atomic"
	"time"

	"github.com/golang-gui/goui/core/signal"
)

const HistoryLimit = 120

// GUIData contains detached values prepared by the GUI's private adapter.
type GUIData struct {
	Roots                        []Root
	Widgets                      []Widget
	DroppedRoots, DroppedWidgets uint64
}

// Source is an internal GUI-thread query bridge, not a user callback or event
// observer. The GUI attaches it lazily; queries never schedule GUI work.
type Source interface {
	Snapshot(time.Time) GUIData
	ReadRoot(any, time.Time) (RootSample, bool)
}

// RootSample borrows no history or live GUI objects. Frame is the most recent
// completed frame, so an in-flight AfterPaint never reads its own unfinished work.
type RootSample struct {
	Root  Root
	Frame Frame
}

// Session owns one capture. Only operation counters may be recorded from other
// threads; ownership, source, frame history and snapshots are GUI-thread-only.
type Session struct {
	values      [Count]atomic.Uint64
	owner       any
	options     Options
	started     time.Time
	active      bool
	source      Source
	serial      uint64
	history     [HistoryLimit]Frame
	next, count int
	frozen      Snapshot
	stopped     signal.Signal0
}

var current atomic.Pointer[Session]

// Start enables resource-only collection, without installing a GUI source.
func Start() (*Session, error) { return StartCapture(nil, Options{}) }

func StartCapture(owner any, options Options) (*Session, error) {
	s := &Session{owner: owner, options: options, started: time.Now(), active: true}
	if !current.CompareAndSwap(nil, s) {
		return nil, errors.New("diagnostics already active")
	}
	return s, nil
}

// Current is the only recording gate. A nil result needs no clock or map access.
func Current() *Session { return current.Load() }

func (s *Session) Owner() any              { return s.owner }
func (s *Session) WidgetsEnabled() bool    { return s.options.Widgets }
func (s *Session) Active() bool            { return s.active }
func (s *Session) Source() Source          { return s.source }
func (s *Session) SetSource(source Source) { s.source = source }
func (s *Session) NextRef() uint64         { s.serial++; return s.serial }

// ConnectStop releases optional observers on both explicit Stop and owner exit.
// Call only while active, on the GUI thread.
func (s *Session) ConnectStop(fn func()) signal.Handle { return s.stopped.Connect(fn) }

// ReadRoot reads one existing record without copying the capture's history or
// enumerating widgets. It does not create a collector or a root record.
func (s *Session) ReadRoot(target any) (RootSample, bool) {
	if s == nil || !s.active || s.source == nil {
		return RootSample{}, false
	}
	return s.source.ReadRoot(target, time.Now())
}

func StopOwner(owner any) {
	if s := Current(); s != nil && s.owner == owner {
		s.Stop()
	}
}

// Stop freezes values and drops the GUI adapter and owner. In-flight native
// increments can finish on the old counters but cannot change the frozen copy.
func (s *Session) Stop() {
	if s == nil || !s.active {
		return
	}
	current.CompareAndSwap(s, nil)
	s.active = false
	s.frozen = s.snapshot(time.Now())
	s.source, s.owner = nil, nil
	s.stopped.Emit()
}

func (s *Session) Snapshot() Snapshot {
	if s == nil {
		return Snapshot{}
	}
	if !s.active {
		out := s.frozen
		out.Roots, out.Widgets, out.Frames = slices.Clone(out.Roots), slices.Clone(out.Widgets), slices.Clone(out.Frames)
		return out
	}
	return s.snapshot(time.Now())
}

func (s *Session) snapshot(now time.Time) Snapshot {
	out := Snapshot{Active: s.active, Started: s.started, Captured: now, Options: s.options, Resources: resourcesFromValues(s.Values())}
	if s.source != nil {
		data := s.source.Snapshot(now)
		out.Roots, out.Widgets = data.Roots, data.Widgets
		out.DroppedRoots, out.DroppedWidgets = data.DroppedRoots, data.DroppedWidgets
	}
	for i := 0; i < s.count; i++ {
		out.Frames = append(out.Frames, s.history[(s.next-s.count+i+len(s.history))%len(s.history)])
	}
	return out
}

// RootRecord holds generic frame aggregates; GUI identities and weak references
// stay in the GUI adapter. A frame reuses current instead of allocating a span.
type RootRecord struct {
	Root
	current     FrameRecord
	last        Frame
	times       [HistoryLimit]time.Time
	next, count int
}

type FrameRecord struct {
	Frame
	owner *Session
	root  *RootRecord
}

func (s *Session) BeginFrame(r *RootRecord) *FrameRecord {
	r.current = FrameRecord{Frame: Frame{RootRef: r.Ref, Started: time.Now()}, owner: s, root: r}
	return &r.current
}

func (f *FrameRecord) Finish() {
	if !f.owner.active {
		return
	}
	now := time.Now()
	f.Total = now.Sub(f.Started)
	r, s := f.root, f.owner
	r.Frames++
	if f.Completed {
		r.Completed++
		r.last = f.Frame
		if r.count > 0 && now.Sub(r.times[(r.next-1+len(r.times))%len(r.times)]) > time.Second {
			r.count = 0
		}
		r.times[r.next] = now
		r.next = (r.next + 1) % len(r.times)
		r.count = min(r.count+1, len(r.times))
	} else {
		r.Aborted++
	}
	s.history[s.next] = f.Frame
	s.next = (s.next + 1) % len(s.history)
	s.count = min(s.count+1, len(s.history))
}

func (r *RootRecord) LastFrame() Frame { return r.last }

func (r *RootRecord) Snapshot(now time.Time, closed, suspended bool) Root {
	out := r.Root
	out.Closed = closed
	out.Idle = closed || suspended || r.count == 0 || now.Sub(r.times[(r.next-1+len(r.times))%len(r.times)]) > time.Second
	if !out.Idle && r.count > 1 {
		first, last := r.times[(r.next-r.count+len(r.times))%len(r.times)], r.times[(r.next-1+len(r.times))%len(r.times)]
		if last.After(first) {
			out.FPS = float64(r.count-1) / last.Sub(first).Seconds()
		}
	}
	return out
}
