package ui

import (
	"strconv"
	"testing"

	"github.com/golang-gui/goui/core/state"
	"github.com/golang-gui/goui/gui"
)

func TestStatePeekAndReadWithoutBuild(t *testing.T) {
	var value State[int]
	if value.Peek() != 0 || Read[int](&value) != 0 {
		t.Fatal("零值 State 的普通读取不正确")
	}
	value.Set(4)
	if value.Peek() != 4 || Read[int](&value) != 4 || activeReads.Load() != nil {
		t.Fatal("构建范围外的读取不应创建收集器")
	}
	model := state.Make("model")
	if Read[string](&model) != "model" {
		t.Fatal("core/state 未能通过 Read 使用")
	}
}

func TestReadCoreStateTracksComposition(t *testing.T) {
	value := state.Make(1)
	builds := 0
	_, fake := bindingApp(t, func() RootView {
		return Window("main").Content(Build(func() View {
			builds++
			return Label(strconv.Itoa(Read(&value)))
		}))
	})
	value.Set(2)
	fake.runPosted()
	if builds != 2 || fake.windows[0].Widget().(*gui.Label).Text() != "2" {
		t.Fatal("core/state 通过 Read 没有建立局部依赖")
	}
}

func BenchmarkStateSnapshotRead(b *testing.B) {
	value := MakeState(1)
	for name, read := range map[string]func() int{"Get": value.Get, "Peek": value.Peek} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				readBenchmarkValue = read()
			}
		})
	}
}

func TestStateSetWithoutObserversDoesNotRequestUpdate(t *testing.T) {
	fake := newWindowTestApplication()
	value, builds := MakeState("initial"), 0
	rt := newApp(fake, func() RootView {
		builds++
		return Window("main").Content(Label(value.Get()))
	})
	t.Cleanup(func() { rt.stop(); rt.destroyAll() })
	if err := rt.rebuild(); err != nil {
		t.Fatal(err)
	}
	value.Set("changed")
	value.Update(func(string) string { return "changed again" })
	if len(fake.posts) != 0 || builds != 1 || fake.windows[0].Widget().(*gui.Label).Text() != "initial" {
		t.Fatal("未订阅的 State 不应自动更新字面快照")
	}
	rt.RequestUpdate()
	fake.runPosted()
	if builds != 2 || fake.windows[0].Widget().(*gui.Label).Text() != "changed again" {
		t.Fatal("显式全应用重建仍应读取当前快照")
	}
}

func TestZeroStateSetNotifiesWithoutRebuild(t *testing.T) {
	app := newWindowTestApplication()
	_ = newApp(app, func() RootView {
		return Window("main").Content(Label("main"))
	})

	var state State[int]
	if got := state.Get(); got != 0 {
		t.Fatalf("zero State value = %d, want 0", got)
	}

	changes := 0
	h := state.Connect(func() { changes++ })
	defer h.Disconnect()
	state.Set(1)
	if changes != 1 {
		t.Fatal("零值 State 未通知监听者")
	}

	if got := state.Get(); got != 1 {
		t.Fatalf("updated zero State value = %d, want 1", got)
	}
	if len(app.posts) != 0 {
		t.Fatalf("unobserved zero State must not post updates, got %d", len(app.posts))
	}
}

func TestStateSetUpdatesOnlyBoundWidget(t *testing.T) {
	app := newWindowTestApplication()
	title := MakeState("first")
	builds := 0
	rt := newApp(app, func() RootView {
		builds++
		return Window("main").Content(Label().BindText(&title))
	})

	if err := rt.rebuild(); err != nil {
		t.Fatal(err)
	}
	label := app.windows[0].Widget().(*gui.Label)
	if label.Text() != "first" || builds != 1 {
		t.Fatalf("unexpected initial state: text=%q builds=%d", label.Text(), builds)
	}

	title.Set("second")
	if label.Text() != "first" {
		t.Fatalf("state set should defer rebuild, got %q", label.Text())
	}

	app.runPosted()
	if label.Text() != "second" || builds != 1 {
		t.Fatalf("unexpected rebuilt state: text=%q builds=%d", label.Text(), builds)
	}
}

func TestStateSetWithoutActiveAppUpdatesValue(t *testing.T) {
	state := MakeState(1)

	state.Set(2)

	if got := state.Get(); got != 2 {
		t.Fatalf("state value = %d, want 2", got)
	}
}

func TestStateUpdate(t *testing.T) {
	state := MakeState(1)
	changes := 0
	h := state.Connect(func() { changes++ })

	state.Update(func(before int) int { return before + 10 })
	if got := state.Get(); got != 11 {
		t.Fatalf("state value = %d, want 11", got)
	}
	if changes != 1 {
		t.Fatalf("Update should emit exactly once, got %d", changes)
	}

	h.Disconnect()
}
