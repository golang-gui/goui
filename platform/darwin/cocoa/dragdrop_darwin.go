package cocoa

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"slices"
	"strings"

	"github.com/ebitengine/purego/objc"
	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/platform/common"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/appkit"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/core_graphics"
	. "github.com/golang-gui/goui/platform/darwin/frameworks/foundation"
	"github.com/golang-gui/goui/platform/dragdrop"
	"github.com/golang-gui/goui/platform/events"
)

const (
	dragTextType   = "public.utf8-plain-text"
	dragFileType   = "public.file-url"
	dragURLType    = "public.url"
	dragLocalType  = "org.golang-gui.goui.local-drag"
	dragMIMEPrefix = "org.golang-gui.goui.mime."
)

type dragService struct {
	window  *Window
	formats []dragdrop.Format
	offer   *macOffer
	source  *macSource
}

type macSource struct {
	service  *dragService
	view     NSView
	onEvent  events.EventHandler
	id       uint64
	actions  dragdrop.Action
	canceled bool
	ended    bool
	session  DraggingSession
}

type macOffer struct {
	service  *dragService
	id       uint64
	sourceID uint64
	info     DraggingInfo
	formats  []dragdrop.Format
	actions  dragdrop.Action
	selected dragdrop.Action
	finished bool
	dropped  bool
	read     bool
}

var activeMacSource *macSource
var nextMacOfferID uint64

func newDragService(surface common.Surface) (common.DragDrop, error) {
	var w *Window
	switch s := surface.(type) {
	case *Window:
		w = s
	case *Popup:
		w = s.win
	default:
		return nil, common.ErrUnsupported
	}
	if w == nil || !w.window.Valid() || w.dnd != nil {
		return nil, common.ErrUnavailable
	}
	d := &dragService{window: w}
	w.dnd = d
	return d, nil
}

func macType(format dragdrop.Format) (string, error) {
	switch format {
	case dragdrop.FormatText:
		return dragTextType, nil
	case dragdrop.FormatFiles:
		return dragFileType, nil
	case dragdrop.FormatURLs:
		return dragURLType, nil
	case dragdrop.FormatLocalMarker:
		return dragLocalType, nil
	default:
		if !strings.HasPrefix(string(format), "mime:") || dragdrop.MIMEFormat(strings.TrimPrefix(string(format), "mime:")) != format {
			return "", fmt.Errorf("cocoa dragdrop: invalid format %q", format)
		}
		return dragMIMEPrefix + hex.EncodeToString([]byte(strings.TrimPrefix(string(format), "mime:"))), nil
	}
}

func (d *dragService) SetFormats(formats []dragdrop.Format) (err error) {
	if d.window == nil || !d.window.window.Valid() {
		return common.ErrUnavailable
	}
	types := make([]objc.ID, 0, len(formats))
	unique := make([]dragdrop.Format, 0, len(formats))
	for _, f := range formats {
		if slices.Contains(unique, f) {
			continue
		}
		if _, err = macType(f); err != nil {
			return err
		}
		unique = append(unique, f)
	}
	AutoReleasePool(func() {
		seenTypes := make(map[string]bool)
		var nativeStrings []NSString
		defer func() {
			for _, s := range nativeStrings {
				s.Release()
			}
		}()
		addType := func(name string) {
			if seenTypes[name] {
				return
			}
			seenTypes[name] = true
			s := ToNSString(name)
			nativeStrings = append(nativeStrings, s)
			types = append(types, s.ID)
		}
		for _, f := range unique {
			typ, _ := macType(f)
			addType(typ)
			if f == dragdrop.FormatURLs {
				addType(dragFileType)
			}
		}
		d.window.view.UnregisterDraggedTypes()
		if len(types) != 0 {
			d.window.view.RegisterDraggedTypes(Cast[NSArray](NSArray_arrayWithObjects(types)))
		}
	})
	d.formats = unique
	return nil
}

func (d *dragService) Destroy() {
	w := d.window
	if w == nil {
		return
	}
	d.Cancel()
	if d.offer != nil {
		d.offer.finished = true
		d.offer = nil
	}
	if w.view.Valid() {
		AutoReleasePool(func() { w.view.UnregisterDraggedTypes() })
	}
	w.dnd = nil
	d.window = nil
}

func (d *dragService) Cancel() {
	if s := d.source; s != nil && !s.ended {
		// AppKit exposes no public cancel method on NSDraggingSession. Refuse
		// subsequent source operations, but keep the native session and source
		// object alive until AppKit sends its terminal callback.
		s.canceled = true
	}
}

func (w *Window) clearDragDown() {
	if w.dragDown.Valid() {
		w.dragDown.Release()
		w.dragDown = NSEvent{}
	}
}

func macOperation(actions dragdrop.Action) uint {
	var native uint
	if actions&dragdrop.Copy != 0 {
		native |= NSDragOperationCopy
	}
	if actions&dragdrop.Move != 0 {
		native |= NSDragOperationMove
	}
	if actions&dragdrop.Link != 0 {
		native |= NSDragOperationLink
	}
	return native
}

func macActions(native uint) dragdrop.Action {
	var actions dragdrop.Action
	if native&NSDragOperationCopy != 0 {
		actions |= dragdrop.Copy
	}
	if native&NSDragOperationMove != 0 {
		actions |= dragdrop.Move
	}
	if native&NSDragOperationLink != 0 {
		actions |= dragdrop.Link
	}
	return actions
}

func (d *dragService) Begin(id uint64, data *dragdrop.Data, actions dragdrop.Action, preview dragdrop.Preview) error {
	w := d.window
	if w == nil || !w.window.Valid() || activeMacSource != nil {
		return common.ErrUnavailable
	}
	if id == 0 || !actions.ValidSet() || actions == 0 {
		return fmt.Errorf("cocoa dragdrop: invalid source ID or actions")
	}
	if err := data.Validate(); err != nil {
		return err
	}
	if err := preview.Validate(); err != nil {
		return err
	}
	if !w.dragDown.Valid() || w.buttons&events.PointerButtonLeftDown == 0 {
		return fmt.Errorf("cocoa dragdrop: no active left-button press")
	}
	var beginErr error
	AutoReleasePool(func() { beginErr = d.beginNative(id, data, actions, preview) })
	return beginErr
}

func (d *dragService) beginNative(id uint64, data *dragdrop.Data, actions dragdrop.Action, preview dragdrop.Preview) error {
	w := d.window
	items, err := makeMacDragItems(data, preview, w)
	if err != nil {
		return err
	}
	for _, item := range items {
		defer item.Release()
	}
	ids := make([]objc.ID, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	s := &macSource{service: d, view: w.view, onEvent: w.onEvent, id: id, actions: actions}
	activeMacSource = s
	d.source = s
	s.session = w.view.BeginDraggingSession(Cast[NSArray](NSArray_arrayWithObjects(ids)), w.dragDown)
	if s.ended {
		// AppKit may synchronously finish a failed drag inside Begin.
		return nil
	}
	if !s.session.Valid() {
		activeMacSource = nil
		d.source = nil
		return fmt.Errorf("cocoa dragdrop: AppKit did not start a session")
	}
	s.session.Retain()
	if d.window == nil || !w.window.Valid() {
		s.canceled = true
		return nil
	}
	w.onEvent(events.DragSourceEvent{EventType: events.DragSourceBegin, ID: id})
	return nil
}

func makeMacDragItems(data *dragdrop.Data, preview dragdrop.Preview, w *Window) ([]DraggingItem, error) {
	var urls []string
	var fileMode bool
	if data != nil {
		if files, ok := data.Files(); ok {
			fileMode = true
			for _, path := range files {
				u, err := dragdrop.FileURLFromPath(path)
				if err != nil {
					return nil, err
				}
				urls = append(urls, u)
			}
		} else if values, ok := data.URLs(); ok {
			urls = values
		}
	}
	count := len(urls)
	if count == 0 {
		count = 1
	}
	items := make([]DraggingItem, 0, count)
	cleanup := func() {
		for _, item := range items {
			item.Release()
		}
	}
	image, frame, err := macPreview(preview, w)
	if err != nil {
		return nil, err
	}
	defer image.Release()
	for i := 0; i < count; i++ {
		pb := NewPasteboardItem()
		if i == 0 {
			if data != nil {
				if text, ok := data.Text(); ok && !macSetString(pb, dragTextType, text) {
					pb.Release()
					cleanup()
					return nil, fmt.Errorf("cocoa dragdrop: cannot set text")
				}
				for _, format := range data.Formats() {
					if !strings.HasPrefix(string(format), "mime:") {
						continue
					}
					value, _ := data.Bytes(strings.TrimPrefix(string(format), "mime:"))
					typ, _ := macType(format)
					if !macSetData(pb, typ, value) {
						pb.Release()
						cleanup()
						return nil, fmt.Errorf("cocoa dragdrop: cannot set %s", format)
					}
				}
			}
			if !macSetString(pb, dragLocalType, "1") {
				pb.Release()
				cleanup()
				return nil, fmt.Errorf("cocoa dragdrop: cannot set session marker")
			}
		}
		if len(urls) > 0 {
			typ := dragURLType
			if fileMode {
				typ = dragFileType
			}
			if !macSetString(pb, typ, urls[i]) {
				pb.Release()
				cleanup()
				return nil, fmt.Errorf("cocoa dragdrop: cannot set URL")
			}
			if fileMode && !macSetString(pb, dragURLType, urls[i]) {
				pb.Release()
				cleanup()
				return nil, fmt.Errorf("cocoa dragdrop: cannot set URL")
			}
		}
		item := NewDraggingItem(pb)
		pb.Release()
		if !item.Valid() {
			cleanup()
			return nil, fmt.Errorf("cocoa dragdrop: cannot create dragging item")
		}
		item.SetFrame(frame, image)
		items = append(items, item)
	}
	return items, nil
}

func macSetString(item PasteboardItem, typ, value string) bool {
	t := ToNSString(typ)
	defer t.Release()
	v := ToNSString(value)
	defer v.Release()
	return item.SetString(v, t)
}
func macSetData(item PasteboardItem, typ string, value []byte) bool {
	t := ToNSString(typ)
	defer t.Release()
	return item.SetData(NewNativeData(value), t)
}

func macPreview(preview dragdrop.Preview, w *Window) (NativeImage, NSRect, error) {
	img := preview.Image
	if img != nil && int64(img.Bounds().Dx())*int64(img.Bounds().Dy()) > 16<<20 {
		return NativeImage{}, NSRect{}, fmt.Errorf("cocoa dragdrop: preview exceeds pixel limit")
	}
	if math.Abs(float64(preview.Hotspot.X)) > 1<<20 || math.Abs(float64(preview.Hotspot.Y)) > 1<<20 {
		return NativeImage{}, NSRect{}, fmt.Errorf("cocoa dragdrop: preview hotspot exceeds native coordinate limit")
	}
	if img == nil {
		// NSDraggingItem needs content even for a local-only payload. Keep the
		// generic icon 24 DIP wide at the effective backing scale.
		physical := float32(w.window.BackingScaleFactor() * pointsPerLogicalUnit(w.window))
		size := max(1, int(math.Round(float64(24*physical))))
		fallback := image.NewRGBA(image.Rect(0, 0, size, size))
		for y := size / 12; y < size-size/12; y++ {
			for x := size / 6; x < size-size/6; x++ {
				fallback.Set(x, y, color.RGBA{R: 245, G: 245, B: 245, A: 255})
			}
		}
		for y := size / 4; y < size*3/4; y += max(1, size/6) {
			for x := size * 7 / 24; x < size*17/24; x++ {
				fallback.Set(x, y, color.RGBA{R: 120, G: 120, B: 120, A: 255})
			}
		}
		img = fallback
		preview.Scale = physical
		preview.Hotspot = geometry.Point{}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return NativeImage{}, NSRect{}, err
	}
	native := NewNativeImage(NewNativeData(encoded.Bytes()))
	if !native.Valid() {
		return NativeImage{}, NSRect{}, fmt.Errorf("cocoa dragdrop: cannot create preview image")
	}
	scale := preview.Scale
	if scale == 0 {
		// Backing pixels / DIP = (backing pixels / point) * (points / DIP).
		// The ratio matters when common's explicit scale differs from Retina.
		scale = float32(w.window.BackingScaleFactor() * pointsPerLogicalUnit(w.window))
	}
	ppu := pointsPerLogicalUnit(w.window)
	width := CGFloat(img.Bounds().Dx()) / CGFloat(scale) * ppu
	height := CGFloat(img.Bounds().Dy()) / CGFloat(scale) * ppu
	point := w.view.ConvertPointFromView(w.dragDown.LocationInWindow(), NSView{})
	frame := NSMakeRect(point.X-CGFloat(preview.Hotspot.X)*ppu, point.Y-height+CGFloat(preview.Hotspot.Y)*ppu, width, height)
	return native, frame, nil
}

func (s *macSource) end(result dragdrop.Result) {
	if s.ended {
		return
	}
	s.ended = true
	if s.session.Valid() {
		s.session.Release()
		s.session = DraggingSession{}
	}
	if activeMacSource == s {
		activeMacSource = nil
	}
	if d := s.service; d != nil {
		if d.source == s {
			d.source = nil
		}
	}
	if s.onEvent != nil {
		s.onEvent(events.DragSourceEvent{EventType: events.DragSourceEnd, ID: s.id, Result: result})
	}
}

func dragSourceMask(view NSView, session DraggingSession, _ uint) uint {
	if s := activeMacSource; s != nil && s.view.ID == view.ID && !s.canceled {
		return macOperation(s.actions)
	}
	return 0
}
func dragSourceEnded(view NSView, _ DraggingSession, _ NSPoint, operation uint) {
	if s := activeMacSource; s != nil && s.view.ID == view.ID {
		result := dragdrop.Result{Action: macActions(operation)}
		if result.Action != dragdrop.Copy && result.Action != dragdrop.Move && result.Action != dragdrop.Link {
			result.Action = 0
		}
		if s.canceled {
			result = dragdrop.Result{Canceled: true}
		}
		s.end(result)
	}
}

func (o *macOffer) ID() uint64       { return o.id }
func (o *macOffer) SourceID() uint64 { return o.sourceID }
func (o *macOffer) Formats() []dragdrop.Format {
	if o.finished {
		return nil
	}
	return slices.Clone(o.formats)
}
func (o *macOffer) Read(format dragdrop.Format) error {
	d := o.service
	if d == nil || d.window == nil || d.offer != o || !o.dropped || o.read || o.finished || !slices.Contains(o.formats, format) {
		return fmt.Errorf("cocoa dragdrop: offer not readable")
	}
	o.read = true
	var data *dragdrop.Data
	var err error
	AutoReleasePool(func() { data, err = readMacOffer(o, format) })
	if d.window != nil && d.offer == o {
		d.window.onEvent(events.DragDataEvent{OfferID: o.id, Format: format, Data: data, Err: err})
	}
	return nil
}
func (o *macOffer) Finish(action dragdrop.Action) error {
	if !action.ValidResult() || action != 0 && (action&o.actions == 0 || action != o.selected) {
		return fmt.Errorf("cocoa dragdrop: invalid finish action")
	}
	if o.service == nil || o.service.offer != o || o.finished || !o.dropped {
		return fmt.Errorf("cocoa dragdrop: offer already finished")
	}
	o.selected = action
	o.finished = true
	return nil
}

func macOfferFormats(pb NSPasteboard, registered []dragdrop.Format) []dragdrop.Format {
	items := pb.Items()
	if !items.Valid() {
		return nil
	}
	nativeTypes := make(map[string]bool)
	for i := uintptr(0); i < items.Count(); i++ {
		item := Cast[PasteboardItem](items.ObjectAtIndex(i))
		types := item.Types()
		for j := uintptr(0); j < types.Count(); j++ {
			nativeTypes[Cast[NSString](types.ObjectAtIndex(j)).UTF8String()] = true
		}
	}
	available := make([]dragdrop.Format, 0, len(registered))
	for _, format := range registered {
		typ, _ := macType(format)
		if nativeTypes[typ] || format == dragdrop.FormatURLs && nativeTypes[dragFileType] {
			available = append(available, format)
		}
	}
	return available
}

func readMacOffer(o *macOffer, format dragdrop.Format) (*dragdrop.Data, error) {
	return readMacPasteboard(o.info.Pasteboard(), format)
}

func readMacPasteboard(pb NSPasteboard, format dragdrop.Format) (*dragdrop.Data, error) {
	items := pb.Items()
	if !items.Valid() {
		return nil, fmt.Errorf("cocoa dragdrop: missing pasteboard items")
	}
	typ, _ := macType(format)
	native := ToNSString(typ)
	defer native.Release()
	data := new(dragdrop.Data)
	switch format {
	case dragdrop.FormatFiles, dragdrop.FormatURLs:
		var values []string
		for i := uintptr(0); i < items.Count(); i++ {
			item := Cast[PasteboardItem](items.ObjectAtIndex(i))
			s := item.String(native)
			if !s.Valid() && format == dragdrop.FormatURLs {
				fallback := ToNSString(dragFileType)
				s = item.String(fallback)
				fallback.Release()
			}
			if s.Valid() {
				values = append(values, s.UTF8String())
			}
		}
		if len(values) == 0 {
			return nil, fmt.Errorf("cocoa dragdrop: missing URL representation")
		}
		if format == dragdrop.FormatFiles {
			paths := make([]string, 0, len(values))
			for _, value := range values {
				path, err := dragdrop.FilePathFromURL(value)
				if err != nil {
					return nil, err
				}
				paths = append(paths, path)
			}
			data.SetFiles(paths)
		} else {
			data.SetURLs(values)
		}
	case dragdrop.FormatText:
		for i := uintptr(0); i < items.Count(); i++ {
			if s := Cast[PasteboardItem](items.ObjectAtIndex(i)).String(native); s.Valid() {
				data.SetText(s.UTF8String())
				break
			}
		}
	case dragdrop.FormatLocalMarker:
		return nil, fmt.Errorf("cocoa dragdrop: local marker has no portable data")
	default:
		for i := uintptr(0); i < items.Count(); i++ {
			if bytes := Cast[PasteboardItem](items.ObjectAtIndex(i)).Data(native); bytes.Valid() {
				if bytes.Length() > dragdrop.MaxDataBytes {
					return nil, fmt.Errorf("cocoa dragdrop: representation exceeds size limit")
				}
				data.SetBytes(strings.TrimPrefix(string(format), "mime:"), bytes.Bytes())
				break
			}
		}
	}
	if !slices.Contains(data.Formats(), format) {
		return nil, fmt.Errorf("cocoa dragdrop: missing %q representation", format)
	}
	if err := data.Validate(); err != nil {
		return nil, err
	}
	return data, nil
}

func macDragPoint(w *Window, info DraggingInfo) geometry.Point {
	point := w.view.ConvertPointFromView(info.Location(), NSView{})
	bounds := w.view.Bounds()
	scale := pointsPerLogicalUnit(w.window)
	return geometry.Point{X: float32((point.X - bounds.Origin.X) / scale), Y: float32((bounds.Origin.Y + bounds.Size.Height - point.Y) / scale)}
}

func dragEntered(view NSView, info DraggingInfo) uint {
	w := windowForView(view)
	if w == nil || w.dnd == nil {
		return 0
	}
	d := w.dnd
	if d.offer != nil {
		d.offer.finished = true
	}
	nextMacOfferID++
	if nextMacOfferID == 0 {
		nextMacOfferID++
	}
	o := &macOffer{service: d, id: nextMacOfferID, info: info, actions: macActions(info.SourceMask())}
	if s := activeMacSource; s != nil && !s.ended && !s.canceled && s.view.ID == info.Source() {
		o.sourceID = s.id
	}
	AutoReleasePool(func() { o.formats = macOfferFormats(info.Pasteboard(), d.formats) })
	d.offer = o
	return d.notify(o, events.DragEnter)
}
func dragUpdated(view NSView, info DraggingInfo) uint {
	w := windowForView(view)
	if w == nil || w.dnd == nil || w.dnd.offer == nil {
		return 0
	}
	w.dnd.offer.info = info
	return w.dnd.notify(w.dnd.offer, events.DragMotion)
}
func (d *dragService) notify(o *macOffer, kind events.EventType) uint {
	if d.window == nil || d.offer != o || o.finished {
		return 0
	}
	var reply dragdrop.Action
	suggested := dragdrop.Copy
	if o.actions&suggested == 0 {
		suggested = 0
	}
	d.window.onEvent(events.DragOfferEvent{EventType: kind, Position: macDragPoint(d.window, o.info), Offer: o,
		Actions: o.actions, Suggested: suggested, ActionReply: &reply})
	if d.offer != o || o.finished {
		return 0
	}
	if reply&^o.actions != 0 || !reply.ValidResult() {
		reply = 0
	}
	o.selected = reply
	return macOperation(reply)
}
func dragExited(view NSView, _ DraggingInfo) {
	w := windowForView(view)
	if w == nil || w.dnd == nil || w.dnd.offer == nil {
		return
	}
	o := w.dnd.offer
	w.dnd.offer = nil
	o.finished = true
	w.onEvent(events.DragOfferEvent{EventType: events.DragLeave, Offer: o})
}
func dragPerformed(view NSView, info DraggingInfo) bool {
	w := windowForView(view)
	if w == nil || w.dnd == nil || w.dnd.offer == nil {
		return false
	}
	d := w.dnd
	o := d.offer
	o.info = info
	if d.notify(o, events.DragMotion) == 0 || d.offer != o {
		return false
	}
	o.dropped = true
	w.onEvent(events.DragOfferEvent{EventType: events.DragDrop, Position: macDragPoint(w, info), Offer: o,
		Actions: o.actions, Suggested: o.selected})
	return d.offer == o && o.finished && o.selected != 0
}
func dragConcluded(view NSView, _ DraggingInfo) {
	w := windowForView(view)
	if w == nil || w.dnd == nil {
		return
	}
	if o := w.dnd.offer; o != nil {
		o.finished = true
		w.dnd.offer = nil
	}
}
