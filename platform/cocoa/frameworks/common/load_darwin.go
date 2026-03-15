package common

import (
	"fmt"
	"github.com/ebitengine/purego"
)

type LoadFunc func(framework string) (handle uintptr, err error)

func LoadSystemFramework(framework string) (handle uintptr, err error) {
	return purego.Dlopen(fmt.Sprintf("/System/Library/Frameworks/%s.framework/%s", framework, framework), purego.RTLD_NOW|purego.RTLD_GLOBAL)
}
