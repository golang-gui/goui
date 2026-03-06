package cgo

import (
	"sync"
	"sync/atomic"
)

type LazyLibrary struct {
	mtx    sync.Mutex
	handle uintptr
	name   string
}

func NewLazyLibrary(name string) *LazyLibrary {
	return &LazyLibrary{name: name}
}

func (l *LazyLibrary) Load() error {
	if atomic.LoadUintptr(&l.handle) == 0 {
		l.mtx.Lock()
		defer l.mtx.Unlock()
		if l.handle == 0 {
			lib, err := Dlopen(l.name, RTLD_NOW)
			if err != nil {
				return err
			}
			atomic.StoreUintptr(&l.handle, lib)
		}
	}
	return nil
}

func (l *LazyLibrary) NewSymbol(name string) *LazySymbol {
	return &LazySymbol{lib: l, name: name}
}

type LazySymbol struct {
	mtx  sync.Mutex
	sym  uintptr
	lib  *LazyLibrary
	name string
}

func (s *LazySymbol) Find() error {
	if atomic.LoadUintptr(&s.sym) == 0 {
		s.mtx.Lock()
		defer s.mtx.Unlock()
		if s.sym == 0 {
			err := s.lib.Load()
			if err != nil {
				return err
			}
			sym, err := Dlsym(s.lib.handle, s.name)
			if err != nil {
				return err
			}
			atomic.StoreUintptr(&s.sym, sym)
		}
	}
	return nil
}

func (s *LazySymbol) Addr() uintptr {
	s.Find()
	return s.sym
}

func (s *LazySymbol) Call(args ...uintptr) (r1, r2 uintptr, err error) {
	if err = s.Find(); err != nil {
		return 0, 0, err
	}
	return Call(s.sym, args...)
}
