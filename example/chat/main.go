// GOUI chat: a project demo that favors a finished look over API coverage.
// Every conversation is fictional sample data; nothing is sent or stored.
//
// Environment: a desktop session; Linux requires X11. Choose the backend and
// scale with GOUI_PLAT_PAINTER and GOUI_PLAT_SCALE.
// Run: go run ./example/chat. It starts in the light palette with 林薇 open.
// Steps and expected results:
//  1. Click another conversation: it opens, its unread badge clears and the
//     message list scrolls to the newest message.
//  2. Type a message and press Enter or click the round send button: a blue
//     bubble appears on the right with 已读 below it (only the newest own
//     message shows it) and the conversation moves to the top of the list.
//     The send button is gray and disabled while the input is blank. The smile
//     button opens a menu that appends an emoji to the input.
//  3. Shortly afterwards the header and the list show three breathing dots
//     while the other side "types", then a reply bubble appears. Each
//     conversation has a few canned replies; afterwards nobody answers.
//  4. Switch to another conversation before the reply arrives: the reply is
//     counted as unread on the original conversation.
//  5. Type in 搜索会话 to filter conversations by name.
//  6. Click the sun/moon button at the bottom of the left rail to switch light
//     and dark; drag the divider to resize the conversation list. The rail's
//     chat, contact and starred icons are decoration and do not navigate.
//  7. The call, video and attachment buttons show a short notice at the bottom
//     instead of starting a real call or opening files.
//
// Side effects: none. Reset: restart. Exit: close the window.
package main

import (
	"log"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/style"
	"github.com/golang-gui/goui/ui"
	wui "github.com/golang-gui/goui/widgets/ui"
)

const (
	typingDelay   = 700 * time.Millisecond  // before the other side starts typing
	typingLasting = 1800 * time.Millisecond // how long the dots show
)

// chat owns the sample data and every piece of presentation state.
type chat struct {
	app           ui.App
	icons         *icons
	toast         wui.ToastHandle
	store         *store
	clock         clock
	dark          bool
	sideWidth     float32
	query         ui.State[string]
	draft         ui.State[string]
	current       ui.State[string]
	revision      ui.State[int] // advances whenever conversation data changes
	typing        map[string]string
	scrollPending bool
}

func newChat() (*chat, error) {
	icons, err := loadIcons()
	if err != nil {
		return nil, err
	}
	c := &chat{icons: icons, store: newStore(), clock: clock{minutes: 10*60 + 36}, sideWidth: 320, typing: map[string]string{}}
	c.current = ui.MakeState("lin")
	c.scrollPending = true
	return c, nil
}

func (c *chat) palette() palette {
	if c.dark {
		return darkPalette
	}
	return lightPalette
}

func (c *chat) changed() { c.revision.Set(c.revision.Peek() + 1) }

func (c *chat) open(id string) {
	if id == c.current.Peek() {
		return
	}
	c.store.open(id)
	c.current.Set(id)
	c.scrollPending = true
	c.changed()
}

func (c *chat) send() {
	id := c.current.Peek()
	if !c.store.send(id, c.draft.Peek(), today, c.clock.next()) {
		return
	}
	c.draft.Set("")
	c.scrollPending = true
	c.changed()
	if !c.store.expectReply(id) {
		return
	}
	c.app.TimeoutFunc(typingDelay, func() {
		c.typing[id] = c.store.nextSender(id)
		c.scrollPending = id == c.current.Peek()
		c.changed()
		c.app.TimeoutFunc(typingLasting, func() { c.deliver(id) })
	})
}

func (c *chat) deliver(id string) {
	delete(c.typing, id)
	visible := id == c.current.Peek()
	if c.store.deliverReply(id, today, c.clock.next(), visible) {
		c.scrollPending = visible
	}
	c.changed()
}

// notice briefly explains a demo-only action at the bottom of the window.
func (c *chat) notice(text string) {
	_ = c.toast.Show(ui.Label(text), 2500*time.Millisecond)
}

// insert appends text to the draft, as an emoji picker would.
func (c *chat) insert(text string) { c.draft.Set(c.draft.Peek() + text) }

func (c *chat) toggleTheme() {
	c.dark = !c.dark
	c.app.RequestUpdate() // The root owns the style sheet.
}

func (c *chat) build(app ui.App) ui.RootView {
	c.app = app
	rules := styleRules(c.palette(), c.dark, app.Settings().FontFamily())
	window := ui.Window("chat").Title("GOUI Chat").Chrome(ui.WindowChromeIntegrated).
		Size(1040, 720).MinSize(720, 520).Center(true).
		OnDestroy(app.Quit).
		Content(ui.Overlay(c.view(), ui.OverlayItem(wui.Toast(&c.toast).ID("notice")).
			OnPosition(func(a, s geometry.Size, p *geometry.Point) {
				*p = geometry.Point{X: max(0, (a.Width-s.Width)/2), Y: max(0, a.Height-s.Height-96)}
			})))
	return ui.Root().StyleSheet(style.Sheet(rules...)).Windows(window)
}

func main() {
	c, err := newChat()
	if err != nil {
		log.Fatal(err)
	}
	if err := ui.Run("org.golanggui.Chat", c.build); err != nil {
		log.Fatal(err)
	}
}
