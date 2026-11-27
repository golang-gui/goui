// GOUI music: a project demo that favors a finished look over API coverage.
// Playback is simulated: there is no audio, and every song is fictional.
//
// Environment: a desktop session; Linux requires X11. Choose the backend and
// scale with GOUI_PLAT_PAINTER and GOUI_PLAT_SCALE.
// Run: go run ./example/music. It starts paused on the first song.
// Steps and expected results:
//  1. Click the round play button or press Space: the record spins, the
//     spectrum rises and the position advances; pause and they settle.
//  2. Click or drag on the position bar to seek; the track thickens and shows
//     its knob while the pointer is over it.
//  3. Click previous/next, press Left/Right, or click a song in Playlist: the
//     window's light, the record label and the spectrum fade to the new
//     album's colors. Previous restarts a song that has played over 3 s.
//     After a song ends the next one starts; the queue wraps around.
//  4. Click the heart to like the song for this session.
//
// Side effects: none. Reset: restart. Exit: close the window.
package main

import (
	"log"
	"time"

	"github.com/golang-gui/goui/animation"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/ui"
)

const (
	tintDuration   = 600 * time.Millisecond // album color crossfade
	energyDuration = 400 * time.Millisecond // spectrum rise and fall
	tickInterval   = 200 * time.Millisecond // clock labels and song ends
)

// music owns the simulated player and every piece of presentation state.
type music struct {
	app      ui.App
	icons    *icons
	player   *player
	started  time.Time
	tint     *animation.Animation[albumColors]
	energy   *animation.Animation[float32]
	spin     float32       // record angle in degrees
	revision ui.State[int] // advances when the song, play state or likes change
	seconds  ui.State[int] // whole seconds shown by the clock labels
	ticker   *ui.Timer
}

func newMusic() (*music, error) {
	icons, err := loadIcons()
	if err != nil {
		return nil, err
	}
	m := &music{icons: icons, player: newPlayer(), started: time.Now()}
	m.tint = animation.New(m.player.current().colors(), func(from, to albumColors, t float64) albumColors {
		return lerpColors(from, to, float32(t))
	})
	m.energy = animation.New(float32(0), animation.Float32)
	return m, nil
}

func (m *music) colors() albumColors { return m.tint.CurrentValue() }

// fraction is the played part of the current song at now, in [0, 1].
func (m *music) fraction(now time.Time) float64 {
	return float64(m.player.position(now)) / float64(m.player.current().Length)
}

// animating reports whether any drawn surface changes from frame to frame.
func (m *music) animating() bool {
	return m.player.playing || m.energy.CurrentValue() > 0 || m.tint.Running()
}

func (m *music) changed() {
	m.revision.Set(m.revision.Peek() + 1)
	m.syncClock()
}

func (m *music) syncClock() {
	if s := int(m.player.position(time.Now()) / time.Second); s != m.seconds.Peek() {
		m.seconds.Set(s)
	}
}

// animate starts a, or jumps to the target without a window.
func animate[T any](m *music, a *animation.Animation[T], target T, d time.Duration) {
	window := m.app.FindWindow("music")
	if window == nil {
		a.SetValue(target)
		return
	}
	if err := a.AnimateTo(window, target, d, animation.EaseOutCubic); err != nil {
		a.SetValue(target)
	}
}

func (m *music) retint() { animate(m, m.tint, m.player.current().colors(), tintDuration) }

func (m *music) toggle() {
	m.player.toggle(time.Now())
	target := float32(0)
	if m.player.playing {
		target = 1
	}
	animate(m, m.energy, target, energyDuration)
	m.changed()
}

func (m *music) play(i int) {
	if i != m.player.index {
		m.player.jump(i, time.Now())
		m.retint()
	}
	if !m.player.playing {
		m.toggle()
		return
	}
	m.changed()
}

func (m *music) next() {
	m.player.next(time.Now())
	m.retint()
	m.changed()
}

func (m *music) previous() {
	before := m.player.index
	m.player.previous(time.Now())
	if m.player.index != before {
		m.retint()
	}
	m.changed()
}

func (m *music) seek(fraction float64) {
	m.player.seek(fraction, time.Now())
	m.changed()
}

func (m *music) like() {
	i := m.player.index
	m.player.liked[i] = !m.player.liked[i]
	m.changed()
}

func (m *music) tick() {
	if m.player.advance(time.Now()) {
		m.retint()
		m.changed()
		return
	}
	m.syncClock()
}

func (m *music) stop() {
	if m.ticker != nil {
		m.ticker.Stop()
		m.ticker = nil
	}
	m.tint.Stop()
	m.energy.Stop()
}

func (m *music) build(app ui.App) ui.RootView {
	m.app = app
	if m.ticker == nil {
		m.ticker = app.TickFunc(tickInterval, m.tick)
	}
	window := ui.Window("music").Title("GOUI Music").Chrome(ui.WindowChromeIntegrated).
		Size(1100, 720).MinSize(860, 600).Center(true).
		Shortcuts(
			ui.Shortcut(ui.KeySpace).OnActivate(m.toggle),
			ui.Shortcut(ui.KeyArrowLeft).OnActivate(m.previous),
			ui.Shortcut(ui.KeyArrowRight).OnActivate(m.next),
		).
		OnDestroy(func() { m.stop(); app.Quit() }).
		Content(m.view())
	return ui.Root().StyleSheet(style.Sheet(styleRules(app.Settings().FontFamily())...)).Windows(window)
}

func main() {
	m, err := newMusic()
	if err != nil {
		log.Fatal(err)
	}
	defer m.stop()
	if err := ui.Run("org.golanggui.Music", m.build); err != nil {
		log.Fatal(err)
	}
}
