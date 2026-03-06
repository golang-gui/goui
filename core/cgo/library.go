package cgo

import "syscall"

type Library uintptr

func LoadLibrary(path string) (Library, error) {
	handle, err := Dlopen(path, RTLD_NOW)
	if err != nil {
		return 0, err
	}
	return Library(handle), nil
}

func (l Library) Close() {
	Dlclose(uintptr(l))
}

func (l Library) GetSymbol(name string) (Symbol, error) {
	symbol, err := Dlsym(uintptr(l), name)
	if err != nil {
		return 0, err
	}
	return Symbol(symbol), nil
}

type Symbol uintptr

func (s Symbol) Call(args ...uintptr) (r1, r2 uintptr, err syscall.Errno) {
	return Call(uintptr(s), args...)
}
