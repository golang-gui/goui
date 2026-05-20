package utils

import (
	"fmt"
	"github.com/ebitengine/purego"
	"github.com/goexlib/cgo"
)

type Framework uintptr

type Function struct {
	Name  string
	PFunc any
}

type Const[T any] struct {
	Name string
	PVar *T
}

func (c Const[T]) getExtern(framework Framework) error {
	p, err := cgo.GetExternVariant[T](framework, c.Name)
	if err != nil {
		return err
	}
	*c.PVar = *p
	return nil
}

type Constant interface {
	getExtern(framework Framework) error
}

func (f Framework) GetSymbol(name string) (uintptr, error) {
	return cgo.Dlsym(uintptr(f), name)
}

func (f Framework) LoadFunctions(fns []Function) (err error) {
	for _, fn := range fns {
		err = cgo.RegisterLibFunc(fn.PFunc, uintptr(f), fn.Name)
		if err != nil {
			return
		}
	}
	return nil
}

func (f Framework) LoadConstants(constants []Constant) (err error) {
	for _, constant := range constants {
		err = constant.getExtern(f)
		if err != nil {
			return
		}
	}
	return nil
}

func LoadSystemFramework(framework string) (_ Framework, err error) {
	handle, err := purego.Dlopen(fmt.Sprintf("/System/Library/Frameworks/%s.framework/%s", framework, framework), purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return 0, err
	}
	return Framework(handle), nil
}
