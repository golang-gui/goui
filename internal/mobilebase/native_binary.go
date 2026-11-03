package mobilebase

import (
	"debug/elf"
	"debug/macho"
	"fmt"
	"strconv"
	"strings"
)

func nativeVersion(text string) (uint32, error) {
	parts := strings.Split(text, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return 0, fmt.Errorf("invalid native version %q", text)
	}
	values := [3]uint64{}
	for i, p := range parts {
		bits := 8
		if i == 0 {
			bits = 16
		}
		value, err := strconv.ParseUint(p, 10, bits)
		if err != nil {
			return 0, fmt.Errorf("invalid native version %q", text)
		}
		values[i] = value
	}
	return uint32(values[0]<<16 | values[1]<<8 | values[2]), nil
}

// CheckNativeBinary verifies format/target and the output's native invariants.
// It does not substitute for device execution, app signing or package validation.
func CheckNativeBinary(path string, inputs *NativeInputs) error {
	if inputs.GOOS == "android" {
		return checkAndroidBinary(path)
	}
	file, err := macho.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if file.Magic != macho.Magic64 || file.Cpu != macho.CpuArm64 || file.Type != macho.TypeExec {
		return fmt.Errorf("expected ARM64 iOS executable")
	}
	minimum, err := nativeVersion(inputs.MinimumVersion)
	if err != nil {
		return err
	}
	sdk, err := nativeVersion(inputs.SDKVersion)
	if err != nil {
		return err
	}
	platform := uint32(2)
	if inputs.Variant == "simulator" {
		platform = 7
	}
	found := false
	for _, load := range file.Loads {
		data := load.Raw()
		if len(data) >= 24 && file.ByteOrder.Uint32(data[:4]) == 0x32 {
			if found {
				return fmt.Errorf("multiple Mach-O build-version commands")
			}
			found = true
			if file.ByteOrder.Uint32(data[8:12]) != platform || file.ByteOrder.Uint32(data[12:16]) != minimum || file.ByteOrder.Uint32(data[16:20]) != sdk {
				return fmt.Errorf("Mach-O platform/deployment/SDK does not match component")
			}
		}
	}
	if !found {
		return fmt.Errorf("missing Mach-O build-version command")
	}
	for _, load := range file.Loads {
		if segment, ok := load.(*macho.Segment); ok && segment.Name == "__DATA_CONST" && segment.Flag&0x10 == 0 {
			return fmt.Errorf("Mach-O __DATA_CONST requires SG_READ_ONLY; use a supported LLD")
		}
	}
	return nil
}

func checkAndroidBinary(path string) error {
	file, err := elf.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if file.Class != elf.ELFCLASS64 || file.Data != elf.ELFDATA2LSB || file.Machine != elf.EM_AARCH64 || file.Type != elf.ET_DYN {
		return fmt.Errorf("expected ARM64 Android shared library")
	}
	load, relro, stack := false, false, false
	for _, program := range file.Progs {
		switch program.Type {
		case elf.PT_LOAD:
			load = true
			if program.Align < 16384 || program.Off%16384 != program.Vaddr%16384 {
				return fmt.Errorf("Android LOAD is not 16 KiB compatible")
			}
		case elf.PT_GNU_RELRO:
			relro = true
			if (program.Vaddr+program.Memsz)%16384 != 0 {
				return fmt.Errorf("Android RELRO is not 16 KiB aligned")
			}
		case elf.PT_GNU_STACK:
			stack = true
			if program.Flags&elf.PF_X != 0 {
				return fmt.Errorf("Android stack is executable")
			}
		}
	}
	if !load || !relro || !stack {
		return fmt.Errorf("missing Android LOAD/RELRO/STACK segment")
	}
	if values, err := file.DynValue(elf.DT_TEXTREL); err != nil {
		return err
	} else if len(values) != 0 {
		return fmt.Errorf("Android library has text relocations")
	}
	now := false
	for _, entry := range []struct {
		tag  elf.DynTag
		mask uint64
	}{{elf.DT_FLAGS, uint64(elf.DF_BIND_NOW)}, {elf.DT_FLAGS_1, uint64(elf.DF_1_NOW)}} {
		values, err := file.DynValue(entry.tag)
		if err != nil {
			return err
		}
		for _, value := range values {
			now = now || value&entry.mask != 0
		}
	}
	if !now {
		return fmt.Errorf("Android library requires immediate symbol binding")
	}
	return nil
}
