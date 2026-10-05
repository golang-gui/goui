package main

import (
	"image/color"
	"testing"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/widgets"
)

// 例程自身的样式及绑定逻辑，不创建原生窗口，不依赖桌面。
func TestCellEditorStyle(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		for _, dark := range []bool{false, true} {
			var source style.StyleSheet = modern.Sheet(modern.Options{Dark: dark})
			if fallback {
				source = gui.DefaultStyleSheet()
			}
			for appearance := range appearances {
				sheet := sheet(dark, fallback, appearance)
				for _, state := range []style.State{style.Normal, style.Focused} {
					editor := sheet.Resolve(style.Sel{Name: "table-cell-input", State: state})
					width, _ := editor.BorderWidth()
					radius, _ := editor.Radius()
					bg, ok := editor.BackgroundColor()
					want := float32(0)
					if state == style.Focused {
						want = 2
					}
					if width != want || radius != 0 || !ok || color.RGBAModel.Convert(bg).(color.RGBA).A != 0 {
						t.Fatalf("editor appearance: fallback=%v dark=%v state=%v width=%g radius=%g background=%v", fallback, dark, state, width, radius, bg)
					}
					base := source.Resolve(style.Sel{Name: "text-input", State: state})
					foreground, _ := editor.ForegroundColor()
					baseForeground, _ := base.ForegroundColor()
					font, _ := editor.FontSize()
					baseFont, _ := base.FontSize()
					if font != baseFont || color.RGBAModel.Convert(foreground) != color.RGBAModel.Convert(baseForeground) {
						t.Fatal("independent cell editor lost the active theme's typography")
					}
					if state == style.Focused {
						border, _ := editor.BorderColor()
						baseBorder, _ := base.BorderColor()
						if color.RGBAModel.Convert(border) != color.RGBAModel.Convert(baseBorder) {
							t.Fatal("cell focus border no longer follows the editor theme")
						}
					}
				}
				for _, part := range []string{"caret", "selection"} {
					editor := sheet.Resolve(style.Sel{Name: "table-cell-input", Part: part})
					base := source.Resolve(style.Sel{Name: "text-input", Part: part})
					fg, _ := editor.ForegroundColor()
					baseFG, _ := base.ForegroundColor()
					bg, _ := editor.BackgroundColor()
					baseBG, _ := base.BackgroundColor()
					if part == "caret" && color.RGBAModel.Convert(fg) != color.RGBAModel.Convert(baseFG) || part == "selection" && color.RGBAModel.Convert(bg) != color.RGBAModel.Convert(baseBG) {
						t.Fatalf("cell editor lost the %s part", part)
					}
				}
				base := source.Resolve(style.Sel{Name: "text-input"})
				ordinary := sheet.Resolve(style.Sel{Name: "text-input"})
				baseRadius, _ := base.Radius()
				radius, _ := ordinary.Radius()
				width, _ := ordinary.BorderWidth()
				if width != 1 || radius != baseRadius {
					t.Fatal("cell-specific rules changed ordinary TextInput")
				}
			}
		}
	}
}

func TestNoteCellBindingKeepsEditsByRowID(t *testing.T) {
	m := newModel()
	version := 0
	d := &cellDelegate{model: m, column: "note", version: &version}
	cell := d.Setup().(*noteCell)
	first := widgets.TableRow{ID: "record-00000", Index: 0}
	second := widgets.TableRow{ID: "record-00001", Index: 1}
	d.Bind(first, cell)
	if len(m.notes) != 0 || cell.input.Text() != "Note 00000" {
		t.Fatal("programmatic binding wrote editor contents back as an edit")
	}
	// SetText exercises the same text signal as editing; pointer/key input and
	// focus isolation are separately checked by the standalone window program.
	cell.input.SetText("edited")
	d.Bind(first, cell)
	if cell.input.Text() != "edited" || m.note(first.ID) != "edited" {
		t.Fatal("repeated binding overwrote the edit")
	}
	d.Unbind(first, cell)
	cell.input.SetText("detached")
	if m.note(first.ID) != "edited" {
		t.Fatal("unbound editor still wrote to its old record")
	}
	d.Bind(second, cell)
	if cell.input.Text() != "Note 00001" || len(m.notes) != 1 {
		t.Fatal("recycled editor carried old text into the new record")
	}
	cell.input.SetText("")
	m.sort("name", widgets.SortDescending)
	// Sorting changes indices, but note ownership follows stable record IDs.
	first.Index = m.ItemsCount() - 1
	d.Unbind(second, cell)
	d.Bind(first, cell)
	if cell.input.Text() != "edited" || m.note(second.ID) != "" {
		t.Fatal("sorting or empty edits lost the record's note")
	}
	d.Unbind(first, cell)
	m.remove(first.ID)
	if _, present := m.notes[first.ID]; present {
		t.Fatal("deleted record retained its editing state")
	}
}
