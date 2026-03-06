package cgo

import (
	"errors"
	"fmt"
	"github.com/ebitengine/purego"
)

func RegisterFunc(pf any, cfn uintptr) (err error) {
	defer func() {
		if msg := recover(); msg != nil {
			err = errors.New(fmt.Sprint(msg))
		}
	}()
	purego.RegisterFunc(pf, cfn)
	return
}

func RegisterLibFunc(pf any, lib uintptr, symbol string) (err error) {
	defer func() {
		if msg := recover(); msg != nil {
			err = errors.New(fmt.Sprint(msg))
		}
	}()
	purego.RegisterLibFunc(pf, lib, symbol)
	return
}
