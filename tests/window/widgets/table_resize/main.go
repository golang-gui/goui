// 表格手动列宽的原生窗口回归，不使用DevServer。
//
// 环境：Linux/X11（记录WM）、Windows或macOS；记录后端、缩放比例。
// 启动：go run ./tests/window/widgets/table_resize
// 可设置GOUI_PLAT_PAINTER=software/opengl/direct2d、GOUI_PLAT_SCALE=1/2。
// 初始：1180×620 DIP窗口，856 DIP宽的五列表，100行；所有列自动收缩。
//
// 操作与逐项预期（每一步完成后按F6，失败立即退出1）：
//  1. 等READY，按F6记录起始分配。
//  2. 将Size右边缘向右拖24 DIP，松手后按F6：Size增加24，左侧不变，
//     Description减少24；拖动每次OnResize都请求声明式重建，不能回跳。
//  3. 将Description右边缘向左拖32 DIP，松手后按F6：左侧各列保持，
//     Description减少32，右侧留白；表头、单元格和手柄Range一致。
//  4. 点击Rebuild，按F6：所有基础/实际宽度及列身份保持。
//  5. 按住Size右边缘向右移24 DIP，按Esc再松手，按F6：恢复整组宽度，
//     仅Size产生调整及回退通知；不得排序或残留捕获。
//  6. 点击Expand，按F6：开始新的自动分配；所有基础宽度保持，没有Resize。
//  7. 将Description右边缘向左拖24 DIP，松手后按F6：末列可手调且保留
//     Expand配置，左侧不变；不得被额外空间立即撑回去。
//  8. 点击Wide，按F6：外部表格宽度改为1100 DIP，前四列恢复基础宽度，
//     末列按保留的Expand填满；不产生Resize。点击Finish/F8退出0。
//
// F6输出独立数值断言及下一步所需的窗口局部坐标；还须观察原生边缘是否跟手。
// 平台差异：macOS可能需要Fn+F6/F8；拖动距离按实际PixelScale换算为物理像素。
// 副作用：仅测试窗口、内存数据和日志，不读写文件、剪贴板或系统设置。
// 复位：重启。提前关闭或缺少步骤退出1，正常完成由UI清理窗口及控件树。
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"slices"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	"github.com/golang-gui/goui/widgets"
	wui "github.com/golang-gui/goui/widgets/ui"
)

type rows struct{ *gui.SliceListModel[string] }

func (r *rows) RowID(i int) string { return fmt.Sprint(i) }

type sample struct {
	Widths  []float32                     `json:"widths"`
	Bases   []float32                     `json:"bases"`
	Headers []geometry.Rectangle          `json:"headers"`
	Buttons map[string]geometry.Rectangle `json:"buttons"`
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < .002 }

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	values := make([]string, 100)
	for i := range values {
		values[i] = fmt.Sprintf("Row %03d", i)
	}
	m := &rows{gui.NewSliceListModel(values)}
	ids := []string{"name", "note", "action", "size", "description"}
	widths := []float32{240, 200, 100, 140, 420}
	minima := []float32{80, 80, 80, 40, 40}
	notifications := make([]int, 5)
	phase, generation := 0, 0
	expand, wide, ready, finished := false, false, false, false
	var failure error
	var previous sample
	var original []*widgets.TableColumn
	var savedNotifications []int
	err := ui.Run("org.golanggui.TableResize", func(app ui.App) ui.RootView {
		fail := func(err error) {
			if failure == nil {
				failure = err
			}
			app.Quit()
		}
		table := func() *widgets.TableView { return app.FindWidget("main", "table").(*widgets.TableView) }
		capture := func() (sample, error) {
			v := table()
			info := v.Snapshot()
			s := sample{Buttons: map[string]geometry.Rectangle{}}
			for _, id := range []string{"rebuild", "expand", "wide", "finish"} {
				s.Buttons[id] = app.FindWidget("main", id).Snapshot().Bounds
			}
			for i, c := range v.Columns() {
				header := info.Children[i]
				if c != original[i] || !near(c.Width(), widths[i]) || header.Bounds.Width < c.MinWidth() || len(header.Children) != 1 || !near(header.Children[0].Range.Value, header.Bounds.Width) {
					return s, fmt.Errorf("column %d identity/base/min/range", i)
				}
				s.Widths = append(s.Widths, header.Bounds.Width)
				s.Bases = append(s.Bases, c.Width())
				s.Headers = append(s.Headers, header.Bounds)
			}
			for _, row := range info.Children[5:] {
				for i, cell := range row.Children {
					if !near(cell.Bounds.Width, s.Widths[i]) || !near(cell.Bounds.X, s.Headers[i].X) {
						return s, errors.New("header/cell geometry differs")
					}
				}
			}
			return s, nil
		}
		verify := func() error {
			if !ready || phase >= 8 {
				return errors.New("not ready or duplicate verification")
			}
			s, err := capture()
			if err != nil {
				return err
			}
			want := slices.Clone(previous.Widths)
			switch phase {
			case 0:
				if s.Widths[3] >= 140 || s.Widths[4] >= 420 {
					return errors.New("initial columns are not shrunk")
				}
			case 1:
				want[3] += 24
				want[4] -= 24
			case 2:
				want[4] -= 32
			case 3:
				if generation == 0 {
					return errors.New("Rebuild was not used")
				}
			case 4:
				if notifications[3] <= savedNotifications[3] || !slices.Equal(s.Bases, previous.Bases) {
					return errors.New("Esc did not restore base widths or emit rollback")
				}
			case 5:
				if !expand || !slices.Equal(notifications, savedNotifications) || !slices.Equal(s.Bases, previous.Bases) {
					return errors.New("Expand altered preferences or emitted Resize")
				}
			case 6:
				want[4] -= 24
			case 7:
				if !wide || !expand || !slices.Equal(notifications, savedNotifications) || !slices.Equal(s.Bases, previous.Bases) {
					return errors.New("automatic resume altered preferences or emitted Resize")
				}
				for i := 0; i < 4; i++ {
					if !near(s.Widths[i], widths[i]) {
						return fmt.Errorf("column %d did not recover its base width", i)
					}
				}
				if s.Widths[4] <= widths[4] || table().Snapshot().MaxScrollX != 0 {
					return errors.New("last column did not expand after width change")
				}
			}
			if phase != 0 && phase != 5 && phase != 7 {
				for i, w := range want {
					if !near(s.Widths[i], w) {
						return fmt.Errorf("phase %d column %d: actual=%g want=%g", phase, i, s.Widths[i], w)
					}
				}
			}
			if notifications[0] != 0 || notifications[1] != 0 || notifications[2] != 0 || phase >= 1 && notifications[3] == 0 || phase >= 2 && notifications[4] == 0 {
				return errors.New("Resize notifications do not match the manually edited columns")
			}
			data, _ := json.Marshal(s)
			fmt.Printf("CHECK phase=%d notifications=%v state=%s\n", phase, notifications, data)
			previous = s
			savedNotifications = slices.Clone(notifications)
			phase++
			return nil
		}
		finish := func() {
			if phase != 8 {
				fail(fmt.Errorf("missing stages: phase=%d", phase))
				return
			}
			finished = true
			fmt.Println("PASS table resize")
			app.Quit()
		}
		columns := make([]*wui.TableColumnView[string], 5)
		for i, id := range ids {
			columns[i] = wui.TableColumn(id, []string{"Name", "Notes", "Action", "Size", "Description"}[i], func(_ widgets.TableRow, text string) ui.View { return ui.Label(text) }).Width(widths[i]).MinWidth(minima[i]).OnResize(func(w float32) {
				widths[i] = w
				notifications[i]++
				fmt.Printf("RESIZE column=%s width=%g\n", id, w)
				app.RequestUpdate()
			})
			if i == 4 {
				columns[i].Expand(expand)
			}
		}
		width := float32(856)
		if wide {
			width = 1100
		}
		tv := wui.TableView(m, columns...).ID("table").MainWeight(0).MinWidth(width).MaxWidth(width).MinHeight(240).MaxHeight(240)
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{})).Windows(ui.Window("main").Title("GOUI Table resize").Size(1180, 620).Content(ui.VBox(
			ui.HBox(ui.Button("Rebuild").ID("rebuild").OnClick(func() { generation++; app.RequestUpdate() }), ui.Button("Expand").ID("expand").OnClick(func() { expand = true; app.RequestUpdate() }), ui.Button("Wide").ID("wide").OnClick(func() { wide = true; app.RequestUpdate() }), ui.Button("Finish").ID("finish").OnClick(finish)).Spacing(8),
			ui.Label("F6: initial / Size +24 / Description -32 / Rebuild / Esc / Expand / Description -24 / Wide"), tv,
		).Padding(16).Spacing(12).CrossAlign(layout.CrossStart)).Shortcuts(ui.Shortcut(ui.KeyF6).OnActivate(func() {
			if err := verify(); err != nil {
				fail(err)
			}
		}), ui.Shortcut(ui.KeyF8).OnActivate(finish)).OnAfterPaint(func(p gui.Painter, _ geometry.Rectangle) {
			if ready || table().Snapshot().Bounds.Width != width {
				return
			}
			original = table().Columns()
			s, err := capture()
			if err != nil {
				fail(err)
				return
			}
			resource, err := p.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
			if err != nil {
				fail(err)
				return
			}
			defer resource.Destroy()
			ready = true
			data, _ := json.Marshal(s)
			fmt.Printf("READY backend=%T scale=%g state=%s\n", resource, p.PixelScale(), data)
		}))
	})
	if err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	if !finished {
		return errors.New("closed before all stages passed")
	}
	return nil
}
