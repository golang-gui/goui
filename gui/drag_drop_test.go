package gui

import (
	"bytes"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDragDataFrozenRepresentations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.txt")
	files := []string{path}
	bits := []byte{1, 2, 3}
	local := &struct{ Number int }{Number: 4}
	data := new(DragData)
	data.SetFiles(files)
	data.SetBytes("application/x-goui-test", bits)
	data.SetLocal("record", local)
	files[0] = "changed"
	bits[0] = 9
	if err := data.validate(); err != nil {
		t.Fatal(err)
	}
	frozen := data.freeze()
	data.SetFiles([]string{"other"})
	data.SetBytes("application/x-goui-test", []byte{7})
	data.SetLocal("record", nil)
	gotFiles, ok := frozen.Files()
	if !ok || !reflect.DeepEqual(gotFiles, []string{path}) {
		t.Fatalf("frozen files %v %t", gotFiles, ok)
	}
	gotBytes, ok := frozen.Bytes("application/x-goui-test")
	if !ok || !bytes.Equal(gotBytes, []byte{1, 2, 3}) {
		t.Fatalf("frozen bytes %v %t", gotBytes, ok)
	}
	value, ok := frozen.Local("record")
	if !ok || value != local {
		t.Fatalf("frozen local identity %v %t", value, ok)
	}
	if got := frozen.selected(LocalFormat("record")); !got.contains(LocalFormat("record")) || got.contains(DragFormatFiles) {
		t.Fatalf("selected local leaked representations: %v", got.formats())
	}
}

func TestDragDataRejectsInvalidLocalAndEmpty(t *testing.T) {
	if LocalFormat("") != "" || LocalFormat("x\x00y") != "" {
		t.Fatal("invalid local names accepted")
	}
	if err := (new(DragData)).validate(); err == nil {
		t.Fatal("empty data accepted")
	}
	data := new(DragData)
	data.SetLocal("bad\x00name", 1)
	if err := data.validate(); err == nil {
		t.Fatal("invalid local name accepted")
	}
	data = new(DragData)
	data.SetLocal("nil-value", nil)
	if err := data.validate(); err != nil {
		t.Fatal(err)
	}
	if value, ok := data.Local("nil-value"); !ok || value != nil {
		t.Fatalf("nil value and absent confused: %v %t", value, ok)
	}
}
