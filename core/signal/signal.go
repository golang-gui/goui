package signal

import (
	"sync"
	"sync/atomic"
)

type Handle interface {
	Block()
	Unblock()
	Disconnect()
}

type Handles []Handle

func (h Handles) Block() {
	for _, handle := range h {
		if handle != nil {
			handle.Block()
		}
	}
}

func (h Handles) Unblock() {
	for _, handle := range h {
		if handle != nil {
			handle.Unblock()
		}
	}
}

func (h Handles) Disconnect() {
	for _, handle := range h {
		if handle != nil {
			handle.Disconnect()
		}
	}
}

type Signal0 struct {
	signalBase[func()]
}

func (s *Signal0) Connect(fn func()) Handle {
	return s.connect(fn)
}

func (s *Signal0) Emit() {
	for _, slot := range s.snapshot() {
		if slot.callable() {
			slot.fn()
		}
	}
}

type Signal1[T any] struct {
	signalBase[func(T)]
}

func (s *Signal1[T]) Connect(fn func(T)) Handle {
	return s.connect(fn)
}

func (s *Signal1[T]) Emit(arg T) {
	for _, slot := range s.snapshot() {
		if slot.callable() {
			slot.fn(arg)
		}
	}
}

type Signal2[T1, T2 any] struct {
	signalBase[func(T1, T2)]
}

func (s *Signal2[T1, T2]) Connect(fn func(T1, T2)) Handle {
	return s.connect(fn)
}

func (s *Signal2[T1, T2]) Emit(arg1 T1, arg2 T2) {
	for _, slot := range s.snapshot() {
		if slot.callable() {
			slot.fn(arg1, arg2)
		}
	}
}

type Signal3[T1, T2, T3 any] struct {
	signalBase[func(T1, T2, T3)]
}

func (s *Signal3[T1, T2, T3]) Connect(fn func(T1, T2, T3)) Handle {
	return s.connect(fn)
}

func (s *Signal3[T1, T2, T3]) Emit(arg1 T1, arg2 T2, arg3 T3) {
	for _, slot := range s.snapshot() {
		if slot.callable() {
			slot.fn(arg1, arg2, arg3)
		}
	}
}

type signalBase[F any] struct {
	mu    sync.Mutex
	slots []*slot[F]
}

func (s *signalBase[F]) connect(fn F) Handle {
	slot := &slot[F]{
		signal: s,
		fn:     fn,
	}

	s.mu.Lock()
	s.slots = append(s.slots, slot)
	s.mu.Unlock()
	return slot
}

func (s *signalBase[F]) disconnect(slot *slot[F]) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, current := range s.slots {
		if current == slot {
			s.slots = append(s.slots[:i], s.slots[i+1:]...)
			return
		}
	}
}

func (s *signalBase[F]) snapshot() []*slot[F] {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]*slot[F](nil), s.slots...)
}

type slot[F any] struct {
	signal       *signalBase[F]
	fn           F
	blockCount   atomic.Int64
	disconnected atomic.Bool
}

func (s *slot[F]) Block() {
	s.blockCount.Add(1)
}

func (s *slot[F]) Unblock() {
	for {
		current := s.blockCount.Load()
		if current == 0 {
			return
		}
		if s.blockCount.CompareAndSwap(current, current-1) {
			return
		}
	}
}

func (s *slot[F]) Disconnect() {
	if s.disconnected.CompareAndSwap(false, true) {
		s.signal.disconnect(s)
	}
}

func (s *slot[F]) callable() bool {
	return !s.disconnected.Load() && s.blockCount.Load() == 0
}
