package main

import (
	"fmt"
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

const (
	highCPU   = 80
	statWidth = 240 // DIP; exceeds every stat card's natural width
)

func (m *monitor) view() ui.View {
	return ui.VBox(
		m.header(),
		ui.VBox(
			ui.HBox(
				// Equal minimums give the weighted cards the same basis, so they
				// share the width evenly whatever their content measures.
				m.cpuCard().MinWidth(statWidth).MainWeight(1),
				m.memoryCard().MinWidth(statWidth).MainWeight(1),
				m.networkCard().MinWidth(statWidth).MainWeight(1),
			).Spacing(16).CrossAlign(layout.CrossStretch),
			m.historyCard(),
			m.processCard().MainWeight(1),
		).Paddings(geometry.Insets{Left: 20, Right: 20, Top: 4, Bottom: 20}).Spacing(16).
			CrossAlign(layout.CrossStretch).MainWeight(1),
	).CrossAlign(layout.CrossStretch)
}

func (m *monitor) header() ui.View {
	// The icon shows the palette the button switches to.
	theme, label := m.icons.sun, "Switch to light theme"
	if !m.dark {
		theme, label = m.icons.moon, "Switch to dark theme"
	}
	return ui.HeaderBar(ui.HBox(
		ui.Label("GOUI Monitor").Style(styleTitle),
		ui.HBox().MainWeight(1),
		ui.Button().Content(ui.Icon(theme).Size(20).Style(styleIcon)).Style(styleIconButton).
			ID("theme").Tooltip(label).Padding(0).MinSize(36, 36).MaxSize(36, 36).OnClick(m.toggleTheme),
	).Spacing(12)).Paddings(geometry.Insets{Left: 20, Right: 12, Top: 8, Bottom: 8})
}

func card(children ...ui.View) *ui.BoxView {
	return ui.VBox(children...).Spacing(12).Padding(16).CrossAlign(layout.CrossStretch).Style(styleCard)
}

func (m *monitor) cpuCard() *ui.BoxView {
	return card(
		ui.Label("CPU").Style(styleCaption),
		ui.Build(func() ui.View {
			r := m.shown.Get()
			gauge := styleGauge
			if r.CPU > highCPU {
				gauge = styleGaugeHigh
			}
			load := r.CPU / 100 * cpuCores
			return ui.HBox(
				wui.ProgressBar(r.CPU/100).Shape(wui.ProgressCircular).Thickness(6).Style(gauge).
					MinSize(64, 64).MaxSize(64, 64),
				ui.VBox(
					ui.Label(fmt.Sprintf("%.1f%%", r.CPU)).Style(styleValue),
					ui.Label(fmt.Sprintf("Load %.2f · %d cores", load, cpuCores)).Style(styleMuted),
				).Spacing(2),
			).Spacing(18)
		}),
	)
}

func (m *monitor) memoryCard() *ui.BoxView {
	return card(
		ui.Label("Memory").Style(styleCaption),
		ui.Build(func() ui.View {
			r := m.shown.Get()
			return ui.VBox(
				ui.HBox(
					ui.Label(fmt.Sprintf("%.1f GB", r.Memory)).Style(styleValue),
					ui.Label(fmt.Sprintf("/ %.0f GB", float32(memoryTotal))).Style(styleMuted),
				).Spacing(6).CrossAlign(layout.CrossBaseline),
				wui.ProgressBar(r.Memory/memoryTotal).Thickness(6).Style(styleMeter),
				ui.Label(fmt.Sprintf("Used %.0f%% · %.1f GB free", r.Memory/memoryTotal*100, memoryTotal-r.Memory)).Style(styleMuted),
			).Spacing(10).CrossAlign(layout.CrossStretch)
		}),
	)
}

func (m *monitor) networkCard() *ui.BoxView {
	return card(
		ui.Label("Network").Style(styleCaption),
		ui.Build(func() ui.View {
			r := m.shown.Get()
			return ui.HBox(
				ui.VBox(ui.Label("↓ Download").Style(styleMuted), ui.Label(formatRate(r.Down)).Style(styleTitle)).Spacing(2),
				ui.VBox(ui.Label("↑ Upload").Style(styleMuted), ui.Label(formatRate(r.Up)).Style(styleTitle)).Spacing(2),
			).Spacing(24)
		}),
		newChart("Network rate", func(p gui.Painter, size geometry.Size) {
			c := m.palette()
			ceiling := max(slices.Max(m.sim.down), 1) * 1.15
			drawSeries(p, size, ceiling, m.scroll, series{values: m.sim.down, color: c.accent, fill: true})
			drawSeries(p, size, ceiling, m.scroll, series{values: m.sim.up, color: c.muted})
		}).ID("network-chart").MinHeight(44).MainWeight(1),
	)
}

func (m *monitor) historyCard() ui.View {
	return card(
		ui.HBox(
			ui.Label("CPU usage").Style(styleTitle),
			ui.Label("last 60 s").Style(styleMuted),
			ui.HBox().MainWeight(1),
			ui.Label("100%").Style(styleMuted),
		).Spacing(10),
		newChart("CPU usage", func(p gui.Painter, size geometry.Size) {
			c := m.palette()
			line := c.accent
			if m.sim.current.CPU > highCPU {
				line = c.danger
			}
			drawGrid(p, size, c.border)
			drawSeries(p, size, 100, m.scroll, series{values: m.sim.cpu, color: line, fill: true})
			drawLatest(p, size, 100, m.scroll, m.sim.cpu, line, c.surface)
		}).ID("cpu-chart").MinHeight(150).MaxHeight(150),
	)
}

func (m *monitor) processCard() *ui.BoxView {
	cell := func(text, name string, end bool) ui.View {
		row := ui.HBox(ui.Label(text).Style(name)).Paddings(geometry.Insets{Left: 12, Right: 12, Top: 8, Bottom: 8})
		if end {
			row.MainAlign(layout.MainEnd)
		}
		return row
	}
	table := ui.Build(func() ui.View {
		return wui.TableView[process](m.processes,
			wui.TableColumn("name", "Name", func(_ widgets.TableRow, p process) ui.View { return cell(p.Name, styleCell, false) }).
				Width(220).MinWidth(140).Expand(true).Sortable(true),
			wui.TableColumn("pid", "PID", func(_ widgets.TableRow, p process) ui.View { return cell(fmt.Sprint(p.PID), styleCellMuted, true) }).
				Width(80).Resizable(false),
			wui.TableColumn("cpu", "CPU", func(_ widgets.TableRow, p process) ui.View {
				return cell(fmt.Sprintf("%.1f%%", p.CPU), styleCell, true)
			}).
				Width(90).Resizable(false).Sortable(true),
			wui.TableColumn("memory", "Memory", func(_ widgets.TableRow, p process) ui.View { return cell(formatMemory(p.Memory), styleCell, true) }).
				Width(100).Resizable(false).Sortable(true),
			wui.TableColumn("usage", "Usage", func(_ widgets.TableRow, p process) ui.View {
				bar := styleBar
				if p.CPU > 30 {
					bar = styleBarHigh
				}
				return ui.HBox(wui.ProgressBar(min(p.CPU/50, 1)).Thickness(4).Style(bar).MainWeight(1)).
					Paddings(geometry.Insets{Left: 12, Right: 16}).CrossAlign(layout.CrossCenter)
			}).Width(160).Resizable(false),
		).ID("processes").
			Sort(m.sortBy.Get(), m.sortOrder.Get()).
			Selection(m.selection.Get()).
			OnSelection(func(ids []string) { m.selection.Set(slices.Clone(ids)) }).
			OnSortRequest(func(column string, order widgets.SortOrder) {
				m.sortBy.Set(column)
				m.sortOrder.Set(order)
				m.processes.SetItems(m.sim.sortedProcesses(column, order))
			}).
			VScrollBarOptions(ui.ScrollBarOptions{Thickness: 10, ThumbThickness: 6})
	})
	return card(
		ui.HBox(
			ui.Label("Processes").Style(styleTitle),
			ui.Label(fmt.Sprintf("%d total", len(m.sim.processes))).Style(styleMuted),
		).Spacing(10).Paddings(geometry.Insets{Left: 12, Right: 12}),
		ui.Modify(table).MainWeight(1),
	).Paddings(geometry.Insets{Left: 4, Right: 4, Top: 16, Bottom: 4}).Spacing(8)
}
