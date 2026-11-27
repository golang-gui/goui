package main

import (
	"fmt"
	"time"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/ui"
	wui "github.com/golang-gui/goui/widgets/ui"
)

const (
	columnWidth  = 360 // DIP; the now-playing column
	recordSize   = 336 // DIP; a 264 DIP record plus room for its shadow
	recordMargin = 36  // DIP
	queueWidth   = 340 // DIP
	spinSpeed    = 90  // degrees per second at full energy
)

func (m *music) view() ui.View {
	// The ambient light is the Overlay's main child so it fills the window;
	// the content floats above it at the same size.
	ambient := canvas("", func(p gui.Painter, size geometry.Size) { drawAmbient(p, size, m.colors()) }).
		Live(m.tint.Running)
	content := ui.VBox(
		// An empty header row is the window's drag region and keeps clear of
		// the window controls.
		ui.HeaderBar(ui.HBox()).Style(styleClear).Padding(0).MinHeight(44),
		ui.HBox(
			ui.VBox(m.nowPlaying()).MainAlign(layout.MainCenter).CrossAlign(layout.CrossCenter).MainWeight(1),
			m.queue().MinWidth(queueWidth).MaxWidth(queueWidth),
		).Spacing(32).CrossAlign(layout.CrossStretch).
			Paddings(geometry.Insets{Left: 32, Right: 28, Bottom: 28}).MainWeight(1),
	).CrossAlign(layout.CrossStretch)
	return ui.Overlay(ambient, ui.OverlayItem(content).Fill(true))
}

func (m *music) nowPlaying() ui.View {
	var last time.Time
	record := canvas("Record", func(p gui.Painter, size geometry.Size) {
		drawRecord(p, size, m.colors(), m.spin, recordMargin)
	}).Live(m.animating).OnFrame(func(now time.Time) {
		// Speed follows the spectrum's energy, so the record winds up and
		// coasts to a stop instead of starting and halting abruptly.
		if !last.IsZero() {
			m.spin += float32(now.Sub(last).Seconds()) * spinSpeed * m.energy.CurrentValue()
			if m.spin >= 360 {
				m.spin -= 360
			}
		}
		last = now
		if !m.animating() {
			last = time.Time{}
		}
	}).MinSize(recordSize, recordSize).MaxSize(recordSize, recordSize)

	spectrum := canvas("Spectrum", func(p gui.Painter, size geometry.Size) {
		drawSpectrum(p, size, m.colors(), 28, m.energy.CurrentValue(), time.Since(m.started).Seconds())
	}).Live(m.animating).MinHeight(40).MaxHeight(40)

	return ui.VBox(
		record,
		ui.Build(m.songInfo),
		spectrum,
		ui.VBox(
			seek(m).ID("seek").MinHeight(22).MaxHeight(22),
			ui.Build(m.clock),
		).Spacing(2).CrossAlign(layout.CrossStretch),
		ui.Build(m.controls),
	).Spacing(16).CrossAlign(layout.CrossStretch).MinWidth(columnWidth).MaxWidth(columnWidth)
}

func (m *music) songInfo() ui.View {
	m.revision.Get()
	t := m.player.current()
	heart, label, ink := m.icons.heart, "Like", styleIcon
	if m.player.liked[m.player.index] {
		heart, label, ink = m.icons.heartFilled, "Unlike", styleLikedIcon
	}
	return ui.HBox(
		ui.VBox(
			ui.Label(t.Title).Style(styleTitle),
			ui.Label(t.Artist+" · "+t.Album).Style(styleArtist),
		).Spacing(4).MainWeight(1),
		ui.Button().Content(ui.Icon(heart).Size(22).Style(ink)).Style(styleIconButton).
			ID("like").Tooltip(label).Padding(0).MinSize(40, 40).MaxSize(40, 40).OnClick(m.like),
	).Spacing(12).CrossAlign(layout.CrossCenter).Paddings(geometry.Insets{Left: 8})
}

func (m *music) clock() ui.View {
	m.revision.Get()
	elapsed := time.Duration(m.seconds.Get()) * time.Second
	remaining := m.player.current().Length - elapsed
	return ui.HBox(
		ui.Label(formatDuration(elapsed)).Style(styleTime),
		ui.HBox().MainWeight(1),
		ui.Label("-"+formatDuration(remaining)).Style(styleTime),
	).Paddings(geometry.Insets{Left: 8, Right: 8})
}

func (m *music) controls() ui.View {
	m.revision.Get()
	play, label := m.icons.play, "Play"
	if m.player.playing {
		play, label = m.icons.pause, "Pause"
	}
	skip := func(source ui.IconSource, id, label string, fn func()) ui.View {
		return ui.Button().Content(ui.Icon(source).Size(26).Style(styleIcon)).Style(styleIconButton).
			ID(id).Tooltip(label).Padding(0).MinSize(48, 48).MaxSize(48, 48).OnClick(fn)
	}
	return ui.HBox(
		skip(m.icons.previous, "previous", "Previous", m.previous),
		ui.Button().Content(ui.Icon(play).Size(28).Style(stylePlayIcon)).Style(stylePlay).
			ID("play").Tooltip(label).Padding(0).MinSize(64, 64).MaxSize(64, 64).OnClick(m.toggle),
		skip(m.icons.next, "next", "Next", m.next),
	).Spacing(32).MainAlign(layout.MainCenter).CrossAlign(layout.CrossCenter)
}

func (m *music) queue() *ui.BoxView {
	return ui.VBox(
		ui.HBox(
			ui.Label("Playlist").Style(stylePanelTitle),
			ui.HBox().MainWeight(1),
			ui.Label(fmt.Sprintf("%d songs", len(tracks))).Style(styleCount),
		).CrossAlign(layout.CrossBaseline).Paddings(geometry.Insets{Left: 10, Right: 10, Bottom: 6}),
		ui.ScrollView(ui.Build(m.queueRows)).Direction(ui.ScrollVertical).
			VScrollBarOptions(ui.ScrollBarOptions{Thickness: 8, ThumbThickness: 4}).MainWeight(1),
	).Style(stylePanel).Spacing(8).Paddings(geometry.Insets{Left: 8, Right: 8, Top: 18, Bottom: 8}).
		CrossAlign(layout.CrossStretch)
}

func (m *music) queueRows() ui.View {
	m.revision.Get()
	rows := make([]ui.View, len(tracks))
	for i, t := range tracks {
		current := i == m.player.index
		style := styleRow
		var suffix ui.View = ui.Label(formatDuration(t.Length)).Style(styleLength)
		if current {
			style = styleRowCurrent
			// The playing song shows a small live meter instead of its length.
			suffix = canvas("Now playing", func(p gui.Painter, size geometry.Size) {
				drawSpectrum(p, size, m.colors(), 3, max(.25, m.energy.CurrentValue()), time.Since(m.started).Seconds()*1.6)
			}).Live(m.animating).MinSize(18, 18).MaxSize(18, 18)
		}
		colors := t.colors()
		thumb := canvas("", func(p gui.Painter, size geometry.Size) { drawRecord(p, size, colors, 0, 0) }).
			MinSize(40, 40).MaxSize(40, 40)
		rows[i] = wui.ListRow(t.Title).Subtitle(t.Artist).Prefix(thumb).Suffix(suffix).
			Activatable(true).OnActivate(func() { m.play(i) }).
			Paddings(geometry.Insets{Left: 10, Right: 14, Top: 8, Bottom: 8}).Spacing(12).
			Style(style).ID(fmt.Sprintf("song-%d", i))
	}
	return ui.VBox(rows...).Spacing(2).CrossAlign(layout.CrossStretch)
}
