package widgets

import (
	"fmt"
	"math"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/graphics"
	"github.com/golang-gui/goui/style"
)

// DropDownItem describes an option. Business values belong to the application;
// the selected index refers to the current model order, not object identity.
type DropDownItem struct {
	// Text is the default Label source. With a custom delegate, it is the
	// fallback semantic name, used verbatim if the content reports no Text.
	Text     string
	Disabled bool
}

const DropDownInfoKey = "goui.dropdown"

// DropDownInfo describes committed selection or one realized option. Current
// is popup browsing, not selection or keyboard focus. Index is zero-based.
type DropDownInfo struct {
	Count    int  `json:"count"`
	Index    int  `json:"index"`
	Expanded bool `json:"expanded,omitempty"`
	Current  bool `json:"current,omitempty"`
}

// DropDown is a non-editable single-choice control. SetSelected is silent;
// Selected notifies completed user changes. All operations run on the GUI thread.
// Publish model changes on that thread, even when its storage is concurrency-safe.
type DropDown struct {
	gui.WidgetBase
	model                      gui.ListData[DropDownItem]
	modelHandle                signal.Handle
	delegate, selectedDelegate gui.ListItemDelegate
	displayDelegate            gui.ListItemDelegate
	display                    gui.Widget
	placeholderLabel           *gui.Label
	placeholder                string
	selected, current, bound   int
	padding                    geometry.Insets
	popupMaxHeight             float32
	revision                   uint64
	selectedSignal             signal.Signal2[int, uint64]
	openError                  signal.Signal2[error, uint64]
	popup                      gui.Popover
	content                    *dropDownContent
	motion                     *gui.MotionEventController
	click                      *gui.ClickEventController
	keys                       *gui.KeyEventController
}

// NewDropDown creates an enabled, focusable control with no selection, 6 DIP
// padding on every edge and a 320 DIP maximum popup body height. Native resources
// are lazy.
func NewDropDown() *DropDown {
	d := &DropDown{selected: -1, current: -1, bound: -1, padding: geometry.UniformInsets(6), popupMaxHeight: 320}
	d.WidgetBase.SetFocusable(true)
	d.SetLayoutManager(dropDownLayout{d})
	d.placeholderLabel = gui.NewLabel("")
	d.placeholderLabel.SetStyleName("drop-down-placeholder")
	d.WidgetBase.AddChild(d, d.placeholderLabel)
	d.motion = gui.NewMotionEventController()
	d.click = gui.NewClickEventController()
	d.click.ConnectPressed(func(_ gui.EventContext, _ bool) { d.syncPressed() })
	d.click.ConnectClicked(func(ctx gui.EventContext) {
		ctx.StopPropagation()
		d.openFromInput()
	})
	d.keys = gui.NewKeyEventController()
	d.keys.ConnectKeyDown(d.keyDown)
	d.addControllers()
	d.ConnectFocused(func(focused bool) {
		if !focused {
			d.Close()
		}
		d.RequestPaint()
	})
	d.ConnectMount(func() { d.connectModel(); d.Refresh() })
	d.ConnectUnmount(func() {
		d.revision++
		d.click.Reset()
		if d.modelHandle != nil {
			d.modelHandle.Disconnect()
			d.modelHandle = nil
		}
		if d.popup != nil {
			d.popup.Destroy()
			d.popup = nil
		}
		if d.content != nil {
			d.content.list.SetModel(nil)
		}
		d.unbindDisplay()
		d.current = -1
		d.SetState(style.Pressed, false)
	})
	return d
}

func (d *DropDown) Model() gui.ListData[DropDownItem] { return d.model }

// SetModel always reinstalls the model and clears selection, even for the same
// instance. Use Items notifications for data changes, Refresh for presentation.
// An Items notification retains a valid position, not a particular business value.
func (d *DropDown) SetModel(model gui.ListData[DropDownItem]) {
	if d.Destroyed() {
		return
	}
	d.revision++
	revision := d.revision
	d.Close()
	if d.modelHandle != nil {
		d.modelHandle.Disconnect()
		d.modelHandle = nil
	}
	d.unbindDisplay()
	if d.Destroyed() || d.revision != revision {
		return
	}
	d.model, d.selected = model, -1
	d.connectModel()
	if d.content != nil {
		d.content.list.SetModel(model)
	}
	d.refreshDisplay()
}

func (d *DropDown) connectModel() {
	if d.model != nil && d.modelHandle == nil && !d.Destroyed() {
		d.modelHandle = d.model.ConnectItems(d.modelChanged)
	}
}

func (d *DropDown) count() int {
	if d.model == nil {
		return 0
	}
	return max(0, d.model.ItemsCount())
}

func (d *DropDown) modelChanged() {
	if d.Destroyed() {
		return
	}
	d.revision++
	revision := d.revision
	// A press and its index cannot survive a structural model notification.
	d.Close()
	d.unbindDisplay()
	if d.Destroyed() || d.revision != revision {
		return
	}
	if d.selected >= d.count() {
		d.selected = -1
	}
	d.refreshDisplay()
	if d.content != nil {
		d.content.list.Refresh()
		d.content.RequestLayout()
	}
}

func (d *DropDown) Selected() int { return d.selected }

// SetSelected silently synchronizes a zero-based position; -1 or an invalid
// position clears selection. Programmatic selection may include disabled items.
// An unchanged position does not interrupt popup browsing or rebuild content.
func (d *DropDown) SetSelected(index int) {
	if d.Destroyed() {
		return
	}
	if index < 0 || index >= d.count() {
		index = -1
	}
	if d.selected == index {
		return
	}
	d.revision++
	revision := d.revision
	d.Close()
	d.unbindDisplay()
	if d.Destroyed() || d.revision != revision {
		return
	}
	d.selected = index
	d.refreshDisplay()
}

// ConnectSelected reports only actual, user-confirmed changes. Setters, model
// synchronization, popup browsing and confirming the same item do not notify.
func (d *DropDown) ConnectSelected(fn func(int)) signal.Handle {
	return d.selectedSignal.Connect(func(index int, revision uint64) {
		if !d.Destroyed() && d.revision == revision && d.selected == index {
			fn(index)
		}
	})
}

func (d *DropDown) Placeholder() string { return d.placeholder }
func (d *DropDown) SetPlaceholder(text string) {
	if !d.Destroyed() && d.placeholder != text {
		d.placeholder = text
		d.placeholderLabel.SetText(text)
		d.RequestLayout()
	}
}

func (d *DropDown) SetVisible(visible bool) {
	if !visible {
		d.Close()
		d.click.Reset()
		d.SetState(style.Pressed, false)
	}
	d.WidgetBase.SetVisible(visible)
}
func (d *DropDown) addControllers() {
	d.AddEventController(d.motion)
	d.AddEventController(d.click)
	d.AddEventController(d.keys)
}

func dropDownExtent(value float32) float32 {
	if value < 0 || math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
		return 0
	}
	return value
}

func (d *DropDown) Padding() geometry.Insets { return d.padding }

// SetPadding sets the per-edge inner padding in DIP; invalid edges become zero.
func (d *DropDown) SetPadding(padding geometry.Insets) {
	padding = normalizeInsets(padding)
	if !d.Destroyed() && padding != d.padding {
		d.padding = padding
		d.RequestLayout()
	}
}
func (d *DropDown) PopupMaxHeight() float32 { return d.popupMaxHeight }

// SetPopupMaxHeight limits the body, excluding shadow. Zero removes this limit;
// the Popover still respects the available desktop work area.
func (d *DropDown) SetPopupMaxHeight(value float32) {
	value = dropDownExtent(value)
	if !d.Destroyed() && value != d.popupMaxHeight {
		d.popupMaxHeight = value
		if d.content != nil {
			d.content.RequestLayout()
		}
	}
}

func (d *DropDown) Delegate() gui.ListItemDelegate         { return d.delegate }
func (d *DropDown) SelectedDelegate() gui.ListItemDelegate { return d.selectedDelegate }

// SetDelegate reinstalls content even for the same delegate. The popup and
// collapsed display get separate Setup results. Nil restores default Labels.
func (d *DropDown) SetDelegate(delegate gui.ListItemDelegate) {
	if d.Destroyed() {
		return
	}
	d.revision++
	revision := d.revision
	d.Close()
	if d.selectedDelegate == nil {
		d.releaseDisplay()
	}
	if d.Destroyed() || d.revision != revision {
		return
	}
	d.delegate = delegate
	if d.content != nil {
		d.content.list.SetDelegate(&dropDownListDelegate{owner: d, delegate: d.rowDelegate()})
	}
	d.refreshDisplay()
}

// SetSelectedDelegate overrides only the collapsed display. Nil uses Delegate
// (or the default Label). Returned widgets must be fresh and detached.
func (d *DropDown) SetSelectedDelegate(delegate gui.ListItemDelegate) {
	if d.Destroyed() {
		return
	}
	d.revision++
	revision := d.revision
	d.Close()
	d.releaseDisplay()
	if d.Destroyed() || d.revision != revision {
		return
	}
	d.selectedDelegate = delegate
	d.refreshDisplay()
}

func (d *DropDown) rowDelegate() gui.ListItemDelegate {
	if d.delegate != nil {
		return d.delegate
	}
	return &dropDownLabelDelegate{owner: d}
}

func (d *DropDown) unbindDisplay() {
	index, content, delegate := d.bound, d.display, d.displayDelegate
	d.bound = -1
	if index >= 0 && delegate != nil && content != nil {
		delegate.Unbind(index, content)
	}
}
func (d *DropDown) releaseDisplay() {
	content, delegate, index := d.display, d.displayDelegate, d.bound
	d.display, d.displayDelegate, d.bound = nil, nil, -1
	if index >= 0 && delegate != nil && content != nil {
		delegate.Unbind(index, content)
	}
	if content != nil {
		d.RemoveChild(content)
	}
}

func (d *DropDown) refreshDisplay() {
	if d.Destroyed() {
		return
	}
	d.placeholderLabel.SetVisible(d.selected < 0)
	if d.selected < 0 {
		if d.display != nil {
			d.display.SetVisible(false)
		}
		d.RequestLayout()
		return
	}
	revision := d.revision
	if d.display == nil {
		delegate := d.selectedDelegate
		if delegate == nil {
			delegate = d.delegate
		}
		if delegate == nil {
			delegate = &dropDownLabelDelegate{owner: d, selected: true}
		}
		content := delegate.Setup()
		if d.Destroyed() || d.revision != revision {
			return
		}
		if content == nil {
			return
		}
		d.display, d.displayDelegate = content, delegate
		d.WidgetBase.AddChild(d, content)
		if content.Parent() != d {
			d.display, d.displayDelegate = nil, nil
			return
		}
	}
	index, content := d.selected, d.display
	d.bound = index
	d.displayDelegate.Bind(index, content)
	if d.Destroyed() || d.revision != revision || d.display != content {
		return
	}
	content.SetVisible(true)
	d.RequestLayout()
	content.RequestLayout()
}

// Refresh rebinds presentation without reinstalling the model or canceling
// popup browsing. Style changes use each child's independent StyleChanged hook.
func (d *DropDown) Refresh() {
	d.refreshDisplay()
	if d.content != nil {
		d.content.list.Refresh()
		d.content.RequestLayout()
	}
	d.RequestPaint()
}

func (d *DropDown) Opened() bool { return d.popup != nil && d.popup.Visible() }

// Open lazily creates an owner-modal, no-native-focus Popover. Empty or fully
// disabled lists and disabled controls are a no-op; an unmounted anchor fails.
func (d *DropDown) Open() error {
	if d.Destroyed() {
		return fmt.Errorf("drop-down: destroyed")
	}
	if !gui.IsEnabled(d) || !d.Visible() || d.Opened() {
		return nil
	}
	index := d.selected
	if !d.available(index) {
		index = d.neighbor(-1, 1)
	}
	if index < 0 {
		return nil
	}
	if d.Rect().Width <= 0 {
		return fmt.Errorf("drop-down: anchor has no layout")
	}
	if d.content == nil {
		d.content = newDropDownContent(d)
	}
	if d.popup == nil {
		d.popup = gui.NewPopover(d, &gui.PopoverOptions{Transparent: true})
		d.popup.SetStyleName("drop-down-popup")
		d.popup.SetModal(true)
		d.popup.SetAutoDismiss(true)
		d.popup.ConnectDismissRequest(d.Close)
		d.popup.ConnectClosed(func() { d.current = -1; d.SetState(style.Pressed, false); d.RequestPaint() })
		d.popup.SetWidget(d.content)
	}
	d.popup.SetPlacement(gui.PopoverPlacementBottom)
	d.current = index
	d.content.list.Refresh()
	d.content.RequestLayout()
	revision := d.revision
	if err := d.popup.Show(); err != nil {
		d.Close()
		return err
	}
	if d.Destroyed() || revision != d.revision || !d.Opened() {
		return fmt.Errorf("drop-down: released while opening")
	}
	if host, ok := d.popup.(gui.EventTarget); ok {
		host.SetFocusedWidget(d.content)
	}
	if win := d.Window(); win != nil {
		win.SetFocusedWidget(d)
	}
	if d.Destroyed() || revision != d.revision || !d.Opened() {
		return fmt.Errorf("drop-down: released during focus")
	}
	d.content.list.Reveal(index)
	d.syncPressed()
	d.RequestPaint()
	return nil
}

func (d *DropDown) Close() {
	if d.popup != nil {
		d.popup.Hide()
	}
	d.current = -1
	d.SetState(style.Pressed, false)
	if d.content != nil {
		for _, widget := range d.content.list.Children() {
			r := widget.(*dropDownRow)
			r.SetState(style.Pressed, false)
		}
	}
	d.RequestPaint()
}

// ConnectOpenError reports only failures triggered by user input. Open itself
// returns its error without also notifying this signal.
func (d *DropDown) ConnectOpenError(fn func(error)) signal.Handle {
	return d.openError.Connect(func(err error, revision uint64) {
		if !d.Destroyed() && d.revision == revision {
			fn(err)
		}
	})
}
func (d *DropDown) openFromInput() {
	if err := d.Open(); err != nil && !d.Destroyed() {
		d.openError.Emit(err, d.revision)
	}
}

func (d *DropDown) available(index int) bool {
	return index >= 0 && index < d.count() && !d.model.ItemAt(index).Disabled
}
func (d *DropDown) neighbor(from, direction int) int {
	for index := from + direction; index >= 0 && index < d.count(); index += direction {
		if d.available(index) {
			return index
		}
	}
	return -1
}
func (d *DropDown) choose(index int) {
	if d.Destroyed() || !gui.IsEnabled(d) || !d.available(index) {
		return
	}
	changed := d.selected != index
	d.SetSelected(index)
	d.Close()
	if changed && !d.Destroyed() && d.selected == index {
		d.selectedSignal.Emit(index, d.revision)
	}
}

func (d *DropDown) keyDown(ctx gui.EventContext, event events.KeyEvent) {
	if !gui.IsEnabled(d) {
		return
	}
	plain := func(key gui.Key) bool { return (gui.KeyGesture{Key: key}).Matches(event) }
	if plain(gui.KeyEnter) || plain(gui.KeySpace) || (gui.KeyGesture{Key: gui.KeyArrowDown, Modifiers: gui.ModAlt}).Matches(event) {
		if !event.Repeat {
			d.openFromInput()
		}
	} else if plain(gui.KeyArrowDown) || plain(gui.KeyArrowUp) {
		direction := 1
		if event.Key == events.KeyArrowUp {
			direction = -1
		}
		from := d.selected
		if from < 0 && direction < 0 {
			from = d.count()
		}
		if index := d.neighbor(from, direction); index >= 0 {
			d.choose(index)
		}
	} else {
		return
	}
	ctx.StopPropagation()
	event.PreventDefault()
}

func (d *DropDown) Paint(p gui.Painter) {
	name := d.StyleName()
	if name == "" {
		name = "drop-down"
	}
	state := d.State()
	rect := geometry.Rectangle{Size: d.Rect().Size}
	paintStyledBox(d, p, rect, name, style.PartDefault, state)
	s := gui.ResolveWidgetStyle(d, name, "arrow", state)
	color, ok := s.ForegroundColor()
	if ok && color != nil {
		x := max(float32(0), rect.Width-d.padding.Right-8)
		y := d.padding.Top + (rect.Height-d.padding.Vertical())/2
		c := graphics.ColorOf(color)
		p.DrawLine(geometry.Point{X: x - 3, Y: y - 1.5}, geometry.Point{X: x, Y: y + 1.5}, 1.5, c)
		p.DrawLine(geometry.Point{X: x, Y: y + 1.5}, geometry.Point{X: x + 3, Y: y - 1.5}, 1.5, c)
	}
	if d.FocusVisible() {
		paintStyledBorder(p, rect, gui.ResolveWidgetStyle(d, name, "focus", d.State()))
	}
}

func (d *DropDown) Snapshot() gui.WidgetInfo {
	info := d.WidgetBase.Snapshot()
	info.Role = RoleComboBox
	if d.selected >= 0 && d.selected < d.count() {
		info.Text = dropDownDisplayText(d.display, d.model.ItemAt(d.selected).Text)
	} else {
		info.Text = d.placeholderLabel.PlainText()
	}
	if info.Enabled {
		info.Actions = append(info.Actions, gui.ActionClick)
	}
	info.SetAttribute(DropDownInfoKey, DropDownInfo{Count: d.count(), Index: d.selected, Expanded: d.Opened()})
	return info
}

// dropDownDisplayText uses the content's own text semantics. The built-in Label
// already has parsed text; custom content may provide a different semantic name.
// Missing custom text falls back to the model without interpreting its syntax.
func dropDownDisplayText(content gui.Widget, fallback string) string {
	if label, ok := content.(*gui.Label); ok {
		return label.PlainText()
	}
	if content != nil {
		if text := content.Snapshot().Text; text != "" {
			return text
		}
	}
	return fallback
}

type dropDownLabelDelegate struct {
	owner    *DropDown
	selected bool
}

func (d *dropDownLabelDelegate) Setup() gui.Widget { return gui.NewLabel("") }
func (d *dropDownLabelDelegate) Bind(index int, widget gui.Widget) {
	label := widget.(*gui.Label)
	item := d.owner.model.ItemAt(index)
	name := "drop-down-item-text"
	disabled := item.Disabled
	if d.selected {
		name, disabled = "drop-down-text", false
	}
	if disabled {
		name += "-disabled"
	}
	label.SetStyleName(name)
	label.SetText(item.Text)
}
func (*dropDownLabelDelegate) Unbind(int, gui.Widget) {}

type dropDownLayout struct{ owner *DropDown }

func (l dropDownLayout) Measure(children []layout.Child, c layout.Constraint) layout.Measurement {
	inner := c.Insets(l.owner.padding)
	var content layout.Measurement
	if len(children) > 0 {
		content = children[0].Measure(layout.Loose(geometry.Size{Width: max(0, inner.Max.Width-24), Height: inner.Max.Height}))
	}
	size := c.Clamp(geometry.Size{Width: content.Width + 24 + l.owner.padding.Horizontal(), Height: max(float32(16), content.Height) + l.owner.padding.Vertical()})
	content.Baseline += l.owner.padding.Top + (size.Height-l.owner.padding.Vertical()-content.Height)/2
	content.Size = size
	return content
}
func (l dropDownLayout) Arrange(children []layout.Child, rect geometry.Rectangle) {
	if len(children) == 0 {
		return
	}
	inner := rect.Insets(l.owner.padding.Left, l.owner.padding.Top, l.owner.padding.Right, l.owner.padding.Bottom)
	area := geometry.Size{Width: max(0, inner.Width-24), Height: inner.Height}
	content := children[0].Measure(layout.Loose(area))
	children[0].Arrange(geometry.Rect(inner.X, inner.Y+(inner.Height-content.Height)/2, area.Width, content.Height))
}

func (d *DropDown) syncPressed() { d.SetState(style.Pressed, d.click.Pressed() || d.Opened()) }
