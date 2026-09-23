package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
)

const probeMIME = "application/x-goui-drag-probe"

var probeBytes = []byte{0, 1, 2, 0xff, '\n', 0, 42}
var probeURLs = []string{"https://example.com/goui?q=drag%20drop", "https://example.org/%E4%B8%AD%E6%96%87"}

// This state belongs to the window probe, not the framework. It checks observable
// signal counts and payloads without bypassing input dispatch.
type probe struct {
	status, counts                                  *gui.Label
	prepares, begins, ends, drops, clicks, failures int
	active, begun, reject                           bool
	clickStart, dropStart                           int
	dropAction                                      gui.DragAction
	text                                            string
	files                                           []string
	local                                           *struct{ value string }
	lastDrop                                        string
}

func newProbe(large bool) *probe {
	p := &probe{status: gui.NewLabel(""), counts: gui.NewLabel(""),
		text: "GOUI drag text / 中文 / 100%", local: &struct{ value string }{"local-value"}}
	if large {
		p.text = strings.Repeat("GOUI 中文\n", (256<<10)/len("GOUI 中文\n"))
		p.text += strings.Repeat(".", (256<<10)-len(p.text))
	}
	p.status.SetWrapMode(gui.WrapWordChar)
	p.status.SetMinSize(geometry.Size{Height: 80})
	p.status.SetMaxSize(geometry.Size{Width: 740, Height: 80})
	p.report("Ready. File paths and full received values are printed in the terminal.")
	return p
}

func (p *probe) report(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	fmt.Println(message)
	p.status.SetText(message + "\n" + p.lastDrop)
	p.counts.SetText(fmt.Sprintf("Prepare=%d  Begin=%d  Drop=%d  End=%d  Click=%d  FAIL=%d",
		p.prepares, p.begins, p.drops, p.ends, p.clicks, p.failures))
}

func (p *probe) check(ok bool, message string) {
	if !ok {
		p.failures++
		p.report("FAIL: %s", message)
	}
}

func (p *probe) targets(host string) *gui.LinearBox {
	row := gui.NewLinearBox(layout.DirectionHorizontal)
	row.SetSpacing(8)
	for _, entry := range []struct {
		name   string
		format gui.DragFormat
	}{
		{"Local", gui.LocalFormat("probe")}, {"Text", gui.DragFormatText},
		{"Files", gui.DragFormatFiles}, {"URLs", gui.DragFormatURLs}, {"MIME", gui.MIMEFormat(probeMIME)},
	} {
		row.AddChild(p.target(host+"/"+entry.name, entry.name, entry.format))
	}
	return row
}

func (p *probe) target(name, caption string, format gui.DragFormat) *gui.Button {
	button := gui.NewButton()
	button.SetID(name)
	label := gui.NewLabel("Drop " + caption)
	button.SetChild(label)
	button.SetMinSize(geometry.Size{Width: 110, Height: 100})
	button.SetMainWeight(1)
	drop := gui.NewDropTarget(format)
	drop.SetActions(gui.DragCopy | gui.DragMove | gui.DragLink)
	entered := false
	drop.ConnectEnter(func(e *gui.DragMotion) {
		p.check(!entered, name+": duplicate Enter without Leave")
		entered = true
		if p.reject {
			e.Action = 0
		}
		label.SetText("Hover " + caption)
		fmt.Printf("%s Enter format=%s action=%d at=%v\n", name, e.Format, e.Action, e.Position)
	})
	drop.ConnectMotion(func(e *gui.DragMotion) {
		if p.reject {
			e.Action = 0
		}
	})
	drop.ConnectLeave(func() {
		p.check(entered, name+": Leave without Enter")
		entered = false
		label.SetText("Drop " + caption)
		fmt.Printf("%s Leave\n", name)
	})
	drop.ConnectDrop(func(e *gui.DropEvent) {
		p.check(entered, name+": Drop without Enter")
		verified, accepted, detail := false, false, ""
		switch e.Format {
		case gui.LocalFormat("probe"):
			value, ok := e.Data.Local("probe")
			verified = ok && value == p.local
			accepted, detail = verified, "Local object identity"
		case gui.DragFormatText:
			value, ok := e.Data.Text()
			verified, accepted = ok && value == p.text, ok
			detail = fmt.Sprintf("Text %d bytes", len(value))
			if len(value) < 256 {
				fmt.Printf("text=%q\n", value)
			}
		case gui.DragFormatURLs:
			value, ok := e.Data.URLs()
			verified, accepted = ok && slices.Equal(value, probeURLs), ok && len(value) != 0
			detail = fmt.Sprintf("URLs %d", len(value))
			fmt.Printf("urls=%q\n", value)
		case gui.MIMEFormat(probeMIME):
			value, ok := e.Data.Bytes(probeMIME)
			verified = ok && bytes.Equal(value, probeBytes)
			accepted, detail = verified, fmt.Sprintf("MIME %x", value)
		case gui.DragFormatFiles:
			value, ok := e.Data.Files()
			verified = ok && slices.Equal(value, p.files)
			accepted = ok && len(value) != 0
			for _, path := range value {
				fmt.Printf("file=%q\n", path)
				accepted = accepted && filepath.IsAbs(path)
			}
			detail = fmt.Sprintf("Files %d (paths logged)", len(value))
		}
		if p.active {
			p.check(verified, "local source payload did not match independent expected value")
			accepted = accepted && verified
		}
		e.Accepted = accepted
		if !accepted {
			p.check(false, name+": unexpected payload")
			return
		}
		p.drops++
		if p.active {
			p.dropAction = e.Action
		}
		verdict := "RECEIVED"
		if verified {
			verdict = "VERIFIED"
		}
		p.lastDrop = fmt.Sprintf("%s %s: %s, action=%d", verdict, name, detail, e.Action)
		p.report("%s", p.lastDrop)
	})
	drop.ConnectError(func(err error) { p.check(false, name+": "+err.Error()) })
	button.AddEventController(drop)
	return button
}
