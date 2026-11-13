package x11

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf8"
	"unsafe"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
)

// xSettings is a display-owned XSETTINGS client, shared by settings and cursors.
// It uses GOUI's X connection and event loop; it never dispatches GTK events.
type xSettings struct {
	display                      xlib.Display
	root, owner                  xlib.Window
	selection, settings, manager xlib.Atom
	rootMask, ownerMask          int64
	values                       map[string]xSetting
	err                          error
	closed                       bool
}

type xSetting struct {
	kind    byte
	integer int32
	text    string
	color   [4]uint16
}

func newXSettings(display xlib.Display) *xSettings {
	s := &xSettings{
		display: display, root: display.DefaultRootWindow(),
		selection: display.InternAtom(fmt.Sprintf("_XSETTINGS_S%d", display.DefaultScreen()), false),
		settings:  display.InternAtom("_XSETTINGS_SETTINGS", false),
		manager:   display.InternAtom("MANAGER", false),
	}
	s.rootMask = s.subscribe(s.root)
	s.refresh()
	return s
}

func (p *Platform) desktopSettings() *xSettings {
	if p.xsettings == nil {
		p.xsettings = newXSettings(p.display)
	}
	return p.xsettings
}

func (s *xSettings) subscribe(window xlib.Window) int64 {
	var attributes xlib.WindowAttributes
	s.display.GetWindowAttributes(window, &attributes)
	s.display.ChangeWindowAttributes(window, xlib.CwEventMask, &xlib.SetWindowAttributes{
		EventMask: uint64(attributes.YourEventMask) | xlib.EventMaskPropertyChange | xlib.EventMaskStructureNotify,
	})
	return attributes.YourEventMask
}
func (s *xSettings) restore(window xlib.Window, mask int64) {
	s.display.ChangeWindowAttributes(window, xlib.CwEventMask, &xlib.SetWindowAttributes{EventMask: uint64(mask)})
}

// refresh performs only bounded X requests while the server is grabbed. Looking
// up the owner and subscribing/reading atomically avoids a destroyed-owner race.
// Parse after releasing the server; publish all values together or none.
func (s *xSettings) refresh() {
	if s.closed {
		s.values, s.err = nil, common.ErrUnavailable
		return
	}
	s.display.GrabServer()
	owner := s.display.GetSelectionOwner(s.selection)
	if owner != s.owner {
		s.owner = owner
		if owner != 0 {
			s.ownerMask = s.subscribe(owner)
		}
	}
	var data []byte
	var err error = common.ErrSettingUnsupported
	if owner != 0 {
		data, err = settingsProperty(s.display, owner, s.settings, s.settings)
	}
	s.display.UngrabServer()
	s.display.Flush()
	s.values = nil
	if err == nil {
		s.values, err = parseXSettings(data)
	}
	s.err = err
}

func (s *xSettings) handleEvent(event *xlib.Event) bool {
	if s.closed {
		return false
	}
	relevant := false
	switch event.Type {
	case xlib.PropertyNotify:
		e := event.PropertyEvent()
		relevant = e.Window == s.owner && e.Atom == s.settings
	case xlib.ClientMessage:
		e := event.ClientMessageEvent()
		relevant = e.Window == s.root && e.MessageType == s.manager && e.Format == 32 && xlib.Atom(e.L[1]) == s.selection
	case xlib.DestroyNotify:
		relevant = event.DestroyWindowEvent().Window == s.owner
		if relevant {
			s.owner = 0
		}
	}
	if relevant {
		s.refresh()
	}
	return relevant
}

func (s *xSettings) string(name string) (string, error) {
	// Settings queries remain immediate native observations, including when
	// the platform event loop has not run yet or a manager was just replaced.
	s.refresh()
	if s.err != nil {
		return "", s.err
	}
	v, ok := s.values[name]
	if !ok || v.text == "" && v.kind == 1 {
		return "", common.ErrSettingUnsupported
	}
	if v.kind != 1 {
		return "", fmt.Errorf("XSETTINGS %s: expected string, got type %d", name, v.kind)
	}
	if !utf8.ValidString(v.text) || strings.IndexByte(v.text, 0) >= 0 {
		return "", fmt.Errorf("XSETTINGS %s: invalid text", name)
	}
	return v.text, nil
}

func (s *xSettings) integer(name string) (int32, error) {
	s.refresh()
	if s.err != nil {
		return 0, s.err
	}
	v, ok := s.values[name]
	if !ok {
		return 0, common.ErrSettingUnsupported
	}
	if v.kind != 0 {
		return 0, fmt.Errorf("XSETTINGS %s: expected integer, got type %d", name, v.kind)
	}
	return v.integer, nil
}

func (s *xSettings) destroy() {
	if s.closed {
		return
	}
	s.closed = true
	s.display.GrabServer()
	if s.owner != 0 && s.display.GetSelectionOwner(s.selection) == s.owner {
		s.restore(s.owner, s.ownerMask)
	}
	s.restore(s.root, s.rootMask)
	s.display.UngrabServer()
	s.display.Flush()
	s.values, s.err = nil, common.ErrUnavailable
}

// settingsProperty bounds native reads to 1 MiB, rejecting wrong types, formats
// and partial properties. The caller protects a foreign window's lifetime.
func settingsProperty(display xlib.Display, window xlib.Window, property, expected xlib.Atom) ([]byte, error) {
	var actual xlib.Atom
	var format int32
	var count, remaining uint
	var data *byte
	status := display.GetWindowProperty(window, property, 0, 1<<18, false, expected,
		&actual, &format, &count, &remaining, &data)
	if data != nil {
		defer xlib.Free(data)
	}
	if status != 0 {
		return nil, fmt.Errorf("XSETTINGS property read failed: status %d", status)
	}
	if actual == 0 {
		return nil, common.ErrSettingUnsupported
	}
	if actual != expected || format != 8 || remaining != 0 || count > 1<<20 || data == nil {
		return nil, fmt.Errorf("invalid XSETTINGS property: type=%d format=%d count=%d remaining=%d", actual, format, count, remaining)
	}
	return append([]byte(nil), unsafe.Slice(data, int(count))...), nil
}

// XSETTINGS v0.5 has its own byte order, four-byte padding and typed entries.
// Validate the whole property before publishing a snapshot, including unknown
// names. No allocation is based on an unchecked wire length or setting count.
func parseXSettings(data []byte) (map[string]xSetting, error) {
	invalid := func() (map[string]xSetting, error) { return nil, fmt.Errorf("malformed XSETTINGS property") }
	if len(data) < 12 || len(data) > 1<<20 || data[0] > 1 {
		return invalid()
	}
	var order binary.ByteOrder = binary.LittleEndian
	if data[0] == 1 {
		order = binary.BigEndian
	}
	take := func(n uint32) []byte {
		if uint64(n) > uint64(len(data)) {
			return nil
		}
		value := data[:n]
		data = data[n:]
		return value
	}
	count := order.Uint32(data[8:12])
	data = data[12:]
	values := make(map[string]xSetting)
	for i := uint32(0); i < count; i++ {
		header := take(4)
		if header == nil {
			return invalid()
		}
		n := uint32(order.Uint16(header[2:]))
		name := take(n)
		if !validXSettingName(name) || take((4-n%4)%4) == nil || take(4) == nil {
			return invalid()
		}
		key := string(name)
		if _, duplicate := values[key]; duplicate {
			return invalid()
		}
		v := xSetting{kind: header[0]}
		switch v.kind {
		case 0:
			value := take(4)
			if value == nil {
				return invalid()
			}
			v.integer = int32(order.Uint32(value))
		case 1:
			length := take(4)
			if length == nil {
				return invalid()
			}
			n = order.Uint32(length)
			value := take(n)
			if value == nil || take((4-n%4)%4) == nil {
				return invalid()
			}
			v.text = string(value)
		case 2:
			value := take(8)
			if value == nil {
				return invalid()
			}
			for j := range v.color {
				v.color[j] = order.Uint16(value[j*2:])
			}
		default:
			return invalid()
		}
		values[key] = v
	}
	if len(data) != 0 {
		return invalid()
	}
	return values, nil
}

func validXSettingName(name []byte) bool {
	start := true
	for _, c := range name {
		switch {
		case c == '/':
			if start {
				return false
			}
			start = true
		case c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '_' || !start && c >= '0' && c <= '9':
			start = false
		default:
			return false
		}
	}
	return !start
}
