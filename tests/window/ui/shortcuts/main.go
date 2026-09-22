// 快捷键与键盘布局窗口验证。
//
// 环境：Windows/macOS/Linux 桌面；记录 OS、实际键盘布局、IME、后端和缩放。
// 启动：go run ./tests/window/ui/shortcuts；macOS 的 Primary 为 Command，其他为 Ctrl。
// 初始：全部计数为零；快捷键启用，输入框为空。
// 操作与预期：
//  1. 点击按钮获得非编辑焦点，Primary+S 保存计数加一；按住不连续增加。
//     F6 允许系统重复，按住会增加 repeat；Primary+Shift+S 不增加保存计数。
//  2. 输入框键入 abc，Primary+A 应全选文本，不能增加 window-A；按钮获得焦点后
//     Primary+A 应增加 window-A。输入框 F5 增加 local-F5，按钮 F5 增加 window-F5。
//     回归重点：每次 F5 按下后保持约 100～200ms 再抬起，连续三次应只让对应计数加三；
//     不能仅模拟同一时刻的按下/抬起。长按 F5 一秒只加一，抬起重按再加一；
//     输入框长按 F6 仍应重复增加 repeat。Linux 在输入法中/英文模式下分别验证。
//  3. 点击 Rebuild 多次后按 Primary+S，只增加一次；Disable 后不再增加，Enable 恢复。
//  4. 系统切换 French AZERTY / German QWERTZ（记录所用布局），按钮获得焦点后
//     按逻辑 Primary+Q、Primary+Y，对应 Q/Y 计数增加，不按美式固定键位判断。
//     开/关 CapsLock 并重复；显式 Shift 是额外修饰键，不匹配上述声明。
//  5. 输入框使用该布局的 AltGr/Option 字符、死键和中文 IME，预期正常提交且没有
//     多余命令计数；Primary+S 激活后不向文本插入 s 或控制字符。
//  6. 关闭窗口，应退出，无残留窗口。不要以程序启动代替以上步骤验收。
//
// 副作用：只改变程序内状态；如手动更改系统键盘布局/环境变量，退出后恢复原值。
// 复位：重新启动。系统保留组合可能被截获，不能要求应用收到 Win 等保留快捷键。
package main

import (
	"fmt"
	"log"

	"github.com/golang-gui/goui/ui"
)

func main() {
	save, repeat, windowA, localF5, windowF5, q, y := 0, 0, 0, 0, 0, 0, 0
	enabled := true
	if err := ui.Run("org.golang-gui.Shortcuts", func(app ui.App) ui.RootView {
		binding := func(key ui.Key, mods ui.KeyModifiers, counter *int) *ui.ShortcutView {
			return ui.Shortcut(key).Modifiers(mods).Enabled(enabled).OnActivate(func() {
				*counter++
				log.Printf("key=%d save=%d local-F5=%d window-F5=%d repeat=%d Q=%d Y=%d", key, save, localF5, windowF5, repeat, q, y)
				app.RequestUpdate()
			})
		}
		return ui.Window("shortcuts").Title("GOUI shortcuts / keyboard layouts").Size(720, 420).
			Shortcuts(binding(ui.KeyS, ui.ModPrimary, &save), binding(ui.KeyA, ui.ModPrimary, &windowA),
				binding(ui.KeyF5, 0, &windowF5), binding(ui.KeyF6, 0, &repeat).AutoRepeat(true),
				binding(ui.KeyQ, ui.ModPrimary, &q), binding(ui.KeyY, ui.ModPrimary, &y)).
			Content(ui.VBox(
				ui.Label(fmt.Sprintf("save=%d repeat=%d window-A=%d\nlocal-F5=%d window-F5=%d Q=%d Y=%d", save, repeat, windowA, localF5, windowF5, q, y)),
				ui.TextInput().Shortcuts(binding(ui.KeyF5, 0, &localF5)),
				ui.Button("Rebuild / focus button").OnClick(func() { app.RequestUpdate() }),
				ui.Button(fmt.Sprintf("Enabled: %t (toggle)", enabled)).OnClick(func() { enabled = !enabled; app.RequestUpdate() }),
			).Spacing(12))
	}); err != nil {
		log.Fatal(err)
	}
}
