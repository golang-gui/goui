package ui

import (
	"image"
	"testing"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/typography"
)

func TestRootCreatesAndUpdatesLabel(t *testing.T) {
	root := newRoot()

	widget := root.update(Label("hello").Name("title"))
	label, ok := widget.(*gui.Label)
	if !ok {
		t.Fatalf("updated %T, want *gui.Label", widget)
	}
	if label.ID() != "title" || label.Text() != "hello" {
		t.Fatalf("unexpected label state: id=%q text=%q", label.ID(), label.Text())
	}

	updated := root.update(Label("world").Name("title2"))
	if updated != label {
		t.Fatal("label at the same root slot and type should be reused")
	}
	if label.ID() != "title2" || label.Text() != "world" {
		t.Fatalf("label was not updated: id=%q text=%q", label.ID(), label.Text())
	}
}

func TestRootReplacesDifferentViewType(t *testing.T) {
	root := newRoot()

	label := root.update(Label("name").Name("field"))
	input := root.update(TextInput().Name("field").Text("name"))

	if input == label {
		t.Fatal("different view types should not reuse the same widget")
	}
	textInput, ok := input.(*gui.TextInput)
	if !ok {
		t.Fatalf("updated %T, want *gui.TextInput", input)
	}
	if textInput.Text() != "name" {
		t.Fatalf("unexpected text input text: %q", textInput.Text())
	}
}

func TestRootExpandsCompositionViewBeforeDiff(t *testing.T) {
	root := newRoot()
	builds := 0

	widget := root.update(testCompositionView{
		view:   Label("first"),
		builds: &builds,
	})
	label := widget.(*gui.Label)
	if label.Text() != "first" || builds != 1 {
		t.Fatalf("unexpected initial composition state: text=%q builds=%d", label.Text(), builds)
	}

	updated := root.update(testCompositionView{
		view:   Label("second"),
		builds: &builds,
	})
	if updated != label {
		t.Fatal("composition view should reuse the same expanded widget type")
	}
	if label.Text() != "second" || builds != 2 {
		t.Fatalf("unexpected updated composition state: text=%q builds=%d", label.Text(), builds)
	}

	replaced := root.update(testCompositionView{
		view:   TextInput().Text("third"),
		builds: &builds,
	})
	if replaced == label {
		t.Fatal("composition view should replace widget when the expanded type changes")
	}
	input := replaced.(*gui.TextInput)
	if input.Text() != "third" || builds != 3 {
		t.Fatalf("unexpected replaced composition state: text=%q builds=%d", input.Text(), builds)
	}
}

func TestRootUpdatesBoxChildrenByPositionAndType(t *testing.T) {
	root := newRoot()

	widget := root.update(VBox().
		Name("root").
		Spacing(6).
		Children(
			Label("one").Name("first"),
			Label("two").Name("second"),
		))
	box := widget.(*gui.LinearBox)
	if box.ID() != "root" || box.Direction() != layout.DirectionVertical || box.Spacing() != 6 {
		t.Fatalf("unexpected box state: id=%q direction=%v spacing=%v", box.ID(), box.Direction(), box.Spacing())
	}
	children := box.Children()
	if len(children) != 2 {
		t.Fatalf("expected two children, got %d", len(children))
	}
	first := children[0].(*gui.Label)
	second := children[1].(*gui.Label)

	root.update(VBox().
		Spacing(8).
		Children(
			Label("ONE").Name("first-updated"),
			Label("TWO").Name("second-updated"),
			Label("THREE").Name("third"),
		))

	children = box.Children()
	if len(children) != 3 {
		t.Fatalf("expected three children, got %d", len(children))
	}
	if children[0] != first || children[1] != second {
		t.Fatal("children at the same slot and type should be reused")
	}
	if first.Text() != "ONE" || second.Text() != "TWO" {
		t.Fatalf("children were not updated: %q %q", first.Text(), second.Text())
	}
	if children[2].(*gui.Label).Text() != "THREE" {
		t.Fatalf("tail child was not appended: %q", children[2].(*gui.Label).Text())
	}
	if box.Spacing() != 8 {
		t.Fatalf("box spacing was not updated: %v", box.Spacing())
	}
}

func TestRootRebuildsTailOnChildTypeMismatch(t *testing.T) {
	root := newRoot()

	box := root.update(VBox(
		Label("first"),
		TextInput().Text("second"),
	)).(*gui.LinearBox)
	oldChildren := box.Children()

	root.update(VBox(
		Label("FIRST"),
		Label("inserted"),
		TextInput().Text("second"),
	))

	children := box.Children()
	if len(children) != 3 {
		t.Fatalf("expected three children, got %d", len(children))
	}
	if children[0] != oldChildren[0] {
		t.Fatal("same type prefix child should be reused")
	}
	if children[1] == oldChildren[1] {
		t.Fatal("tail should be rebuilt after the first child type mismatch")
	}
	if children[0].(*gui.Label).Text() != "FIRST" || children[1].(*gui.Label).Text() != "inserted" {
		t.Fatalf("unexpected rebuilt children: %v", children)
	}
}

func TestRootUpdatesButtonChild(t *testing.T) {
	root := newRoot()

	button := root.update(Button().Content(Label("OK"))).(*gui.Button)
	children := button.Children()
	if len(children) != 1 || children[0].(*gui.Label).Text() != "OK" {
		t.Fatalf("unexpected button child: %v", children)
	}
	child := children[0]

	root.update(Button("Cancel"))
	children = button.Children()
	if len(children) != 1 || children[0] != child || children[0].(*gui.Label).Text() != "Cancel" {
		t.Fatalf("button child was not updated in place: %v", children)
	}

	root.update(Button())
	if children := button.Children(); len(children) != 0 {
		t.Fatalf("button child was not removed: %v", children)
	}
}

func TestButtonViewUpdatesClickHandlerThroughState(t *testing.T) {
	root := newRoot()
	firstCalls := 0
	secondCalls := 0

	button := root.update(Button().OnClick(func() {
		firstCalls++
	})).(*gui.Button)
	triggerButtonClick(button)
	if firstCalls != 1 || secondCalls != 0 {
		t.Fatalf("unexpected first click calls: first=%d second=%d", firstCalls, secondCalls)
	}

	root.update(Button().OnClick(func() {
		secondCalls++
	}))
	triggerButtonClick(button)
	if firstCalls != 1 || secondCalls != 1 {
		t.Fatalf("handler was not updated through state: first=%d second=%d", firstCalls, secondCalls)
	}

	root.unmountWindow()
	triggerButtonClick(button)
	if firstCalls != 1 || secondCalls != 1 {
		t.Fatalf("unmounted click handler should be disconnected: first=%d second=%d", firstCalls, secondCalls)
	}
}

func TestRootUpdatesTextInputHandlerThroughState(t *testing.T) {
	root := newRoot()
	firstCalls := 0
	secondCalls := 0

	widget := root.update(TextInput().
		Text("a").
		OnText(func(string) {
			firstCalls++
		}))
	input := widget.(*gui.TextInput)

	input.SetText("b")
	if firstCalls != 1 || secondCalls != 0 {
		t.Fatalf("unexpected first handler calls: first=%d second=%d", firstCalls, secondCalls)
	}

	root.update(TextInput().
		Text("c").
		OnText(func(string) {
			secondCalls++
		}))
	if firstCalls != 1 || secondCalls != 0 {
		t.Fatalf("declarative text update should not emit handlers: first=%d second=%d", firstCalls, secondCalls)
	}

	input.SetText("d")
	if firstCalls != 1 || secondCalls != 1 {
		t.Fatalf("handler was not replaced: first=%d second=%d", firstCalls, secondCalls)
	}
}

func TestRootClearsConnectionWhenHandlerRemoved(t *testing.T) {
	root := newRoot()
	calls := 0

	input := root.update(TextInput().
		OnText(func(string) {
			calls++
		})).(*gui.TextInput)
	input.SetText("a")
	if calls != 1 {
		t.Fatalf("expected first handler call, got %d", calls)
	}

	root.update(TextInput())
	input.SetText("b")
	if calls != 1 {
		t.Fatalf("removed handler should not be called, got %d", calls)
	}
}

func TestRootUpdatesImageAndCommonFields(t *testing.T) {
	root := newRoot()
	first := image.NewRGBA(image.Rect(0, 0, 10, 10))
	second := image.NewRGBA(image.Rect(0, 0, 20, 20))

	widget := root.update(Image(first).Name("logo"))
	imageWidget := widget.(*gui.Image)
	if imageWidget.ID() != "logo" || imageWidget.Image() != first || !imageWidget.Visible() {
		t.Fatalf("unexpected image state: id=%q visible=%v", imageWidget.ID(), imageWidget.Visible())
	}

	updated := root.update(Image(second).Name("logo2").Hidden(true))
	if updated != imageWidget {
		t.Fatal("image at the same root slot and type should be reused")
	}
	if imageWidget.ID() != "logo2" || imageWidget.Image() != second || imageWidget.Visible() {
		t.Fatalf("image fields were not updated: id=%q visible=%v", imageWidget.ID(), imageWidget.Visible())
	}
}

func TestRootPreservesViewStateAcrossUpdatesAndUnmounts(t *testing.T) {
	root := newRoot()
	tracker := &lifecycleTracker{}

	widget := root.update(lifecycleView{text: "first", tracker: tracker})
	label := widget.(*gui.Label)
	updated := root.update(lifecycleView{text: "second", tracker: tracker})

	if updated != label {
		t.Fatal("same view type should reuse the mounted widget")
	}
	if tracker.mounts != 1 || tracker.updates != 2 || tracker.unmounts != 0 {
		t.Fatalf("unexpected lifecycle before unmount: mounts=%d updates=%d unmounts=%d", tracker.mounts, tracker.updates, tracker.unmounts)
	}
	if label.Text() != "second" {
		t.Fatalf("view update did not update widget text: %q", label.Text())
	}

	root.update(Label("replacement"))

	if tracker.unmounts != 1 {
		t.Fatalf("expected one unmount, got %d", tracker.unmounts)
	}
	if tracker.unmountedStateUpdates != 2 || tracker.unmountedText != "second" {
		t.Fatalf("unmount did not see preserved state/widget: state updates=%d text=%q", tracker.unmountedStateUpdates, tracker.unmountedText)
	}
}

func TestTextInputViewDisconnectsHandlerOnUnmount(t *testing.T) {
	root := newRoot()
	calls := 0

	input := root.update(TextInput().OnText(func(string) {
		calls++
	})).(*gui.TextInput)

	root.unmountWindow()
	input.SetText("after-unmount")

	if calls != 0 {
		t.Fatalf("unmounted text handler should be disconnected, got %d calls", calls)
	}
}

func TestRootUnmountClearsWidget(t *testing.T) {
	root := newRoot()
	root.update(VBox(Label("child")))
	if root.widget() == nil {
		t.Fatal("expected rendered widget")
	}

	root.unmountWindow()
	if root.widget() != nil {
		t.Fatal("unmount should clear root widget")
	}
}

type testEventContext struct {
	event   events.Event
	stopped bool
}

func (c *testEventContext) Event() events.Event {
	return c.event
}

func (c *testEventContext) Position() (geometry.Point, bool) {
	return geometry.Point{}, false
}

func (c *testEventContext) StopPropagation() {
	c.stopped = true
}

func (c *testEventContext) PropagationStopped() bool {
	return c.stopped
}

func triggerButtonClick(button *gui.Button) {
	for _, controller := range button.EventControllers() {
		click, ok := controller.(*gui.ClickEventController)
		if !ok {
			continue
		}
		click.HandleEvent(&testEventContext{event: events.PointerEvent{
			EventType: events.PointerDown,
			Button:    events.PointerButtonLeft,
			Buttons:   events.PointerButtonLeftDown,
		}})
		click.HandleEvent(&testEventContext{event: events.PointerEvent{
			EventType: events.PointerUp,
			Button:    events.PointerButtonLeft,
		}})
		return
	}
}

type lifecycleTracker struct {
	mounts                int
	updates               int
	unmounts              int
	unmountedStateUpdates int
	unmountedText         string
}

type lifecycleState struct {
	updates int
}

type lifecycleView struct {
	text    string
	tracker *lifecycleTracker
}

type testCompositionView struct {
	view   View
	builds *int
}

func (v testCompositionView) Build() View {
	if v.builds != nil {
		*v.builds = *v.builds + 1
	}
	return v.view
}

func (v lifecycleView) Build() View {
	return v
}

func (v lifecycleView) Mount(ctx BuildContext) gui.Widget {
	v.tracker.mounts++
	ctx.SetState(&lifecycleState{})
	return gui.NewLabel("")
}

func (v lifecycleView) Update(ctx BuildContext, widget gui.Widget) {
	v.tracker.updates++
	state := ctx.State().(*lifecycleState)
	state.updates++
	widget.(*gui.Label).SetText(v.text)
}

func (v lifecycleView) Unmount(ctx BuildContext, widget gui.Widget) {
	v.tracker.unmounts++
	state := ctx.State().(*lifecycleState)
	v.tracker.unmountedStateUpdates = state.updates
	v.tracker.unmountedText = widget.(*gui.Label).Text()
}

func useTestApplication(t *testing.T, app gui.Application) {
	t.Helper()

	old := gui.App
	gui.App = app
	t.Cleanup(func() {
		gui.App = old
	})
}

type testApplication struct {
	posts []func()
}

func newTestApplication() *testApplication {
	return new(testApplication)
}

func (a *testApplication) Platform() platform.Platform {
	return nil
}

func (a *testApplication) Typography() typography.Context {
	return nil
}

func (a *testApplication) Clipboard() platform.Clipboard {
	return nil
}

func (a *testApplication) NewWindow() (gui.Window, error) {
	return nil, nil
}

func (a *testApplication) Run() {}

func (a *testApplication) Quit() {}

func (a *testApplication) Post(task func()) {
	a.posts = append(a.posts, task)
}

func (a *testApplication) Windows() []gui.Window {
	return nil
}

func (a *testApplication) Snapshot() gui.ApplicationInfo {
	return gui.ApplicationInfo{}
}

func (a *testApplication) DispatchWindowEvent(string, events.Event) error {
	return nil
}

func (a *testApplication) runPosted() {
	posts := a.posts
	a.posts = nil
	for _, post := range posts {
		post()
	}
}

type testWindow struct {
	id            string
	title         string
	widget        gui.Widget
	shows         int
	destroyed     bool
	focused       bool
	focusedWidget gui.Widget
	closeRequest  signal.Signal1[*bool]
	destroy       signal.Signal0
	focusChanged  signal.Signal1[bool]
}

func newTestWindow() *testWindow {
	return new(testWindow)
}

func (w *testWindow) Widget() gui.Widget {
	return w.widget
}

func (w *testWindow) RequestPaint() error {
	return nil
}

func (w *testWindow) ID() string {
	return w.id
}

func (w *testWindow) SetID(id string) {
	w.id = id
}

func (w *testWindow) SetWidget(widget gui.Widget) {
	w.widget = widget
}

func (w *testWindow) Title() string {
	return w.title
}

func (w *testWindow) SetTitle(title string) error {
	w.title = title
	return nil
}

func (w *testWindow) Focused() bool {
	return w.focused
}

func (w *testWindow) FocusedWidget() gui.Widget {
	return w.focusedWidget
}

func (w *testWindow) SetFocusedWidget(widget gui.Widget) bool {
	w.focusedWidget = widget
	return true
}

func (w *testWindow) Show() error {
	w.shows++
	return nil
}

func (w *testWindow) RequestClose() error {
	allow := true
	w.closeRequest.Emit(&allow)
	if allow {
		w.Destroy()
	}
	return nil
}

func (w *testWindow) Destroy() {
	if w.destroyed {
		return
	}
	w.destroyed = true
	w.destroy.Emit()
}

func (w *testWindow) Snapshot() gui.WindowInfo {
	return gui.WindowInfo{}
}

func (w *testWindow) DispatchEvent(events.Event) error {
	return nil
}

func (w *testWindow) ConnectCloseRequest(fn func(*bool)) signal.Handle {
	return w.closeRequest.Connect(fn)
}

func (w *testWindow) ConnectDestroy(fn func()) signal.Handle {
	return w.destroy.Connect(fn)
}

func (w *testWindow) ConnectFocusChanged(fn func(bool)) signal.Handle {
	return w.focusChanged.Connect(fn)
}
