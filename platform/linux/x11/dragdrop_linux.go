package x11

import (
	"errors"
	"fmt"
	"image"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/dragdrop"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/platform/linux/libs/xcursor"
	"github.com/golang-gui/goui/platform/linux/libs/xlib"
	"github.com/golang-gui/goui/platform/linux/libs/xshape"
	xdraw "golang.org/x/image/draw"

	"github.com/goexlib/cgo"
)

// dragService is one native XDND capability per X11 surface.
type dragService struct {
	window      *Window
	platform    *Platform
	formats     []dragdrop.Format
	formatAtoms map[dragdrop.Format]xlib.Atom
	offer       *xdndOffer
}

type xdndOffer struct {
	service   *dragService
	id        uint64
	source    xlib.Window
	sourceID  uint64
	version   uint8
	types     map[dragdrop.Format]xlib.Atom
	actions   dragdrop.Action
	position  geometry.Point
	time      xlib.Time
	suggested dragdrop.Action
	entered   bool
	dropped   bool
	reading   bool
	finished  bool
	format    dragdrop.Format
	incr      bool
	buffer    []byte
	readTimer *time.Timer
}

func (p *Platform) initDragAtoms() {
	a := &p.atoms
	a.XdndAware = p.display.InternAtom("XdndAware", false)
	a.XdndEnter = p.display.InternAtom("XdndEnter", false)
	a.XdndPosition = p.display.InternAtom("XdndPosition", false)
	a.XdndStatus = p.display.InternAtom("XdndStatus", false)
	a.XdndLeave = p.display.InternAtom("XdndLeave", false)
	a.XdndDrop = p.display.InternAtom("XdndDrop", false)
	a.XdndFinished = p.display.InternAtom("XdndFinished", false)
	a.XdndTypeList = p.display.InternAtom("XdndTypeList", false)
	a.XdndProxy = p.display.InternAtom("XdndProxy", false)
	a.XdndActionList = p.display.InternAtom("XdndActionList", false)
	a.XdndSelection = p.display.InternAtom("XdndSelection", false)
	a.XdndActionCopy = p.display.InternAtom("XdndActionCopy", false)
	a.XdndActionMove = p.display.InternAtom("XdndActionMove", false)
	a.XdndActionLink = p.display.InternAtom("XdndActionLink", false)
	a.GOUI_DND = p.display.InternAtom("GOUI_DND", false)
	a.URI_LIST = p.display.InternAtom("text/uri-list", false)
	a.TEXT_UTF8 = p.display.InternAtom("text/plain;charset=utf-8", false)
	a.INCR = p.display.InternAtom("INCR", false)
	a.GOUI_LOCAL = p.display.InternAtom("application/x-goui-local", false)
}

func (d *dragService) SetFormats(formats []dragdrop.Format) error {
	if d.window == nil || d.window.wid == 0 {
		return fmt.Errorf("x11 dragdrop: destroyed surface")
	}
	unique := make([]dragdrop.Format, 0, len(formats))
	atoms := make(map[dragdrop.Format]xlib.Atom)
	for _, format := range formats {
		if slices.Contains(unique, format) {
			continue
		}
		switch format {
		case dragdrop.FormatLocalMarker:
			atoms[format] = d.platform.atoms.GOUI_LOCAL
		case dragdrop.FormatFiles, dragdrop.FormatURLs:
			atoms[format] = d.platform.atoms.URI_LIST
		case dragdrop.FormatText:
			atoms[format] = d.platform.atoms.UTF8_STRING
		default:
			const prefix = "mime:"
			name := strings.TrimPrefix(string(format), prefix)
			if !strings.HasPrefix(string(format), prefix) || dragdrop.MIMEFormat(name) != format {
				return fmt.Errorf("x11 dragdrop: invalid portable format %q", format)
			}
			atoms[format] = d.platform.display.InternAtom(name, false)
		}
		unique = append(unique, format)
	}
	d.formats, d.formatAtoms = unique, atoms
	if len(unique) == 0 {
		d.platform.display.DeleteProperty(d.window.wid, d.platform.atoms.XdndAware)
	} else {
		version := []xlib.Atom{5}
		d.platform.display.ChangeProperty(d.window.wid, d.platform.atoms.XdndAware,
			xlib.AtomAtom, 32, xlib.PropModeReplace, cgo.CSlice(version), 1)
	}
	d.platform.display.Flush()
	return nil
}

func (d *dragService) Begin(id uint64, data *dragdrop.Data, actions dragdrop.Action, preview dragdrop.Preview, feedback dragdrop.Feedback) error {
	return d.beginSource(id, data, actions, preview, feedback)
}

func (d *dragService) Cancel() {
	if source := d.platform.dragSource; source != nil && source.service == d {
		source.cancel()
	}
}

func (d *dragService) Destroy() {
	w := d.window
	if w == nil {
		return
	}
	d.Cancel()
	offer := d.offer
	if offer != nil && offer.dropped {
		offer.finish(0)
	} else if offer != nil {
		offer.finished = true
	}
	d.offer = nil
	d.window = nil
	w.dnd = nil
	if offer != nil && !offer.dropped && offer.entered && w.wid != 0 {
		w.emitEvent(events.DragOfferEvent{EventType: events.DragLeave, Offer: offer})
	}
	if w.wid != 0 {
		d.platform.display.DeleteProperty(w.wid, d.platform.atoms.XdndAware)
		d.platform.display.Flush()
	}
}

func (o *xdndOffer) ID() uint64 { return o.id }

func (o *xdndOffer) SourceID() uint64 { return o.sourceID }

func (o *xdndOffer) Formats() []dragdrop.Format {
	if o.finished {
		return nil
	}
	formats := make([]dragdrop.Format, 0, len(o.types))
	for _, format := range o.service.formats {
		if o.types[format] != 0 {
			formats = append(formats, format)
		}
	}
	return formats
}

func (o *xdndOffer) Read(format dragdrop.Format) error {
	d := o.service
	if d.window == nil || d.offer != o || !o.dropped || o.reading || o.finished {
		return fmt.Errorf("x11 dragdrop: offer is not readable")
	}
	typ := o.types[format]
	if typ == 0 || !slices.Contains(d.formats, format) {
		return fmt.Errorf("x11 dragdrop: format %q unavailable", format)
	}
	o.format, o.reading = format, true
	o.startReadTimeout()
	d.platform.display.DeleteProperty(d.window.wid, d.platform.atoms.GOUI_DND)
	d.platform.display.ConvertSelection(d.platform.atoms.XdndSelection, typ,
		d.platform.atoms.GOUI_DND, d.window.wid, o.time)
	d.platform.display.Flush()
	return nil
}

func (o *xdndOffer) Finish(action dragdrop.Action) error {
	if !action.ValidResult() || action != 0 && action&o.actions == 0 {
		return fmt.Errorf("x11 dragdrop: invalid finish action %d", action)
	}
	if o.finished || o.service.offer != o || !o.dropped {
		return fmt.Errorf("x11 dragdrop: offer is not finishable")
	}
	o.finish(action)
	return nil
}

func (o *xdndOffer) finish(action dragdrop.Action) {
	if o.finished {
		return
	}
	o.finished = true
	if o.readTimer != nil {
		o.readTimer.Stop()
		o.readTimer = nil
	}
	o.buffer = nil
	d := o.service
	if d.offer == o {
		d.offer = nil
	}
	if d.window == nil || d.window.wid == 0 {
		return
	}
	a := d.platform.atoms
	var flags, nativeAction int64
	if action != 0 {
		flags = 1
		nativeAction = int64(d.platform.atomForDragAction(action))
	}
	if o.version < 5 {
		flags, nativeAction = 0, 0
	}
	d.platform.sendDragMessage(o.source, a.XdndFinished, [5]int64{
		int64(d.window.wid), flags, nativeAction,
	})
}

func (p *Platform) atomForDragAction(action dragdrop.Action) xlib.Atom {
	switch action {
	case dragdrop.Copy:
		return p.atoms.XdndActionCopy
	case dragdrop.Move:
		return p.atoms.XdndActionMove
	case dragdrop.Link:
		return p.atoms.XdndActionLink
	}
	return 0
}

func (p *Platform) dragActionForAtom(atom xlib.Atom) dragdrop.Action {
	switch atom {
	case p.atoms.XdndActionCopy:
		return dragdrop.Copy
	case p.atoms.XdndActionMove:
		return dragdrop.Move
	case p.atoms.XdndActionLink:
		return dragdrop.Link
	}
	return 0
}

func (p *Platform) sendDragMessage(dest xlib.Window, typ xlib.Atom, values [5]int64) {
	p.sendDragMessageWindow(dest, dest, typ, values)
}

func (p *Platform) sendDragMessageWindow(dest, messageWindow xlib.Window, typ xlib.Atom, values [5]int64) {
	var event xlib.Event
	m := event.ClientMessageEvent()
	m.Type, m.Window, m.MessageType, m.Format, m.L = xlib.ClientMessage, messageWindow, typ, 32, values
	// The peer may disappear between any existence query and this request.
	// Track the actual request instead; late BadWindow replies must not kill
	// the process. Xlib serial queries below do not wait for the server.
	p.retireDragMessages(p.display.LastKnownRequestProcessed())
	serial := p.display.NextRequest()
	if p.dragMessages == nil {
		p.dragMessages = make(map[uintptr]xlib.Window)
	}
	p.dragMessages[serial] = dest
	if p.display.SendEvent(dest, false, 0, &event) == 0 {
		delete(p.dragMessages, serial)
	}
	p.display.Flush()
}

func (p *Platform) onDragMessageError(display xlib.Display, event *xlib.ErrorEvent) int32 {
	if display == p.display && p.consumeDragMessageError(event) {
		return 0
	}
	// XSetErrorHandler returns a callable default handler as well. Do not
	// suppress unrelated requests or errors from another library/display.
	return p.previousXErrorHandler.Call(display, event)
}

func (p *Platform) consumeDragMessageError(event *xlib.ErrorEvent) bool {
	if event.Display != p.display || event.ErrorCode != xlib.BadWindow ||
		event.RequestCode != xlib.RequestSendEvent || event.MinorCode != 0 {
		return false
	}
	dest, pending := p.dragMessages[event.Serial]
	if !pending || xlib.ID(dest) != event.ResourceID {
		return false
	}
	delete(p.dragMessages, event.Serial)
	return true
}

func (p *Platform) retireDragMessages(processed uintptr) {
	for serial := range p.dragMessages {
		if serial <= processed {
			delete(p.dragMessages, serial)
		}
	}
}

func (d *dragService) handleClientMessage(m *xlib.ClientMessageEvent) bool {
	p := d.platform
	if m.Format != 32 {
		return false
	}
	switch m.MessageType {
	case p.atoms.XdndEnter:
		d.enter(m)
	case p.atoms.XdndPosition:
		d.position(m)
	case p.atoms.XdndLeave:
		d.leave(m)
	case p.atoms.XdndDrop:
		d.drop(m)
	default:
		return false
	}
	return true
}

func (d *dragService) enter(m *xlib.ClientMessageEvent) {
	if d.offer != nil {
		if d.offer.dropped {
			d.offer.finish(0)
		} else {
			d.leaveCurrent()
		}
	}
	if d.window == nil || len(d.formats) == 0 {
		return
	}
	version := uint8(uint64(m.L[1]) >> 24)
	if version < 3 {
		return
	}
	version = min(version, 5)
	source := xlib.Window(m.L[0])
	var advertised []xlib.Atom
	if m.L[1]&1 != 0 {
		advertised = d.platform.readDragAtoms(source, d.platform.atoms.XdndTypeList)
	} else {
		for _, raw := range m.L[2:5] {
			if raw != 0 {
				advertised = append(advertised, xlib.Atom(raw))
			}
		}
	}
	types := make(map[dragdrop.Format]xlib.Atom)
	for _, format := range d.formats {
		preferred := d.formatAtoms[format]
		if slices.Contains(advertised, preferred) {
			types[format] = preferred
		} else if format == dragdrop.FormatText && slices.Contains(advertised, d.platform.atoms.TEXT_UTF8) {
			types[format] = d.platform.atoms.TEXT_UTF8
		}
	}
	d.platform.nextDragOfferID++
	d.offer = &xdndOffer{service: d, id: d.platform.nextDragOfferID, source: source,
		version: version, types: types, actions: dragdrop.Copy}
	if local := d.platform.dragSource; local != nil && local.window.wid == source &&
		d.platform.display.GetSelectionOwner(d.platform.atoms.XdndSelection) == source {
		d.offer.sourceID = local.id
	}
	for _, atom := range d.platform.readDragAtoms(source, d.platform.atoms.XdndActionList) {
		d.offer.actions |= d.platform.dragActionForAtom(atom)
	}
}

func (d *dragService) position(m *xlib.ClientMessageEvent) {
	o := d.offer
	if o == nil || o.source != xlib.Window(m.L[0]) || d.window == nil || o.dropped || o.finished {
		return
	}
	x, y := int(int16(uint32(m.L[2])>>16)), int(int16(uint32(m.L[2])))
	x, y, ok := d.platform.display.TranslateCoordinatesChecked(d.platform.defScreen.Root, d.window.wid, x, y)
	if !ok {
		return
	}
	scale := currentScale()
	o.position = geometry.Point{X: float32(x) / scale, Y: float32(y) / scale}
	o.time = xlib.Time(m.L[3])
	suggested := d.platform.dragActionForAtom(xlib.Atom(m.L[4]))
	o.actions |= suggested
	o.suggested = suggested
	answer := dragdrop.Action(0)
	typ := events.DragMotion
	if !o.entered {
		o.entered = true
		typ = events.DragEnter
	}
	d.window.emitEvent(events.DragOfferEvent{EventType: typ, Position: o.position,
		Offer: o, Actions: o.actions, Suggested: suggested, ActionReply: &answer})
	if d.window == nil || d.offer != o {
		return
	}
	if !answer.ValidResult() || answer&o.actions == 0 {
		answer = 0
	}
	flags := int64(2) // request position updates even without pointer motion
	if answer != 0 {
		flags |= 1
	}
	d.platform.sendDragMessage(o.source, d.platform.atoms.XdndStatus, [5]int64{
		int64(d.window.wid), flags, 0, 0, int64(d.platform.atomForDragAction(answer)),
	})
}

func (d *dragService) leave(m *xlib.ClientMessageEvent) {
	if d.offer != nil && d.offer.source == xlib.Window(m.L[0]) && !d.offer.dropped {
		d.leaveCurrent()
	}
}

func (d *dragService) leaveCurrent() {
	o := d.offer
	if o == nil {
		return
	}
	d.offer = nil
	o.finished = true
	if o.entered && d.window != nil {
		d.window.emitEvent(events.DragOfferEvent{EventType: events.DragLeave, Offer: o})
	}
}

func (d *dragService) drop(m *xlib.ClientMessageEvent) {
	o := d.offer
	if o == nil || o.source != xlib.Window(m.L[0]) || d.window == nil || o.dropped || o.finished {
		return
	}
	o.dropped = true
	if !o.entered {
		o.finish(0)
		return
	}
	if m.L[2] != 0 {
		o.time = xlib.Time(m.L[2])
	}
	d.window.emitEvent(events.DragOfferEvent{EventType: events.DragDrop, Position: o.position,
		Offer: o, Actions: o.actions, Suggested: o.suggested})
	if d.offer == o && !o.reading && !o.finished {
		o.finish(0)
	}
}

func (p *Platform) handleDragSelection(ev *xlib.SelectionEvent) bool {
	if ev.Selection != p.atoms.XdndSelection {
		return false
	}
	w := windowMap[ev.Requestor]
	if w == nil || w.dnd == nil || w.dnd.offer == nil {
		return true
	}
	d, o := w.dnd, w.dnd.offer
	if !o.reading || o.finished || ev.Target != o.types[o.format] {
		return true
	}
	var data *dragdrop.Data
	var err error
	if ev.Property == 0 {
		err = fmt.Errorf("x11 dragdrop: selection conversion refused")
	} else {
		var raw []byte
		var typ xlib.Atom
		raw, typ, err = p.readDragBytes(w.wid, ev.Property)
		p.display.DeleteProperty(w.wid, ev.Property)
		if err == nil && typ == p.atoms.INCR {
			o.incr = true
			o.startReadTimeout()
			p.display.Flush()
			return true
		}
		if err == nil && typ != ev.Target {
			err = fmt.Errorf("x11 dragdrop: unexpected selection type %d", typ)
		}
		if err == nil {
			data, err = decodeDragData(o.format, raw)
		}
	}
	d.completeRead(o, data, err)
	return true
}

func (d *dragService) completeRead(o *xdndOffer, data *dragdrop.Data, err error) {
	if o.readTimer != nil {
		o.readTimer.Stop()
		o.readTimer = nil
	}
	o.buffer = nil
	if d.window != nil {
		d.window.emitEvent(events.DragDataEvent{OfferID: o.id, Format: o.format, Data: data, Err: err})
	}
	if d.offer == o && !o.finished {
		o.finish(0)
	}
}

func (o *xdndOffer) startReadTimeout() {
	if o.readTimer != nil {
		o.readTimer.Stop()
	}
	d := o.service
	if d.platform.eventLoop == nil {
		return
	}
	o.readTimer = time.AfterFunc(10*time.Second, func() {
		d.platform.eventLoop.Post(func() {
			if d.offer == o && o.reading && !o.finished {
				d.completeRead(o, nil, fmt.Errorf("x11 dragdrop: selection transfer timed out"))
			}
		})
	})
}

func (p *Platform) handleDragProperty(ev *xlib.PropertyEvent) bool {
	w := windowMap[ev.Window]
	if w == nil || w.dnd == nil || w.dnd.offer == nil ||
		ev.Atom != p.atoms.GOUI_DND || ev.State != 0 {
		return false
	}
	d, o := w.dnd, w.dnd.offer
	if !o.incr || !o.reading || o.finished {
		return false
	}
	raw, typ, err := p.readDragBytes(w.wid, ev.Atom)
	p.display.DeleteProperty(w.wid, ev.Atom)
	if err == nil && typ != o.types[o.format] {
		err = fmt.Errorf("x11 dragdrop: unexpected INCR chunk type %d", typ)
	}
	if err != nil {
		d.completeRead(o, nil, err)
		return true
	}
	if len(raw) == 0 {
		data, decodeErr := decodeDragData(o.format, o.buffer)
		d.completeRead(o, data, decodeErr)
		return true
	}
	if len(raw) > dragdrop.MaxDataBytes-len(o.buffer) {
		d.completeRead(o, nil, fmt.Errorf("x11 dragdrop: INCR transfer exceeds %d bytes", dragdrop.MaxDataBytes))
		return true
	}
	o.buffer = append(o.buffer, raw...)
	o.startReadTimeout()
	p.display.Flush()
	return true
}

func decodeDragData(format dragdrop.Format, raw []byte) (*dragdrop.Data, error) {
	data := new(dragdrop.Data)
	switch format {
	case dragdrop.FormatFiles, dragdrop.FormatURLs:
		urls, err := dragdrop.DecodeURIList(raw)
		if err != nil {
			return nil, err
		}
		if format == dragdrop.FormatURLs {
			data.SetURLs(urls)
		} else {
			if len(urls) == 0 {
				return nil, fmt.Errorf("x11 dragdrop: empty file list")
			}
			paths := make([]string, 0, len(urls))
			for _, uri := range urls {
				path, err := dragdrop.FilePathFromURL(uri)
				if err != nil {
					return nil, err
				}
				paths = append(paths, path)
			}
			data.SetFiles(paths)
		}
	case dragdrop.FormatText:
		if !utf8.Valid(raw) || slices.Contains(raw, 0) {
			return nil, fmt.Errorf("x11 dragdrop: invalid UTF-8 text")
		}
		data.SetText(string(raw))
	default:
		data.SetBytes(strings.TrimPrefix(string(format), "mime:"), raw)
	}
	return data, data.Validate()
}

func (p *Platform) readDragAtoms(w xlib.Window, property xlib.Atom) []xlib.Atom {
	var typ xlib.Atom
	var format int32
	var nitems, after uint
	var raw *byte
	status := p.display.GetWindowProperty(w, property, 0, dragdrop.MaxItems, false,
		xlib.AtomAtom, &typ, &format, &nitems, &after, &raw)
	if raw != nil {
		defer xlib.Free(raw)
	}
	if status != 0 || typ != xlib.AtomAtom || format != 32 || after != 0 || nitems > dragdrop.MaxItems || raw == nil {
		return nil
	}
	values := unsafe.Slice((*uintptr)(unsafe.Pointer(raw)), nitems)
	atoms := make([]xlib.Atom, len(values))
	for i, value := range values {
		atoms[i] = xlib.Atom(value)
	}
	return atoms
}

func (p *Platform) readDragBytes(w xlib.Window, property xlib.Atom) ([]byte, xlib.Atom, error) {
	var typ xlib.Atom
	var format int32
	var nitems, after uint
	var raw *byte
	status := p.display.GetWindowProperty(w, property, 0, dragdrop.MaxDataBytes/4, false,
		0, &typ, &format, &nitems, &after, &raw)
	if raw != nil {
		defer xlib.Free(raw)
	}
	if status != 0 || after != 0 || (nitems != 0 && raw == nil) {
		return nil, typ, fmt.Errorf("x11 dragdrop: invalid or oversized selection reply")
	}
	if typ == p.atoms.INCR && format == 32 && nitems == 1 {
		return nil, typ, nil
	}
	if format != 8 || nitems > dragdrop.MaxDataBytes {
		return nil, typ, fmt.Errorf("x11 dragdrop: unsupported or oversized selection reply")
	}
	if nitems == 0 {
		return []byte{}, typ, nil
	}
	return slices.Clone(unsafe.Slice(raw, nitems)), typ, nil
}

type xdndSource struct {
	service         *dragService
	window          *Window
	id              uint64
	actions         dragdrop.Action
	wire            map[xlib.Atom][]byte
	types           []xlib.Atom
	target          xlib.Window // actual target, not XdndProxy
	proxy           xlib.Window // message destination, possibly the target itself
	version         uint8
	accepted        dragdrop.Action
	awaiting        bool
	pending         bool
	released        bool
	dropped         bool
	cancelRequested bool
	position        geometry.Point
	positionValid   bool
	localTarget     bool
	x, y            int
	stamp           xlib.Time
	state           uint32
	releasedAt      time.Time
	progress        time.Time
	timer           *time.Timer
	transfers       map[xdndTransferKey]*xdndTransfer
	preview         *xdndPreview
	cursors         map[dragdrop.Action]xlib.Cursor
	feedback        dragdrop.Feedback
}

type xdndTransferKey struct {
	requestor xlib.Window
	property  xlib.Atom
}

type xdndTransfer struct {
	typ    xlib.Atom
	data   []byte
	offset int
}

func (d *dragService) beginSource(id uint64, data *dragdrop.Data, actions dragdrop.Action, preview dragdrop.Preview, feedback dragdrop.Feedback) error {
	p := d.platform
	if d.window == nil || d.window.wid == 0 || p.dragSource != nil || p.eventLoop == nil {
		return fmt.Errorf("x11 dragdrop: source unavailable or busy")
	}
	if id == 0 || actions == 0 || !actions.ValidSet() {
		return fmt.Errorf("x11 dragdrop: invalid source id or actions")
	}
	if err := data.Validate(); err != nil {
		return err
	}
	if err := preview.Validate(); err != nil {
		return err
	}
	press := moveResizePress
	if press.window != d.window || press.event.Button != xlib.Button1 || press.event.SendEvent != 0 {
		return fmt.Errorf("x11 dragdrop: Begin requires current native left press")
	}
	copy := data.Clone()
	wire, types, err := p.dragWireData(copy)
	if err != nil {
		return err
	}
	visual, err := newXDNDPreview(preview)
	if err != nil {
		return err
	}
	s := &xdndSource{service: d, window: d.window, id: id, actions: actions,
		feedback: feedback,
		preview:  visual, cursors: make(map[dragdrop.Action]xlib.Cursor),
		wire: wire, types: types, x: int(press.event.XRoot), y: int(press.event.YRoot),
		stamp: press.event.Time, state: press.event.State}
	// The current press has an implicit grab owned by this client. An explicit
	// grab replaces it so motion and release continue to arrive outside us.
	const mask = xlib.EventMaskPointerMotion | xlib.EventMaskButtonRelease
	if status := p.display.GrabPointer(d.window.wid, false, mask, s.cursor(0), press.event.Time); status != 0 {
		s.destroyVisuals()
		return fmt.Errorf("x11 dragdrop: XGrabPointer status %d", status)
	}
	p.display.SetSelectionOwner(p.atoms.XdndSelection, d.window.wid, press.event.Time)
	if p.display.GetSelectionOwner(p.atoms.XdndSelection) != d.window.wid {
		p.display.UngrabPointer(press.event.Time)
		s.destroyVisuals()
		return fmt.Errorf("x11 dragdrop: could not own XdndSelection")
	}
	if len(types) > 3 {
		p.display.ChangeProperty(d.window.wid, p.atoms.XdndTypeList, xlib.AtomAtom,
			32, xlib.PropModeReplace, cgo.CSlice(types), len(types))
	}
	var actionAtoms []xlib.Atom
	for _, action := range []dragdrop.Action{dragdrop.Copy, dragdrop.Move, dragdrop.Link} {
		if actions&action != 0 {
			actionAtoms = append(actionAtoms, p.atomForDragAction(action))
		}
	}
	p.display.ChangeProperty(d.window.wid, p.atoms.XdndActionList, xlib.AtomAtom,
		32, xlib.PropModeReplace, cgo.CSlice(actionAtoms), len(actionAtoms))
	p.display.Flush()
	p.dragSource = s
	d.window.dragPress = false
	d.window.emitEvent(events.DragSourceEvent{EventType: events.DragSourceBegin, ID: id})
	if p.dragSource == s && s.window.wid != 0 {
		s.motion(s.x, s.y, s.stamp, s.state)
	}
	return nil
}

func (p *Platform) dragWireData(data *dragdrop.Data) (map[xlib.Atom][]byte, []xlib.Atom, error) {
	wire := make(map[xlib.Atom][]byte)
	var types []xlib.Atom
	add := func(atom xlib.Atom, value []byte) {
		if _, exists := wire[atom]; !exists {
			types = append(types, atom)
		}
		wire[atom] = value
	}
	if data != nil {
		if value, ok := data.Text(); ok {
			add(p.atoms.UTF8_STRING, []byte(value))
			add(p.atoms.TEXT_UTF8, []byte(value))
		}
		var urls []string
		if files, ok := data.Files(); ok {
			for _, path := range files {
				uri, err := dragdrop.FileURLFromPath(path)
				if err != nil {
					return nil, nil, err
				}
				urls = append(urls, uri)
			}
		} else if represented, ok := data.URLs(); ok {
			urls = represented
		}
		if len(urls) != 0 {
			encoded, err := dragdrop.EncodeURIList(urls)
			if err != nil {
				return nil, nil, err
			}
			add(p.atoms.URI_LIST, encoded)
		}
		for _, format := range data.Formats() {
			if !strings.HasPrefix(string(format), "mime:") {
				continue
			}
			name := strings.TrimPrefix(string(format), "mime:")
			value, _ := data.Bytes(name)
			add(p.display.InternAtom(name, false), value)
		}
	}
	add(p.atoms.GOUI_LOCAL, []byte{})
	return wire, types, nil
}

func (s *xdndSource) handleNative(event *xlib.Event) bool {
	p := s.service.platform
	switch event.Type {
	case xlib.MotionNotify:
		ev := event.MotionEvent()
		if ev.Window != s.window.wid {
			return false
		}
		s.motion(int(ev.XRoot), int(ev.YRoot), ev.Time, ev.State)
		return true
	case xlib.ButtonRelease:
		ev := event.ButtonEvent()
		if ev.Button != xlib.Button1 {
			return true
		}
		// Refresh both the target and its final position. Motion can be
		// coalesced; the release is authoritative, not the last MotionNotify.
		s.motion(int(ev.XRoot), int(ev.YRoot), ev.Time, ev.State)
		s.release(ev.Time)
		return true
	case xlib.KeyPress:
		key, _ := keyFromNativeEvent(event.KeyEvent())
		if key == events.KeyEscape {
			s.cancel()
			return true
		}
	case xlib.ClientMessage:
		m := event.ClientMessageEvent()
		if m.Window != s.window.wid {
			return false
		}
		switch m.MessageType {
		case p.atoms.XdndStatus:
			s.status(m)
			return true
		case p.atoms.XdndFinished:
			s.finished(m)
			return true
		}
	}
	return false
}

func (s *xdndSource) motion(x, y int, stamp xlib.Time, state uint32) {
	p := s.service.platform
	if p.dragSource != s || s.released {
		return
	}
	s.x, s.y, s.stamp, s.state = x, y, stamp, state
	s.preview.move(x, y)
	target, proxy, version := p.dragTargetAt(x, y)
	if target == s.window.wid && s.service.formats == nil {
		// The source's own surface is not a destination unless it registered one.
		target, proxy = 0, 0
	}
	if target != s.target {
		s.leaveTarget()
		s.target, s.proxy, s.version = target, proxy, version
		if target != 0 {
			s.sendEnter()
		}
	}
	if target == 0 {
		return
	}
	if s.awaiting {
		s.pending = true
		return
	}
	s.sendPosition()
}

func (s *xdndSource) sendEnter() {
	p := s.service.platform
	var values [5]int64
	values[0] = int64(s.window.wid)
	values[1] = int64(s.version) << 24
	if len(s.types) > 3 {
		values[1] |= 1
	}
	for i, atom := range s.types[:min(len(s.types), 3)] {
		values[i+2] = int64(atom)
	}
	p.sendDragMessageWindow(s.proxy, s.target, p.atoms.XdndEnter, values)
}

func (s *xdndSource) sendPosition() {
	p := s.service.platform
	s.awaiting, s.pending = true, false
	suggested := s.suggestedAction()
	packed := int64(uint32(uint16(s.x))<<16 | uint32(uint16(s.y)))
	p.sendDragMessageWindow(s.proxy, s.target, p.atoms.XdndPosition, [5]int64{
		int64(s.window.wid), int64(s.state & 0xff), packed, int64(s.stamp),
		int64(p.atomForDragAction(suggested)),
	})
	s.progress = time.Now()
	s.scheduleTimeout()
}

func (s *xdndSource) suggestedAction() dragdrop.Action {
	if s.state&xlib.ShiftMask != 0 && s.actions&dragdrop.Move != 0 {
		return dragdrop.Move
	}
	if s.state&xlib.Mod1Mask != 0 && s.actions&dragdrop.Link != 0 {
		return dragdrop.Link
	}
	for _, action := range []dragdrop.Action{dragdrop.Copy, dragdrop.Move, dragdrop.Link} {
		if s.actions&action != 0 {
			return action
		}
	}
	return 0
}

func (s *xdndSource) status(m *xlib.ClientMessageEvent) {
	if s.target == 0 || s.target != xlib.Window(m.L[0]) || !s.awaiting || s.dropped {
		return
	}
	s.awaiting = false
	s.accepted = 0
	if m.L[1]&1 != 0 {
		action := s.service.platform.dragActionForAtom(xlib.Atom(m.L[4]))
		if action.ValidResult() && s.actions&action != 0 {
			s.accepted = action
		}
	}
	s.updateCursor()
	s.progress = time.Now()
	s.scheduleTimeout()
	if s.pending {
		// The reply acknowledged an older position. Negotiate the final point
		// before deciding to drop, including when the button is already up.
		s.sendPosition()
	} else if s.released {
		s.dropOrEnd()
	}
}

func (s *xdndSource) release(stamp xlib.Time) {
	if s.released {
		return
	}
	s.released, s.stamp = true, stamp
	s.releasedAt = time.Now()
	s.progress = s.releasedAt
	// motion has resolved the target at the ButtonRelease root coordinates.
	// Preserve that fact before asynchronous status/drop completion can alter it.
	if target := windowMap[s.target]; target != nil && target.dnd != nil {
		s.localTarget = len(target.dnd.formats) != 0
	}
	p := s.service.platform
	x, y, ok := p.display.TranslateCoordinatesChecked(p.defScreen.Root, s.window.wid, s.x, s.y)
	if ok {
		s.position = geometry.Point{X: float32(x) / currentScale(), Y: float32(y) / currentScale()}
		s.positionValid = true
	}
	s.window.dragPress = false
	s.window.buttons &^= events.PointerButtonLeftDown
	s.service.platform.display.UngrabPointer(stamp)
	s.preview.hide()
	if !s.awaiting {
		s.dropOrEnd()
	} else {
		s.scheduleTimeout()
	}
}

func (s *xdndSource) dropOrEnd() {
	if s.target == 0 || s.accepted == 0 {
		s.end(dragdrop.Result{})
		return
	}
	s.dropped = true
	p := s.service.platform
	p.sendDragMessageWindow(s.proxy, s.target, p.atoms.XdndDrop,
		[5]int64{int64(s.window.wid), 0, int64(s.stamp)})
	s.progress = time.Now()
	s.scheduleTimeout()
}

func (s *xdndSource) finished(m *xlib.ClientMessageEvent) {
	if !s.dropped || s.target != xlib.Window(m.L[0]) {
		return
	}
	action := s.accepted
	if s.version >= 5 {
		if m.L[1]&1 == 0 {
			action = 0
		} else {
			action = s.service.platform.dragActionForAtom(xlib.Atom(m.L[2]))
		}
	}
	if !action.ValidResult() || action&s.actions == 0 {
		action = 0
	}
	s.end(dragdrop.Result{Action: action, Canceled: s.cancelRequested && action == 0})
}

func (s *xdndSource) leaveTarget() {
	if s.target != 0 && !s.dropped {
		p := s.service.platform
		p.sendDragMessageWindow(s.proxy, s.target, p.atoms.XdndLeave,
			[5]int64{int64(s.window.wid)})
	}
	s.target, s.proxy, s.accepted = 0, 0, 0
	s.updateCursor()
	s.awaiting, s.pending = false, false
}

func (s *xdndSource) cancel() {
	if s.service.platform.dragSource != s {
		return
	}
	if s.dropped {
		s.cancelRequested = true
		return
	}
	s.leaveTarget()
	s.end(dragdrop.Result{Canceled: true})
}

func (s *xdndSource) end(result dragdrop.Result) {
	p := s.service.platform
	if p.dragSource != s {
		return
	}
	if !result.Canceled && result.Err == nil && s.window.wid != 0 {
		result.Position, result.PositionValid = s.position, s.positionValid
		result.LocalTarget = s.positionValid && s.localTarget
	}
	p.dragSource = nil
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	for key := range s.transfers {
		s.removeTransfer(key)
	}
	p.display.UngrabPointer(0)
	s.destroyVisuals()
	if p.display.GetSelectionOwner(p.atoms.XdndSelection) == s.window.wid {
		p.display.SetSelectionOwner(p.atoms.XdndSelection, 0, 0)
	}
	if s.window.wid != 0 {
		p.display.DeleteProperty(s.window.wid, p.atoms.XdndTypeList)
		p.display.DeleteProperty(s.window.wid, p.atoms.XdndActionList)
	}
	p.display.Flush()
	if s.window.wid != 0 {
		s.window.emitEvent(events.DragSourceEvent{EventType: events.DragSourceEnd,
			ID: s.id, Result: result})
	}
}

// replyDeadline bounds protocol completion after release, not the user's
// time spent dragging. While held, even a silent target can be left or canceled;
// no response is required merely because the pointer stays still.
func (s *xdndSource) replyDeadline() time.Time {
	if !s.released || (!s.awaiting && !s.dropped) {
		return time.Time{}
	}
	deadline := s.releasedAt.Add(30 * time.Second)
	if idle := s.progress.Add(10 * time.Second); idle.Before(deadline) {
		deadline = idle
	}
	return deadline
}

func (s *xdndSource) scheduleTimeout() {
	p := s.service.platform
	loop := p.eventLoop
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	deadline := s.replyDeadline()
	if loop == nil || p.dragSource != s || deadline.IsZero() {
		return
	}
	s.timer = time.AfterFunc(max(0, time.Until(deadline)), func() {
		loop.Post(func() {
			if p.dragSource != s {
				return
			}
			// Stop does not retract a task already posted to the GUI thread.
			// A later reply may have cleared or extended the current wait.
			current := s.replyDeadline()
			if current.IsZero() {
				return
			}
			if time.Now().Before(current) {
				s.scheduleTimeout()
				return
			}
			s.leaveTarget()
			s.end(dragdrop.Result{Err: errors.New("x11 dragdrop: native reply timed out")})
		})
	})
}

func (p *Platform) dragTargetAt(x, y int) (target, proxy xlib.Window, version uint8) {
	current := p.defScreen.Root
	for depth := 0; depth < 32; depth++ {
		_, _, child, ok := p.display.TranslateCoordinatesChild(p.defScreen.Root, current, x, y)
		if !ok || child == 0 || child == current {
			break
		}
		current = child
		if value, err := windowProperty32(p.display, child, p.atoms.XdndAware, xlib.AtomAtom, 1); err == nil && len(value) == 1 && value[0] >= 3 {
			target, proxy, version = child, child, uint8(min(value[0], 5))
		}
		if prop, err := windowProperty32(p.display, child, p.atoms.XdndProxy, xlib.AtomWindow, 1); err == nil && len(prop) == 1 && prop[0] != 0 {
			candidate := xlib.Window(prop[0])
			self, selfErr := windowProperty32(p.display, candidate, p.atoms.XdndProxy, xlib.AtomWindow, 1)
			aware, awareErr := windowProperty32(p.display, candidate, p.atoms.XdndAware, xlib.AtomAtom, 1)
			if selfErr == nil && awareErr == nil && len(self) == 1 && xlib.Window(self[0]) == candidate && len(aware) == 1 && aware[0] >= 3 {
				target, proxy, version = child, candidate, uint8(min(aware[0], 5))
			}
		}
	}
	return
}

func (s *xdndSource) handleSelectionRequest(ev *xlib.SelectionRequestEvent) bool {
	p := s.service.platform
	if ev.Selection != p.atoms.XdndSelection || ev.Owner != s.window.wid {
		return false
	}
	property := ev.Property
	if property == 0 { // obsolete requestor
		property = ev.Target
	}
	if ev.Target == p.atoms.TARGETS {
		list := append([]xlib.Atom{p.atoms.TARGETS}, s.types...)
		p.display.ChangeProperty(ev.Requestor, property, xlib.AtomAtom, 32,
			xlib.PropModeReplace, cgo.CSlice(list), len(list))
	} else if value, ok := s.wire[ev.Target]; ok {
		if len(value) <= 64<<10 {
			p.display.ChangeProperty(ev.Requestor, property, ev.Target, 8,
				xlib.PropModeReplace, cgo.CSlice(value), len(value))
		} else {
			key := xdndTransferKey{requestor: ev.Requestor, property: property}
			if s.transfers == nil {
				s.transfers = make(map[xdndTransferKey]*xdndTransfer)
			}
			if _, exists := s.transfers[key]; exists {
				s.removeTransfer(key)
			}
			if windowMap[ev.Requestor] == nil {
				p.display.SelectInput(ev.Requestor, xlib.EventMaskPropertyChange)
			}
			s.transfers[key] = &xdndTransfer{typ: ev.Target, data: value}
			size := []xlib.Atom{xlib.Atom(len(value))}
			p.display.ChangeProperty(ev.Requestor, property, p.atoms.INCR, 32,
				xlib.PropModeReplace, cgo.CSlice(size), len(size))
		}
	} else {
		property = 0
	}
	var reply xlib.Event
	sel := reply.SelectionEvent()
	sel.Type, sel.Requestor, sel.Selection, sel.Target, sel.Property, sel.Time =
		xlib.SelectionNotify, ev.Requestor, ev.Selection, ev.Target, property, ev.Time
	p.display.SendEvent(ev.Requestor, false, 0, &reply)
	p.display.Flush()
	return true
}

func (s *xdndSource) handleProperty(ev *xlib.PropertyEvent) bool {
	if ev.State != 1 { // PropertyDelete acknowledges each INCR chunk.
		return false
	}
	key := xdndTransferKey{requestor: ev.Window, property: ev.Atom}
	transfer := s.transfers[key]
	if transfer == nil {
		return false
	}
	const chunkSize = 64 << 10
	remaining := len(transfer.data) - transfer.offset
	count := min(remaining, chunkSize)
	chunk := transfer.data[transfer.offset : transfer.offset+count]
	s.service.platform.display.ChangeProperty(key.requestor, key.property, transfer.typ, 8,
		xlib.PropModeReplace, cgo.CSlice(chunk), count)
	transfer.offset += count
	if count == 0 {
		s.removeTransfer(key)
	}
	s.progress = time.Now()
	s.scheduleTimeout()
	s.service.platform.display.Flush()
	return true
}

func (s *xdndSource) removeTransfer(key xdndTransferKey) {
	delete(s.transfers, key)
	if windowMap[key.requestor] != nil {
		return // GOUI windows already listen for PropertyNotify.
	}
	for other := range s.transfers {
		if other.requestor == key.requestor {
			return
		}
	}
	s.service.platform.display.SelectInput(key.requestor, 0)
}

func (s *xdndSource) handleSelectionClear(ev *xlib.SelectionClearEvent) bool {
	if ev.Selection != s.service.platform.atoms.XdndSelection {
		return false
	}
	s.leaveTarget()
	s.end(dragdrop.Result{Err: errors.New("x11 dragdrop: lost XdndSelection ownership")})
	return true
}

// The session owns a separate, input-transparent surface. The pointer always
// remains a normal action cursor; preview dimensions never become cursor size.
type xdndPreview struct {
	window     *Window
	hotX, hotY int
	shown      bool
}

func previewRaster(preview dragdrop.Preview, physical float32) (*image.RGBA, int, int, error) {
	scale := preview.Scale
	if scale == 0 {
		scale = physical
	}
	bounds := preview.Image.Bounds()
	w := math.Round(float64(bounds.Dx()) * float64(physical/scale))
	h := math.Round(float64(bounds.Dy()) * float64(physical/scale))
	hotX := math.Round(float64(preview.Hotspot.X * physical))
	hotY := math.Round(float64(preview.Hotspot.Y * physical))
	if w <= 0 || h <= 0 || w > 16384 || h > 16384 || w*h > 16<<20 ||
		math.Abs(hotX) > 16384 || math.Abs(hotY) > 16384 {
		return nil, 0, 0, fmt.Errorf("x11 dragdrop: preview exceeds physical size limit")
	}
	raster := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	xdraw.CatmullRom.Scale(raster, raster.Bounds(), preview.Image, bounds, xdraw.Src, nil)
	return raster, int(hotX), int(hotY), nil
}

func newXDNDPreview(preview dragdrop.Preview) (*xdndPreview, error) {
	if preview.Image == nil {
		return nil, nil
	}
	raster, hotX, hotY, err := previewRaster(preview, currentScale())
	if err != nil {
		return nil, err
	}
	display := platform.display
	major, minor, ok := xshape.QueryVersion(display)
	// Input regions require Shape 1.1. Without it, keep action feedback but
	// omit the visual rather than create a surface that intercepts the drop.
	if !ok || major < 1 || (major == 1 && minor < 1) {
		return nil, nil
	}
	bitmap := graphics.CopyToBitmap(raster, graphics.PixelFormatBGRA, nil)
	p := &xdndPreview{hotX: hotX, hotY: hotY}
	paint := func(event events.Event) {
		if event.Type() == events.Paint && p.window != nil {
			_ = p.window.drawImage(bitmap)
			display.Flush()
		}
	}
	p.window, err = newNativeWindow(paint, true, raster.Rect.Dx(), raster.Rect.Dy(), true)
	if errors.Is(err, common.ErrUnavailable) || errors.Is(err, common.ErrUnsupported) {
		p.window, err = newNativeWindow(paint, true, raster.Rect.Dx(), raster.Rect.Dy(), false)
		if err == nil {
			// Without a compositor X11 only supports binary window coverage.
			// Use the same image with a bounding shape, not a preview cursor.
			rects := opaquePreview(&bitmap)
			// Keep each request below the core X11 request-size limit, even for
			// fragmented alpha masks. An empty image sets an empty shape.
			first := min(len(rects), 4096)
			xshape.CombineRectangles(display, p.window.wid, xshape.Bounding, 0, 0, rects[:first], xshape.Set, xshape.Unsorted)
			for rects = rects[first:]; len(rects) > 0; {
				n := min(len(rects), 4096)
				xshape.CombineRectangles(display, p.window.wid, xshape.Bounding, 0, 0, rects[:n], xshape.Union, xshape.Unsorted)
				rects = rects[n:]
			}
		}
	}
	if err != nil {
		return nil, err
	}
	xshape.CombineRectangles(display, p.window.wid, xshape.Input, 0, 0, nil, xshape.Set, xshape.Unsorted)
	if err := p.window.drawImage(bitmap); err != nil {
		p.window.Destroy()
		return nil, err
	}
	return p, nil
}

// opaquePreview returns horizontal runs of alpha >= 50%, and unpremultiplies
// covered pixels for the non-composited visual. Fractional alpha is unavailable.
func opaquePreview(bitmap *graphics.Bitmap) []xshape.Rectangle {
	var rectangles []xshape.Rectangle
	w, h := bitmap.Bounds().Dx(), bitmap.Bounds().Dy()
	for y := 0; y < h; y++ {
		start := -1
		for x := 0; x <= w; x++ {
			i := y*bitmap.Stride + x*4
			if x < w && bitmap.Pixels[i+3] >= 128 {
				if start < 0 {
					start = x
				}
				a := uint32(bitmap.Pixels[i+3])
				for c := range 3 {
					bitmap.Pixels[i+c] = byte(min(255, uint32(bitmap.Pixels[i+c])*255/a))
				}
				bitmap.Pixels[i+3] = 255
			} else if start >= 0 {
				rectangles = append(rectangles, xshape.Rectangle{X: int16(start), Y: int16(y), Width: uint16(x - start), Height: 1})
				start = -1
			}
		}
	}
	return rectangles
}

func (p *xdndPreview) move(x, y int) {
	if p == nil {
		return
	}
	display := platform.display
	display.MoveWindow(p.window.wid, x-p.hotX, y-p.hotY)
	if !p.shown {
		_ = p.window.Show()
		p.shown = true
	}
	display.RaiseWindow(p.window.wid)
	display.Flush()
}

func (p *xdndPreview) hide() {
	if p != nil && p.shown {
		_ = p.window.Hide()
		p.shown = false
	}
}

func (s *xdndSource) cursor(action dragdrop.Action) xlib.Cursor {
	// A private cache key distinguishes neutral fallback from native rejection.
	if action == 0 && s.feedback.NeutralOutsideTargets && !s.overLocalTarget() {
		action = 0x80
	}
	if cursor := s.cursors[action]; cursor != 0 {
		return cursor
	}
	display := s.service.platform.display
	names := []string{"dnd-none", "not-allowed"}
	switch action {
	case 0x80:
		names = []string{"left_ptr", "default"}
	case dragdrop.Copy:
		names = []string{"dnd-copy", "copy"}
	case dragdrop.Move:
		names = []string{"dnd-move", "move"}
	case dragdrop.Link:
		names = []string{"dnd-link", "alias"}
	}
	var cursor xlib.Cursor
	if xcursor.Available() {
		for _, name := range names {
			cursor = xcursor.LibraryLoadCursor(display, name)
			if cursor != 0 {
				break
			}
		}
	}
	if cursor == 0 {
		cursor = display.CreateFontCursor(68)
	} // XC_left_ptr
	s.cursors[action] = cursor
	return cursor
}

func (s *xdndSource) overLocalTarget() bool {
	// Use the current native pointer location, not a previously visited target.
	target := windowMap[s.target]
	return target != nil && target.dnd != nil && len(target.dnd.formats) != 0
}

func (s *xdndSource) updateCursor() {
	if !s.released {
		s.service.platform.display.ChangeActivePointerGrab(xlib.EventMaskPointerMotion|xlib.EventMaskButtonRelease, s.cursor(s.accepted), 0)
	}
}

func (s *xdndSource) destroyVisuals() {
	if s.preview != nil {
		s.preview.window.Destroy()
		s.preview = nil
	}
	for _, cursor := range s.cursors {
		s.service.platform.display.FreeCursor(cursor)
	}
	s.cursors = nil
}
