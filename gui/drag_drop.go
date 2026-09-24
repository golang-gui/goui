package gui

import (
	"fmt"
	"image"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform/dragdrop"
	"github.com/golang-gui/goui/platform/events"
)

type DragAction = dragdrop.Action

const (
	DragCopy DragAction = dragdrop.Copy
	DragMove DragAction = dragdrop.Move
	DragLink DragAction = dragdrop.Link
)

type DragFormat = dragdrop.Format

const (
	DragFormatText  DragFormat = dragdrop.FormatText
	DragFormatFiles DragFormat = dragdrop.FormatFiles
	DragFormatURLs  DragFormat = dragdrop.FormatURLs
)

func MIMEFormat(mediaType string) DragFormat { return dragdrop.MIMEFormat(mediaType) }

// LocalFormat names a Go value exchanged only within one Application. A
// foreign offer can never manufacture a local format by advertising this name.
func LocalFormat(name string) DragFormat {
	if name == "" || !utf8.ValidString(name) || strings.ContainsRune(name, 0) {
		return ""
	}
	return DragFormat("local:" + name)
}

// DragData combines finite portable representations with application-local Go
// values. Its zero value is ready to use. Slices and byte payloads are copied;
// Local values are references owned and synchronized by the application.
type DragData struct {
	portable dragdrop.Data
	local    map[string]any
}

func (d *DragData) SetText(value string) { d.portable.SetText(value) }
func (d *DragData) Text() (string, bool) { return d.portable.Text() }

func (d *DragData) SetFiles(paths []string) { d.portable.SetFiles(paths) }
func (d *DragData) Files() ([]string, bool) { return d.portable.Files() }

func (d *DragData) SetURLs(urls []string)  { d.portable.SetURLs(urls) }
func (d *DragData) URLs() ([]string, bool) { return d.portable.URLs() }

func (d *DragData) SetBytes(mediaType string, value []byte) { d.portable.SetBytes(mediaType, value) }
func (d *DragData) Bytes(mediaType string) ([]byte, bool)   { return d.portable.Bytes(mediaType) }

func (d *DragData) SetLocal(name string, value any) {
	if d.local == nil {
		d.local = make(map[string]any)
	}
	d.local[name] = value
}

func (d *DragData) Local(name string) (any, bool) {
	if d == nil {
		return nil, false
	}
	value, ok := d.local[name]
	return value, ok
}

func (d *DragData) formats() []DragFormat {
	if d == nil {
		return nil
	}
	formats := d.portable.Formats()
	keys := make([]string, 0, len(d.local))
	for name := range d.local {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		formats = append(formats, LocalFormat(name))
	}
	return formats
}

func (d *DragData) validate() error {
	if d == nil {
		return fmt.Errorf("gui: no drag data")
	}
	if err := d.portable.Validate(); err != nil {
		return err
	}
	for name := range d.local {
		if LocalFormat(name) == "" {
			return fmt.Errorf("gui: invalid local drag format name %q", name)
		}
	}
	if len(d.formats()) == 0 {
		return fmt.Errorf("gui: empty drag data")
	}
	return nil
}

// freeze copies the transport representations at the Prepare boundary. Local
// object identities intentionally remain shared within the application.
func (d *DragData) freeze() *DragData {
	if d == nil {
		return nil
	}
	copy := &DragData{portable: *d.portable.Clone()}
	if d.local != nil {
		copy.local = make(map[string]any, len(d.local))
		for key, value := range d.local {
			copy.local[key] = value
		}
	}
	return copy
}

func (d *DragData) contains(format DragFormat) bool {
	return slices.Contains(d.formats(), format)
}

func (d *DragData) selected(format DragFormat) *DragData {
	selected := new(DragData)
	switch format {
	case DragFormatText:
		value, _ := d.Text()
		selected.SetText(value)
	case DragFormatFiles:
		value, _ := d.Files()
		selected.SetFiles(value)
	case DragFormatURLs:
		value, _ := d.URLs()
		selected.SetURLs(value)
	default:
		if strings.HasPrefix(string(format), "local:") {
			value, _ := d.Local(strings.TrimPrefix(string(format), "local:"))
			selected.SetLocal(strings.TrimPrefix(string(format), "local:"), value)
		} else if strings.HasPrefix(string(format), "mime:") {
			name := strings.TrimPrefix(string(format), "mime:")
			value, _ := d.Bytes(name)
			selected.SetBytes(name, value)
		}
	}
	return selected
}

type DragPreview struct {
	Image   image.Image
	Scale   float32
	Hotspot geometry.Point
}

type DragPrepare struct {
	Position geometry.Point
	Data     *DragData
	Preview  DragPreview
}

type DragResult = dragdrop.Result

type DragMotion struct {
	Position geometry.Point
	Format   DragFormat
	Allowed  DragAction
	Action   DragAction
}

// DropRequest asks the target to accept data already read in the selected
// format. Only Accepted affects the result, and it must be set synchronously.
type DropRequest struct {
	Position geometry.Point
	Format   DragFormat
	Action   DragAction
	Data     *DragData
	Accepted bool
}

// DragSource is a Capture-phase controller. It only registers a candidate
// during a native left press; the dispatcher coordinates gesture competition
// after ordinary event propagation has finished.
type DragSource struct {
	owner           Widget
	enabled         bool
	actions         DragAction
	armed           bool
	press           geometry.Point
	pressWindow     geometry.Point
	gesture         *GestureParticipation
	prepared        *DragData
	preview         DragPreview
	preparedActions DragAction
	dragging        bool
	prepare         signal.Signal1[*DragPrepare]
	begin           signal.Signal0
	end             signal.Signal1[DragResult]
}

func NewDragSource() *DragSource { return &DragSource{enabled: true, actions: DragCopy} }

func (s *DragSource) Phase() PropagationPhase { return PhaseCapture }
func (s *DragSource) Enabled() bool           { return s.enabled }
func (s *DragSource) Actions() DragAction     { return s.actions }
func (s *DragSource) Dragging() bool          { return s.dragging }
func (s *DragSource) Reset() {
	gesture := s.gesture
	s.gesture = nil
	gesture.Reject()
	s.armed = false
	s.prepared = nil
}
func (s *DragSource) HandleCrossing(CrossingContext) {}

func (s *DragSource) SetEnabled(value bool) {
	if s.enabled == value {
		return
	}
	s.enabled = value
	if !value {
		s.Reset()
		s.Cancel()
	}
	s.notifyChanged()
}

func (s *DragSource) SetActions(value DragAction) {
	if s.actions == value {
		return
	}
	s.actions = value
	s.notifyChanged()
}

func (s *DragSource) Cancel() {
	if s.owner == nil {
		return
	}
	if app := dragAppOf(s.owner.Root()); app != nil && app.dragSession != nil && app.dragSession.source == s {
		app.dragSession.cancel()
	}
}

func (s *DragSource) ConnectPrepare(fn func(*DragPrepare)) signal.Handle {
	return s.prepare.Connect(fn)
}
func (s *DragSource) ConnectBegin(fn func()) signal.Handle { return s.begin.Connect(fn) }
func (s *DragSource) ConnectEnd(fn func(DragResult)) signal.Handle {
	return s.end.Connect(fn)
}

func (s *DragSource) HandleEvent(ctx EventContext) {
	e, ok := ctx.Event().(events.PointerEvent)
	if !ok {
		return
	}
	if e.EventType == events.PointerMove && s.gesture != nil && s.armed && gestureMoved(s.pressWindow, e.Position, gestureDefaultDistance) {
		s.armed = false
		if s.owner == nil || !s.enabled || !s.actions.ValidSet() || s.actions == 0 {
			s.gesture.Reject()
			return
		}
		owner, root := s.owner, s.owner.Root()
		app := dragAppOf(root)
		if app == nil || app.dragSession != nil {
			s.gesture.Reject()
			return
		}
		request := &DragPrepare{Position: s.press}
		s.prepare.Emit(request)
		if s.gesture == nil || s.owner != owner || owner.Root() != root || owner.base().destroyed ||
			!s.enabled || !s.actions.ValidSet() || s.actions == 0 || app.dragSession != nil {
			s.gesture.Reject()
			return
		}
		if request.Data == nil {
			s.gesture.Reject()
			return
		}
		data := request.Data.freeze()
		if err := data.validate(); err != nil {
			s.gesture.Reject()
			s.end.Emit(DragResult{Err: err})
			return
		}
		if s.gesture == nil || s.owner == nil || s.owner.base().destroyed {
			return
		}
		s.prepared, s.preview, s.preparedActions = data, request.Preview, s.actions
		s.gesture.Claim()
		return
	}
	if e.EventType == events.PointerUp && s.gesture != nil && e.Button == events.PointerButtonLeft {
		s.gesture.Reject()
		return
	}
	if e.EventType != events.PointerDown || e.Button != events.PointerButtonLeft || !s.enabled || s.actions == 0 {
		return
	}
	if c, ok := ctx.(*eventContext); ok && c.current != nil {
		if pos, valid := ctx.Position(); valid {
			s.owner, s.press, s.pressWindow, s.armed = c.current, pos, e.Position, true
			s.gesture = JoinGesture(ctx)
		}
	}
}

func (s *DragSource) GestureAccepted(EventContext) {
	gesture, data, preview, actions := s.gesture, s.prepared, s.preview, s.preparedActions
	s.gesture, s.prepared, s.armed = nil, nil, false
	if gesture == nil || data == nil || s.owner == nil {
		return
	}
	root := s.owner.Root()
	app := dragAppOf(root)
	if app == nil {
		return
	}
	gesture.sequence.takeOver()
	if err := app.startDrag(root, s.owner, s, data, preview, actions); err != nil {
		s.end.Emit(DragResult{Err: err})
	}
}

func (s *DragSource) GestureCanceled(GestureCancelReason) {
	s.gesture, s.prepared, s.armed = nil, nil, false
}

func (s *DragSource) setWidget(w Widget) {
	if w == nil && s.dragging {
		s.Cancel()
	}
	s.owner = w
	if w == nil {
		s.Reset()
	}
	s.notifyChanged()
}

func (s *DragSource) notifyChanged() {
	if s.owner != nil {
		s.owner.base().requestSemanticUpdate()
		notifyDragControllersChanged(s.owner.Root())
	}
}

// DropTarget is a Bubble-phase native offer target. DragEnter/Motion/Drop are
// delivered by the dispatcher's hit-path coordinator, not by raw pointer
// events, so ordinary widgets and external applications use one target API.
type DropTarget struct {
	owner   Widget
	enabled bool
	actions DragAction
	formats []DragFormat
	active  bool
	enter   signal.Signal1[*DragMotion]
	motion  signal.Signal1[*DragMotion]
	leave   signal.Signal0
	drop    signal.Signal1[*DropRequest]
	error   signal.Signal1[error]
}

func NewDropTarget(formats ...DragFormat) *DropTarget {
	t := &DropTarget{enabled: true, actions: DragCopy}
	t.SetFormats(formats...)
	return t
}

func (t *DropTarget) Phase() PropagationPhase        { return PhaseBubble }
func (t *DropTarget) Enabled() bool                  { return t.enabled }
func (t *DropTarget) Actions() DragAction            { return t.actions }
func (t *DropTarget) Formats() []DragFormat          { return slices.Clone(t.formats) }
func (t *DropTarget) HandleEvent(EventContext)       {}
func (t *DropTarget) HandleCrossing(CrossingContext) {}

func (t *DropTarget) Reset() {
	if t.active {
		t.active = false
		if t.owner != nil && !t.owner.base().destroyed {
			t.owner.base().requestSemanticUpdate()
		}
		t.leave.Emit()
	}
}

func (t *DropTarget) SetEnabled(value bool) {
	if t.enabled == value {
		return
	}
	t.enabled = value
	if !value {
		t.Reset()
	}
	t.notifyChanged()
}

func (t *DropTarget) SetActions(value DragAction) {
	if t.actions != value {
		t.actions = value
		t.notifyChanged()
	}
}

func (t *DropTarget) SetFormats(formats ...DragFormat) {
	unique := make([]DragFormat, 0, len(formats))
	for _, format := range formats {
		if !slices.Contains(unique, format) {
			unique = append(unique, format)
		}
	}
	if !slices.Equal(t.formats, unique) {
		t.formats = unique
		t.notifyChanged()
	}
}

func (t *DropTarget) ConnectEnter(fn func(*DragMotion)) signal.Handle  { return t.enter.Connect(fn) }
func (t *DropTarget) ConnectMotion(fn func(*DragMotion)) signal.Handle { return t.motion.Connect(fn) }
func (t *DropTarget) ConnectLeave(fn func()) signal.Handle             { return t.leave.Connect(fn) }
func (t *DropTarget) ConnectDrop(fn func(*DropRequest)) signal.Handle  { return t.drop.Connect(fn) }
func (t *DropTarget) ConnectError(fn func(error)) signal.Handle        { return t.error.Connect(fn) }

func (t *DropTarget) setWidget(w Widget) {
	if w == nil {
		t.Reset()
	}
	t.owner = w
	t.notifyChanged()
}

func (t *DropTarget) notifyChanged() {
	if t.owner != nil {
		t.owner.base().requestSemanticUpdate()
		notifyDragControllersChanged(t.owner.Root())
	}
}

func validDragFormat(format DragFormat) bool {
	switch format {
	case DragFormatText, DragFormatFiles, DragFormatURLs:
		return true
	}
	if strings.HasPrefix(string(format), "local:") {
		return LocalFormat(strings.TrimPrefix(string(format), "local:")) == format
	}
	if strings.HasPrefix(string(format), "mime:") {
		return MIMEFormat(strings.TrimPrefix(string(format), "mime:")) == format
	}
	return false
}
