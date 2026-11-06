package base

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func nativeTestInputs(t *testing.T, goos, variant string) *NativeInputs {
	t.Helper()
	root := t.TempDir()
	m := &NativeInputs{Format: 1, GOOS: goos, GOARCH: "arm64", Variant: variant, MinimumVersion: "29", SDKVersion: "29", Provenance: NativeProvenance{"test", "1", "fixture", "test only"}, Files: make(map[string]string), root: root}
	names := []string{"lib/libc.so", "lib/libdl.so", "lib/libm.so", "crt/begin.o", "crt/end.o", "lib/builtins.a"}
	if goos == "android" {
		m.Android = &AndroidLinkInputs{"lib", "crt/begin.o", "crt/end.o", "lib/builtins.a"}
	} else {
		m.MinimumVersion = "15.0"
		m.SDKVersion = "26.2"
		m.IOS = &IOSLinkInputs{"sdk"}
		names = []string{"sdk/usr/lib/libSystem.tbd", "sdk/usr/lib/libobjc.tbd", "sdk/System/Library/Frameworks/UIKit.framework/UIKit.tbd"}
	}
	for _, name := range names {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
		hash, err := baseHash(path)
		if err != nil {
			t.Fatal(err)
		}
		m.Files[name] = hash
	}
	return m
}

func nativeWriteSpec(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestNativeComponents(t *testing.T) {
	m := nativeTestInputs(t, "android", "device")
	path := filepath.Join(m.root, "inputs.json")
	nativeWriteSpec(t, path, m)
	loaded, err := OpenNativeInputs(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Android.CRTBegin != "crt/begin.o" {
		t.Fatal("组件角色未保留")
	}
	output := filepath.Join(t.TempDir(), "component")
	for name := range m.Files {
		m.Files[name] = ""
	}
	nativeWriteSpec(t, path, m)
	if err = ExportNativeInputs(path, m.root, output); err != nil {
		t.Fatal(err)
	}
	if _, err = OpenNativeInputs(filepath.Join(output, "inputs.json")); err != nil {
		t.Fatal(err)
	}
	if err = ExportNativeInputs(path, m.root, output); err == nil {
		t.Fatal("不应覆盖既有组件")
	}
	extra := filepath.Join(output, "unlisted.tbd")
	if err = os.WriteFile(extra, []byte("unsealed input"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = OpenNativeInputs(filepath.Join(output, "inputs.json")); err == nil || !strings.Contains(err.Error(), "undeclared") {
		t.Fatalf("未声明文件应被拒绝: %v", err)
	}
	if err = os.Remove(extra); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(output, "crt/begin.o")
	os.WriteFile(file, []byte("corrupt"), 0600)
	if _, err = OpenNativeInputs(filepath.Join(output, "inputs.json")); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("损坏组件应被拒绝: %v", err)
	}
	failed := filepath.Join(t.TempDir(), "failed")
	m.Files["missing.o"] = ""
	nativeWriteSpec(t, path, m)
	if err = ExportNativeInputs(path, m.root, failed); err == nil {
		t.Fatal("缺失输入不能成功制作")
	}
	if _, err = os.Stat(failed); !os.IsNotExist(err) {
		t.Fatal("失败不能发布部分组件")
	}
}

// 只验证原生头部约束，合成数据不用于声称原生链接或执行正确。
func nativeTestELF() []byte {
	data := make([]byte, 64+4*56+2*64+32)
	copy(data, []byte("\x7fELF\x02\x01\x01"))
	u16 := binary.LittleEndian.PutUint16
	u32 := binary.LittleEndian.PutUint32
	u64 := binary.LittleEndian.PutUint64
	u16(data[16:], 3)
	u16(data[18:], 183)
	u32(data[20:], 1)
	u64(data[32:], 64)
	u64(data[40:], 64+4*56)
	u16(data[52:], 64)
	u16(data[54:], 56)
	u16(data[56:], 4)
	u16(data[58:], 64)
	u16(data[60:], 2)
	ph := func(i int, kind, flags uint32, offset, vaddr, filesz, memsz, align uint64) {
		p := data[64+i*56:]
		u32(p, kind)
		u32(p[4:], flags)
		u64(p[8:], offset)
		u64(p[16:], vaddr)
		u64(p[32:], filesz)
		u64(p[40:], memsz)
		u64(p[48:], align)
	}
	ph(0, 1, 4, 0, 0, uint64(len(data)), uint64(len(data)), 16384)
	ph(1, 0x6474e552, 4, 0, 0, 0, 16384, 1)
	ph(2, 0x6474e551, 6, 0, 0, 0, 0, 16)
	dynamic := uint64(len(data) - 32)
	ph(3, 2, 6, dynamic, dynamic, 32, 32, 8)
	section := data[64+4*56+64:]
	u32(section[4:], 6)
	u64(section[24:], dynamic)
	u64(section[32:], 32)
	u64(section[48:], 8)
	u64(section[56:], 16)
	u64(data[dynamic:], 30)
	u64(data[dynamic+8:], 8)
	return data
}

func nativeTestMachO() []byte {
	data := make([]byte, 32+24+72)
	u32 := binary.LittleEndian.PutUint32
	u32(data, 0xfeedfacf)
	u32(data[4:], 0x100000c)
	u32(data[12:], 2)
	u32(data[16:], 2)
	u32(data[20:], 24+72)
	command := data[32:]
	u32(command, 0x32)
	u32(command[4:], 24)
	u32(command[8:], 7)
	u32(command[12:], 15<<16)
	u32(command[16:], 26<<16|2<<8)
	segment := data[56:]
	u32(segment, 0x19)
	u32(segment[4:], 72)
	copy(segment[8:], "__DATA_CONST")
	u32(segment[68:], 0x10)
	return data
}

func TestNativeOutputChecks(t *testing.T) {
	android := nativeTestInputs(t, "android", "device")
	ios := nativeTestInputs(t, "ios", "simulator")
	for _, test := range []struct {
		name string
		data []byte
		m    *NativeInputs
		edit func([]byte)
		want string
	}{
		{"Android", nativeTestELF(), android, func([]byte) {}, ""},
		{"Android错误架构", nativeTestELF(), android, func(b []byte) { binary.LittleEndian.PutUint16(b[18:], 62) }, "ARM64"},
		{"Android低页对齐", nativeTestELF(), android, func(b []byte) { binary.LittleEndian.PutUint64(b[64+48:], 4096) }, "16 KiB"},
		{"AndroidRELRO尾端", nativeTestELF(), android, func(b []byte) { binary.LittleEndian.PutUint64(b[64+56+40:], 16383) }, "RELRO"},
		{"Android可执行栈", nativeTestELF(), android, func(b []byte) { binary.LittleEndian.PutUint32(b[64+112+4:], 7) }, "stack"},
		{"Android延迟绑定", nativeTestELF(), android, func(b []byte) { binary.LittleEndian.PutUint64(b[len(b)-24:], 0) }, "immediate"},
		{"iOS", nativeTestMachO(), ios, func([]byte) {}, ""},
		{"iOS错误平台", nativeTestMachO(), ios, func(b []byte) { binary.LittleEndian.PutUint32(b[40:], 2) }, "platform"},
		{"iOS部署版本", nativeTestMachO(), ios, func(b []byte) { binary.LittleEndian.PutUint32(b[44:], 16<<16) }, "deployment"},
		{"iOS常量段只读标记", nativeTestMachO(), ios, func(b []byte) { binary.LittleEndian.PutUint32(b[124:], 0) }, "SG_READ_ONLY"},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.edit(test.data)
			path := filepath.Join(t.TempDir(), "binary")
			os.WriteFile(path, test.data, 0600)
			err := CheckNativeBinary(path, test.m)
			if test.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v want %s", err, test.want)
			}
		})
	}
}

func TestNativeToolVersionIdentity(t *testing.T) {
	for _, test := range []struct {
		output string
		want   bool
	}{
		{"LLD 22.1.8 (compatible with GNU linkers)", true},
		{"Debian LLD 22.1.8", true},
		{"LLD 22.1.80", false},
		{"unrelated 22.1.8", false},
	} {
		if got := nativeLLDVersion(test.output, "22.1.8"); got != test.want {
			t.Fatalf("%q: got %v want %v", test.output, got, test.want)
		}
	}
}

// 极小的 ARM64 ET_REL，只包含 RET 和可选导出符号；无需原生编译器。
func nativeTestObject(name string) []byte {
	data := make([]byte, 128+4*64)
	copy(data, []byte("\x7fELF\x02\x01\x01"))
	u16, u32, u64 := binary.LittleEndian.PutUint16, binary.LittleEndian.PutUint32, binary.LittleEndian.PutUint64
	u16(data[16:], 1)
	u16(data[18:], 183)
	u32(data[20:], 1)
	u64(data[40:], 128)
	u16(data[52:], 64)
	u16(data[58:], 64)
	u16(data[60:], 4)
	u32(data[64:], 0xd65f03c0)
	if name != "" {
		u32(data[96:], 1)
		data[100] = 0x12
		u16(data[102:], 1)
		u64(data[112:], 4)
	}
	if len(name) > 6 {
		panic("fixture symbol too long")
	}
	copy(data[121:], name)
	section := func(i int, kind uint32, flags, offset, size, align, entsize uint64, link, info uint32) {
		s := data[128+i*64:]
		u32(s[4:], kind)
		u64(s[8:], flags)
		u64(s[24:], offset)
		u64(s[32:], size)
		u32(s[40:], link)
		u32(s[44:], info)
		u64(s[48:], align)
		u64(s[56:], entsize)
	}
	section(1, 1, 6, 64, 4, 4, 0, 0, 0)
	section(2, 2, 0, 72, 48, 8, 24, 3, 1)
	section(3, 3, 0, 120, 8, 1, 0, 0, 0)
	return data
}
