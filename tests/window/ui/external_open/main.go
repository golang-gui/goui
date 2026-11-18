// 声明式外部打开 API 验证。
// 环境：三平台桌面，已安装默认浏览器/文件管理器；默认后端，缩放不影响操作结果。
// 启动：go run ./tests/window/ui/external_open
// 初始：请求状态为 No request yet，不自动启动外部应用。
// 操作与预期：点击 Open URL/Path 才发起请求；有效地址由系统默认应用打开，
// 无效地址显示失败并记录具体错误；请求被接受只表示提交成功，须另观察外部应用结果。
// 副作用：明确点击后可能启动浏览器/文件管理器。复位：重启。
// 退出：关闭本窗口，外部应用由使用者关闭。
package main

import (
	"log"

	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
	"github.com/golang-gui/goui/ui"
)

func main() {
	url := ui.MakeState("https://example.com")
	path := ui.MakeState(".")
	status := ui.MakeState("No request yet.")
	report := func(err error) {
		if err != nil {
			status.Set("Request failed — see console for diagnostic.")
			log.Printf("external open: %v", err)
		} else {
			status.Set("Request accepted; external completion is not observable.")
		}
	}
	if err := ui.Run("org.golang-gui.ExternalOpenExample", func(app ui.App) ui.RootView {
		settings := app.Settings()
		return ui.Root().StyleSheet(modern.Sheet(modern.Options{
			Dark:        settings.ColorScheme() == ui.ColorSchemeDark,
			AccentColor: settings.AccentColor(),
			FontFamily:  settings.FontFamily(), FontSize: settings.FontSize(),
		})).Windows(
			ui.Window("external-open").Title("GOUI External Open").Size(680, 420).
				Chrome(ui.WindowChromeIntegrated).Content(
				ui.VBox(
					ui.HeaderBar(ui.Label("External Open")),
					ui.VBox(
						ui.Label("URL — https, mailto or an installed custom protocol"),
						ui.TextInput().BindText(&url),
						ui.Button("Open URL").OnClick(func() { report(app.OpenURL(url.Get())) }),
						ui.Label("Native file or directory path — relative paths are allowed"),
						ui.TextInput().BindText(&path),
						ui.Button("Open path").OnClick(func() { report(app.OpenPath(path.Get())) }),
						ui.Label(status.Get()),
					).Padding(20).Spacing(12).CrossAlign(layout.CrossStretch),
				).CrossAlign(layout.CrossStretch),
			),
		)
	}); err != nil {
		log.Fatal(err)
	}
}
