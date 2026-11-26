package main

import (
	"fmt"
	"strings"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/ui"
	wui "github.com/golang-gui/goui/widgets/ui"
)

const bubbleMaxWidth = 440 // DIP

// headerHeight aligns the rail, list and conversation headers.
const headerHeight = 60

func (c *chat) view() ui.View {
	return ui.HBox(
		c.rail(),
		wui.HSplit(c.sidebar(), c.conversation()).StartSize(c.sideWidth).
			OnResize(func(start, _ float32) { c.sideWidth = start }).MainWeight(1),
	).CrossAlign(layout.CrossStretch)
}

// rail is the narrow application column: brand, decorative sections, the
// palette switch and the user's own avatar.
func (c *chat) rail() ui.View {
	nav := func(source ui.IconSource, label string, current bool) ui.View {
		box, ink := styleNav, styleNavIcon
		if current {
			box, ink = styleNavCurrent, styleNavIconCurrent
		}
		return ui.VBox(ui.Icon(source).Size(22).Style(ink)).Style(box).Tooltip(label).
			MainAlign(layout.MainCenter).CrossAlign(layout.CrossCenter).MinSize(40, 40).MaxSize(40, 40)
	}
	theme, themeLabel := c.icons.moon, "切换到暗色"
	if c.dark {
		theme, themeLabel = c.icons.sun, "切换到亮色"
	}
	return ui.VBox(
		// HeaderBar stretches its child across the row; center the fixed mark.
		ui.HeaderBar(ui.HBox(ui.VBox(ui.Icon(c.icons.logo).Size(20).Style(styleLogoIcon)).Style(styleLogo).
			MainAlign(layout.MainCenter).CrossAlign(layout.CrossCenter).MinSize(36, 36).MaxSize(36, 36)).
			MainAlign(layout.MainCenter)).Style(styleRail).Padding(0).MinHeight(headerHeight),
		ui.VBox(
			nav(c.icons.chats, "消息", true),
			nav(c.icons.contacts, "联系人", false),
			nav(c.icons.starred, "收藏", false),
		).Spacing(8).Paddings(geometry.Insets{Top: 8}),
		spacer(),
		iconButton(theme, themeLabel).ID("theme").OnClick(c.toggleTheme),
		avatar("我", 1, 36),
	).Style(styleRail).Spacing(12).CrossAlign(layout.CrossCenter).
		Paddings(geometry.Insets{Bottom: 16}).MinWidth(64).MaxWidth(64)
}

// iconButton is a quiet, square, icon-only action.
func iconButton(source ui.IconSource, label string) *ui.ButtonView {
	return ui.Button().Content(ui.Icon(source).Size(20).Style(styleIcon)).Style(styleIconButton).
		Tooltip(label).Padding(0).MinSize(36, 36).MaxSize(36, 36)
}

func divider() ui.View {
	return ui.VBox().Style(styleDivider).MinHeight(1).MaxHeight(1)
}

func spacer() *ui.BoxView { return ui.HBox().MainWeight(1) }

func (c *chat) sidebar() ui.View {
	return ui.VBox(
		ui.HeaderBar(ui.HBox(ui.Label("消息").Style(styleTitle))).Style(styleSidebar).
			Paddings(geometry.Insets{Left: 20, Right: 12}).MinHeight(headerHeight),
		ui.HBox(
			ui.TextInput().BindText(&c.query).Placeholder("搜索会话").Style(styleSearch).ID("search").
				Paddings(geometry.Insets{Left: 12, Right: 12, Top: 7, Bottom: 7}).MainWeight(1),
		).Paddings(geometry.Insets{Left: 12, Right: 12, Bottom: 8}),
		ui.ScrollView(ui.Build(c.conversationList)).Direction(ui.ScrollVertical).
			VScrollBarOptions(ui.ScrollBarOptions{Thickness: 8, ThumbThickness: 4}).MainWeight(1),
	).Style(styleSidebar).CrossAlign(layout.CrossStretch).MinWidth(240)
}

func (c *chat) conversationList() ui.View {
	c.revision.Get()
	current := c.current.Get()
	contacts := c.store.filter(c.query.Get())
	if len(contacts) == 0 {
		return ui.VBox(ui.Label("没有匹配的会话").Style(styleMuted)).
			Padding(24).CrossAlign(layout.CrossCenter)
	}
	rows := make([]ui.View, 0, len(contacts))
	for _, contact := range contacts {
		rows = append(rows, c.conversationRow(contact, contact.ID == current))
	}
	return ui.VBox(rows...).Spacing(2).Paddings(geometry.Insets{Left: 8, Right: 8, Bottom: 8}).
		CrossAlign(layout.CrossStretch)
}

func (c *chat) conversationRow(contact *contact, current bool) ui.View {
	row := styleRow
	if current {
		row = styleRowCurrent
	}
	var badge ui.View = ui.HBox().MinHeight(18)
	if contact.Unread > 0 {
		badge = ui.HBox(ui.Label(fmt.Sprint(contact.Unread)).Style(styleBadgeText)).Style(styleBadge).
			MainAlign(layout.MainCenter).CrossAlign(layout.CrossCenter).
			Paddings(geometry.Insets{Left: 6, Right: 6}).MinSize(18, 18).MaxHeight(18)
	}
	id := contact.ID
	item := wui.ListRow(contact.Name).Subtitle(preview(contact)).
		Prefix(presence(avatar(contact.Name, contact.Color, 44), contact.Online)).
		Suffix(ui.VBox(ui.Label(lastActivity(contact)).Style(styleSmall), badge).
			Spacing(6).CrossAlign(layout.CrossEnd)).
		Activatable(true).OnActivate(func() { c.open(id) }).
		Paddings(geometry.Insets{Left: 10, Right: 12, Top: 10, Bottom: 10}).
		Style(row).ID("row-" + id)
	if !current {
		return item
	}
	// A short accent bar marks the open conversation at the row's left edge.
	bar := ui.VBox().Style(styleIndicator).MinSize(3, 24).MaxSize(3, 24)
	return ui.Overlay(item, ui.OverlayItem(bar).OnPosition(func(a, s geometry.Size, p *geometry.Point) {
		*p = geometry.Point{Y: (a.Height - s.Height) / 2}
	}))
}

func (c *chat) conversation() ui.View {
	return ui.VBox(
		ui.Build(c.conversationHeader),
		divider(),
		// The shade lies over the bottom of the messages, so it scrolls nothing
		// out of view and lifts the composer without a hard rule.
		ui.Overlay(
			ui.ScrollView(ui.Build(c.messages)).Direction(ui.ScrollVertical).ID("messages").
				VScrollBarOptions(ui.ScrollBarOptions{Thickness: 8, ThumbThickness: 4}),
			ui.OverlayItem(shade()).OnPosition(func(a, s geometry.Size, p *geometry.Point) {
				*p = geometry.Point{Y: a.Height - s.Height}
			}),
		).MainWeight(1),
		c.composer(),
	).Style(styleConversation).CrossAlign(layout.CrossStretch).MinWidth(420)
}

func (c *chat) conversationHeader() ui.View {
	c.revision.Get()
	contact := c.store.find(c.current.Get())
	if contact == nil {
		return ui.HeaderBar(ui.HBox())
	}
	status := ui.Label(contact.Seen).Style(styleMuted)
	switch {
	case c.typing[contact.ID] != "":
		status = ui.Label(typingText(contact, c.typing[contact.ID])).Style(styleOnline)
	case contact.Group:
		status = ui.Label(fmt.Sprintf("%d 位成员", len(contact.Members)+1)).Style(styleMuted)
	case contact.Online:
		status = ui.Label("在线").Style(styleOnline)
	}
	return ui.HeaderBar(ui.HBox(
		presence(avatar(contact.Name, contact.Color, 36), contact.Online && !contact.Group),
		ui.VBox(ui.Label(contact.Name).Style(styleTitle), status).Spacing(1),
		spacer(),
		iconButton(c.icons.phone, "语音通话").ID("call").OnClick(func() { c.notice("示例不会发起真实通话") }),
		iconButton(c.icons.video, "视频通话").ID("video").OnClick(func() { c.notice("示例不会发起真实通话") }),
	).Spacing(12)).Paddings(geometry.Insets{Left: 20, Right: 12}).MinHeight(headerHeight)
}

func typingText(contact *contact, sender string) string {
	if contact.Group {
		return sender + " 正在输入…"
	}
	return "正在输入…"
}

func (c *chat) messages() ui.View {
	c.revision.Get()
	contact := c.store.find(c.current.Get())
	if contact == nil {
		return ui.VBox()
	}
	items := make([]ui.View, 0, len(contact.Messages)+4)
	receipt := lastMine(contact)
	for i, m := range contact.Messages {
		var prev, next *message
		if i > 0 {
			prev = &contact.Messages[i-1]
		}
		if i+1 < len(contact.Messages) {
			next = &contact.Messages[i+1]
		}
		if prev == nil || prev.Day != m.Day {
			items = append(items, daySeparator(m.Day))
			prev = nil
		}
		first := prev == nil || prev.Sender != m.Sender
		last := next == nil || next.Sender != m.Sender || next.Day != m.Day
		gap := float32(3)
		if first {
			gap = 14
		}
		items = append(items, ui.VBox(messageRow(contact, m, first, last)).
			Paddings(geometry.Insets{Top: gap}).CrossAlign(layout.CrossStretch))
		if i == receipt {
			items = append(items, ui.HBox(ui.Label("已读").Style(styleReceipt)).MainAlign(layout.MainEnd).
				Paddings(geometry.Insets{Top: 4, Right: 4}))
		}
	}
	if sender := c.typing[contact.ID]; sender != "" {
		items = append(items, ui.VBox(typingRow(contact, sender)).
			Paddings(geometry.Insets{Top: 14}).CrossAlign(layout.CrossStretch))
	}
	list := ui.VBox(items...).Paddings(geometry.Insets{Left: 24, Right: 24, Top: 8, Bottom: 20}).
		CrossAlign(layout.CrossStretch)
	if !c.scrollPending {
		return list
	}
	c.scrollPending = false
	return afterUpdate(list, c.scrollToEnd)
}

// scrollToEnd reveals the newest message. The reconciled list is measured by
// the next layout, so the reveal waits for that frame's BeforePaint, when the
// scroll extent already includes the new message.
func (c *chat) scrollToEnd() {
	sv, ok := c.app.FindWidget("chat", "messages").(*gui.ScrollView)
	if !ok {
		return
	}
	root := sv.Root()
	if root == nil {
		return
	}
	var handle signal.Handle
	handle = root.ConnectBeforePaint(func() {
		handle.Disconnect()
		if !sv.Destroyed() && sv.Root() == root {
			sv.ScrollIntoView(geometry.Rect(0, 1e7, 1, 1))
		}
	})
	_ = root.RequestPaint()
}

func daySeparator(day string) ui.View {
	return ui.HBox(
		ui.HBox(ui.Label(day).Style(styleDayText)).Style(styleDay).
			Paddings(geometry.Insets{Left: 10, Right: 10, Top: 3, Bottom: 3}),
	).MainAlign(layout.MainCenter).Paddings(geometry.Insets{Top: 16, Bottom: 2})
}

func bubble(m message, mine bool) ui.View {
	box, text, at := styleReceived, styleReceivedText, styleSmall
	if mine {
		box, text, at = styleSent, styleSentText, styleSentTime
	}
	// The timestamp shares the text's row and aligns with its last line, so a
	// one-line message keeps a one-line bubble instead of adding an empty row.
	return ui.HBox(
		ui.Label(m.Text).WrapMode(gui.WrapWordChar).Style(text).MainWeight(1),
		ui.Label(m.Time).Style(at),
	).Style(box).Spacing(8).CrossAlign(layout.CrossEnd).
		Paddings(geometry.Insets{Left: 14, Right: 12, Top: 8, Bottom: 6}).MaxWidth(bubbleMaxWidth)
}

// Bubbles are aligned on a VBox's cross axis rather than an HBox's main axis:
// the cross axis bounds them by the available width, so a narrow window wraps
// the text instead of clipping the bubble at its maximum width.
func messageRow(contact *contact, m message, first, last bool) ui.View {
	if m.mine() {
		return ui.VBox(bubble(m, true)).CrossAlign(layout.CrossEnd)
	}
	if !contact.Group {
		return ui.VBox(bubble(m, false)).CrossAlign(layout.CrossStart)
	}
	content := []ui.View{bubble(m, false)}
	if first {
		content = append([]ui.View{ui.HBox(ui.Label(m.Sender).Style(senderStyle(contact.memberColor(m.Sender)))).
			Paddings(geometry.Insets{Left: 4})}, content...)
	}
	return ui.HBox(memberAvatar(contact, m.Sender, last),
		ui.VBox(content...).Spacing(4).CrossAlign(layout.CrossStart).MainWeight(1)).
		Spacing(10).CrossAlign(layout.CrossEnd)
}

// memberAvatar shows a sender's avatar beside the last bubble of a run and
// keeps the same indent elsewhere.
func memberAvatar(contact *contact, sender string, show bool) ui.View {
	if !show {
		return ui.HBox().MinWidth(28).MaxWidth(28)
	}
	return avatar(sender, contact.memberColor(sender), 28)
}

func typingRow(contact *contact, sender string) ui.View {
	dots := ui.HBox(typing()).Style(styleReceived).
		Paddings(geometry.Insets{Left: 12, Right: 12, Top: 8, Bottom: 8})
	if !contact.Group {
		return ui.HBox(dots)
	}
	return ui.HBox(avatar(sender, contact.memberColor(sender), 28), dots).
		Spacing(10).CrossAlign(layout.CrossEnd)
}

var emojis = []string{"😄", "👍", "🎉", "🙏", "❤️", "🤔"}

func (c *chat) composer() ui.View {
	picks := make([]*ui.MenuItemView, len(emojis))
	for i, e := range emojis {
		picks[i] = ui.MenuItem(e, func() { c.insert(e) })
	}
	return ui.HBox(
		iconButton(c.icons.attach, "附件").ID("attach").OnClick(func() { c.notice("示例不会读取本地文件") }),
		ui.MenuButton().Child(ui.Icon(c.icons.emoji).Size(20).Style(styleIcon)).Style(styleIconButton).
			Tooltip("表情").ID("emoji").MinSize(36, 36).MaxSize(36, 36).Menu(picks...),
		ui.TextInput().BindText(&c.draft).Placeholder("输入消息，按 Enter 发送").Style(styleComposer).ID("composer").
			Paddings(geometry.Insets{Left: 16, Right: 16, Top: 9, Bottom: 9}).OnSubmit(c.send).MainWeight(1),
		ui.Build(func() ui.View {
			return ui.Button().Content(ui.Icon(c.icons.send).Size(20).Style(styleSendIcon)).Style(styleSend).
				ID("send").Tooltip("发送").Padding(0).MinSize(40, 40).MaxSize(40, 40).
				Enabled(strings.TrimSpace(c.draft.Get()) != "").OnClick(c.send)
		}),
	).Style(styleComposerBar).Spacing(8).Paddings(geometry.Insets{Left: 12, Right: 16, Top: 12, Bottom: 14})
}
