// 菜单、文本编辑和全屏组合回归。
// 环境：三平台桌面；Software/OpenGL（Windows 另有 Direct2D），1x/2x。
// 启动：go run ./tests/window/ui/menu_editor
// 初始：正文为空，File/Close 禁用；下拉框与带格式 Tooltip 按钮可用。
// 操作与预期：File/Open 选择临时文本文件，正文更新且 Close 启用；取消不改正文；
// Close 清空正文与撤销历史后重新禁用；菜单与下拉框交替打开不能出现旧尺寸/内容；
// View/Fullscreen 后菜单仍锚定按钮并正确退避，View/Normal 恢复；悬停 Tooltip 显示格式文本。
// 平台差异：使用系统文件选择器；状态请求是尽力而为，须观察实际窗口状态，不能只看请求提交。
// 副作用：只读取选中文件；Help/About 显式启动浏览器，不保存正文。
// 复位：重启。退出：关闭窗口；全屏时可先选 View/Normal。
package main

import (
	"log"
	"os"

	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
	wui "github.com/golang-gui/goui/widgets/ui"
)

func main() {

	doc := ui.NewTextModel("")
	fileCloseEnabled := ui.MakeState(false)
	if err := ui.Run("org.golang-gui.MenuEditor", func(app ui.App) ui.RootView {
		return ui.Root().StyleSheet(modern.FromSettings(app.Settings())).Windows(
			ui.Window("main").Content(
				ui.VBox(
					ui.MenuButtonGroup(
						ui.MenuButton("File").Menu(
							ui.MenuItem("Open").DefineCommand(
								ui.Command("file.open").OnActivate(func() {
									app.FileDialog().OpenFile("main", ui.DialogOptions{Title: "Open text file"}, func(accesses []ui.FileAccess, err error) {
										if err != nil {
											log.Print(err)
											return
										}
										for _, access := range accesses {
											defer access.Close()
										}
										if len(accesses) != 1 {
											return
										}
										file, err := accesses[0].LocalPath()
										if err != nil {
											log.Print(err)
											return
										}
										data, err := os.ReadFile(file)
										if err != nil {
											log.Print(err)
											return
										}
										doc.SetText(string(data))
										fileCloseEnabled.Set(true)
									})
								}),
							),
							ui.MenuItem("Close").DefineCommand(
								ui.Command("file.close").BindEnabled(&fileCloseEnabled).OnActivate(func() {
									log.Printf("file.close activate")
									doc.SetText("")
									doc.ClearHistory()
									fileCloseEnabled.Set(false)
								}),
							),
						),
						ui.MenuButton("View").Menu(
							ui.MenuItem("Normal", func() {
								app.FindWindow("main").RequestState(gui.WindowStateNormal)
							}),
							ui.MenuItem("Fullscreen", func() {
								app.FindWindow("main").RequestState(gui.WindowStateFullscreen)
							}),
						),
						ui.MenuButton("Help").Menu(
							ui.MenuItem("About").DefineCommand(
								ui.Command("help.about").OnActivate(func() {
									if err := app.OpenURL("https://github.com/golang-gui/goui"); err != nil {
										log.Print(err)
									}
								}),
							),
						),
					),
					wui.DropDown(ui.SliceList([]wui.DropDownItem{
						{Text: "Item1"},
						{Text: "Item2"},
					})),
					ui.Button("Tooltip").Tooltip(`\{format\}\{warning:This is test tooltip message!\}`),
					ui.ScrollView(ui.TextView().Model(doc).MainWeight(1)).Style("text-view").MainWeight(1),
				),
			).Shortcuts(ui.Shortcut(ui.KeyW).Modifiers(ui.ModPrimary).Command("file.close")),
		)
	}); err != nil {
		log.Fatal(err)
	}
}
