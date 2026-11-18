package utils

import (
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/ebitengine/purego"
)

var constTestFramework = sync.OnceValues(func() (Framework, error) {
	return LoadSystemFramework("CoreFoundation")
})

func TestConstOutputs(t *testing.T) {
	for _, tc := range []struct {
		name           string
		value, address bool
	}{
		{"value", true, false},
		{"address", false, true},
		{"both", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			framework, err := constTestFramework()
			if err != nil {
				t.Fatal(err)
			}
			const name = "kCFBooleanTrue"
			wantAddress, err := purego.Dlsym(uintptr(framework), name)
			if err != nil {
				t.Fatal(err)
			}
			const unchanged = uintptr(0xfeed)
			value, address := unchanged, unchanged
			constant := Const[uintptr]{Name: name}
			if tc.value {
				constant.PVar = &value
			}
			if tc.address {
				constant.PAddr = &address
			}
			if err := framework.LoadConstants([]Constant{constant}); err != nil {
				t.Fatal(err)
			}
			if tc.value {
				if value == 0 || value == unchanged || value == wantAddress {
					t.Fatalf("loaded reference=%#x, symbol address=%#x", value, wantAddress)
				}
				// 用原生 CFBoolean API 验证实际引用，避免只比较同一个加载器的两次内存读取结果。
				var getValue func(uintptr) bool
				if err := framework.LoadFunctions([]Function{{Name: "CFBooleanGetValue", PFunc: &getValue}}); err != nil {
					t.Fatal(err)
				}
				if !getValue(value) {
					t.Fatal("loaded kCFBooleanTrue is not true")
				}
			} else if value != unchanged {
				t.Fatal("value changed without a PVar output")
			}
			if tc.address {
				if address == 0 || address != wantAddress {
					t.Fatalf("symbol address=%#x, want %#x", address, wantAddress)
				}
			} else if address != unchanged {
				t.Fatal("address changed without a PAddr output")
			}
		})
	}
}

func TestConstObjectAddress(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	framework, err := constTestFramework()
	if err != nil {
		t.Fatal(err)
	}
	// 此符号导出首字段 version 为零的结构体；只取地址时不能误读为指针变量。
	const name = "kCFTypeArrayCallBacks"
	var address uintptr
	if err := framework.LoadConstants([]Constant{Const[uintptr]{Name: name, PAddr: &address}}); err != nil {
		t.Fatal(err)
	}
	want, err := purego.Dlsym(uintptr(framework), name)
	if err != nil {
		t.Fatal(err)
	}
	if address == 0 || address != want {
		t.Fatalf("callbacks address=%#x, want exported struct address %#x", address, want)
	}
}

func TestConstErrors(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	framework, err := constTestFramework()
	if err != nil {
		t.Fatal(err)
	}
	const missing = "__goui_missing_constant__"
	const unchanged = uintptr(0xfeed)
	value, address := unchanged, unchanged
	for _, constant := range []Const[uintptr]{
		{Name: missing, PVar: &value},
		{Name: missing, PAddr: &address},
		{Name: missing, PVar: &value, PAddr: &address},
	} {
		if err := framework.LoadConstants([]Constant{constant}); err == nil {
			t.Fatal("missing symbol accepted")
		}
		if value != unchanged || address != unchanged {
			t.Fatal("failed lookup modified outputs")
		}
	}
	const name = "kCFBooleanTrue"
	if err := framework.LoadConstants([]Constant{Const[uintptr]{Name: name}}); err == nil || !strings.Contains(err.Error(), name) {
		t.Fatalf("constant without outputs: error=%v, want configuration error naming %s", err, name)
	}
}
