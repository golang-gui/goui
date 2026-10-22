// 文件访问凭据验收，使用公开 GUI 接口，不依赖 DevServer。
//
// 环境：Linux X11（zenity/kdialog/yad 至少一种）、Windows、macOS 桌面。
// 启动：go run ./tests/window/gui/file_access；macOS 沙箱必须用 cmd/goui
// 构建、打包、签名 .app，再通过 Finder/open 启动，不能用 go run 代替。
// 初始：无已打开资源；打印 HOME、UserConfigDir、UserCacheDir、实际后端和缩放。
// 准备：在应用目录以外的独立临时目录创建 a.txt、b.txt（任意内容）；目录中
// 至少有一项。仅使用测试文件，保存时不要覆盖业务文件。
// 操作与预期：
//  1. Open file 后取消：显示 CANCEL，无错误。再次打开并选择 a.txt：显示
//     PASS selected/read/credential，能够读内容并保存凭据；Name 是文件名。
//  2. Open multiple 选择两个文件：显示 count=2，两个文件均可读。Open folder
//     选择准备目录：显示 entries 数量，凭据关联目录而不是目录中的某个文件。
//  3. Save file 选择尚不存在的目标：程序断言选择没有偷偷创建文件，再写入
//     固定测试内容、保存凭据；显示 PASS save。目标文件保留供人工检查。
//  4. Close access：LocalPath/Credential 返回 os.ErrClosed、再次 Close 成功，
//     Name 保留。Restore saved 可重新获得资源，显示 PASS restored/read。
//  5. 退出并再次启动同一程序（macOS 同一签名 .app），点击 Restore saved：
//     仍能读外部文件/目录，刷新并写回凭据。随后退出，手动移除该测试资源，
//     重启恢复必须显示 EXPECTED restore failure，不能弹选择器或提权。
//  6. Invalid credential：损坏输入被拒绝，显示 PASS invalid credential。
//     Reset credential 删除本例程自己的凭据和日志，其他文件不受影响。
//
// 平台差异：Win/Linux 路径凭据不跟踪重命名；macOS 原生书签可能跟踪移动。
// macOS 沙箱内部配置/缓存应位于应用容器；读写外部文件以选择器授权为准。
// 副作用：仅用户选择的 Save 目标以及 UserConfigDir/org.golang-gui.FileAccessTest
// 中的 access.json、results.log；打开资源后退出会显式 Close。不修改系统设置。
// 复位：Reset credential，然后重新启动。退出：Exit 或关闭主窗口。
package main

import (
	"errors"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/layout"
	"github.com/golang-gui/goui/theme/modern"
)

const appID = "org.golang-gui.FileAccessTest"

type probe struct {
	gui.WidgetBase
	logged bool
}

func (p *probe) Paint(painter gui.Painter) {
	if p.logged {
		return
	}
	p.logged = true
	img, err := painter.NewImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	if err != nil {
		panic(err)
	}
	fmt.Printf("backend=%T scale=%g\n", img, painter.PixelScale())
	img.Destroy()
}

func main() {
	runtime.LockOSThread()
	app, err := gui.NewApplication(appID)
	if err != nil {
		log.Fatal(err)
	}
	app.SetStyleSheet(modern.Sheet(modern.Options{}))
	win, err := app.NewWindow(&gui.WindowOptions{Size: geometry.Size{Width: 760, Height: 480}})
	if err != nil {
		log.Fatal(err)
	}
	defer win.Destroy()
	win.SetTitle("GOUI FileAccess verification")
	config, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	cache, cacheErr := os.UserCacheDir()
	state := filepath.Join(config, appID)
	if err := os.MkdirAll(state, 0700); err != nil {
		log.Fatal(err)
	}
	credentialPath := filepath.Join(state, "access.json")
	status := gui.NewLabel("Ready. Use only independent test files.")
	status.SetWrapMode(gui.WrapWordChar)
	report := func(message string) {
		status.SetText(message)
		fmt.Println(message)
		f, err := os.OpenFile(filepath.Join(state, "results.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			log.Print(err)
			return
		}
		fmt.Fprintln(f, message)
		f.Close()
	}
	report(fmt.Sprintf("ENV os=%s HOME=%s config=%s cache=%s cacheError=%v", runtime.GOOS, os.Getenv("HOME"), config, cache, cacheErr))
	var current gui.FileAccess
	closeCurrent := func() {
		if current != nil {
			current.Close()
			current = nil
		}
	}
	read := func(access gui.FileAccess) error {
		path, err := access.LocalPath()
		if err != nil {
			return err
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			items, err := os.ReadDir(path)
			if err != nil {
				return err
			}
			report(fmt.Sprintf("PASS read directory name=%s entries=%d", access.Name(), len(items)))
		} else {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			report(fmt.Sprintf("PASS read file name=%s bytes=%d", access.Name(), len(data)))
		}
		return nil
	}
	persist := func(access gui.FileAccess) error {
		credential, err := access.Credential()
		if err != nil {
			return err
		}
		return os.WriteFile(credentialPath, credential, 0600)
	}
	accept := func(access []gui.FileAccess, err error) bool {
		if err != nil {
			report("FAIL picker: " + err.Error())
			return false
		}
		if len(access) == 0 {
			report("CANCEL")
			return false
		}
		for _, a := range access {
			if err := read(a); err != nil {
				for _, acquired := range access {
					acquired.Close()
				}
				report("FAIL read: " + err.Error())
				return false
			}
		}
		if err := persist(access[0]); err != nil {
			for _, a := range access {
				a.Close()
			}
			report("FAIL credential: " + err.Error())
			return false
		}
		closeCurrent()
		current = access[0]
		for _, a := range access[1:] {
			a.Close()
		}
		report(fmt.Sprintf("PASS selected/read/credential count=%d name=%s", len(access), current.Name()))
		return true
	}
	selected := func(access []gui.FileAccess, err error) { accept(access, err) }
	root := gui.NewLinearBox(layout.DirectionVertical)
	root.SetPadding(20)
	root.SetSpacing(16)
	root.SetCrossAlign(layout.CrossStretch)
	root.AddChild(gui.NewLabel("FileAccess — native file picker and persistent credentials"))
	addRow := func(buttons ...gui.Widget) {
		row := gui.NewLinearBox(layout.DirectionHorizontal)
		row.SetSpacing(8)
		for _, b := range buttons {
			row.AddChild(b)
		}
		root.AddChild(row)
	}
	button := func(text string, action func()) gui.Widget {
		b := gui.NewButton()
		b.SetChild(gui.NewLabel(text))
		b.ConnectClicked(action)
		return b
	}
	dialog := app.FileDialog()
	addRow(button("Open file", func() { dialog.OpenFile(win, gui.DialogOptions{}, selected) }),
		button("Open multiple", func() { dialog.OpenFile(win, gui.DialogOptions{AllowMultiple: true}, selected) }),
		button("Open folder", func() { dialog.OpenDirectory(win, gui.DialogOptions{}, selected) }))
	addRow(button("Save file", func() {
		dialog.SaveFile(win, gui.DialogOptions{DefaultName: "goui-file-access-new.txt"}, func(access []gui.FileAccess, err error) {
			if err != nil || len(access) == 0 {
				selected(access, err)
				return
			}
			a := access[0]
			path, err := a.LocalPath()
			if err == nil {
				if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
					err = fmt.Errorf("choose a NEW target; selection must not create it (stat=%v)", statErr)
				}
			}
			if err == nil {
				err = os.WriteFile(path, []byte("GOUI FileAccess save test\n"), 0600)
			}
			if err != nil {
				for _, a := range access {
					a.Close()
				}
				report("FAIL save: " + err.Error())
				return
			}
			if accept(access, nil) {
				report("PASS save target=" + path)
			}
		})
	}), button("Close access", func() {
		if current == nil {
			report("No open access")
			return
		}
		name := current.Name()
		err := current.Close()
		_, pathErr := current.LocalPath()
		_, credentialErr := current.Credential()
		if err != nil || current.Close() != nil || !errors.Is(pathErr, os.ErrClosed) || !errors.Is(credentialErr, os.ErrClosed) || current.Name() != name {
			panic("Close contract violated")
		}
		current = nil
		report("PASS closed, repeated close and retained name")
	}))
	addRow(button("Restore saved", func() {
		data, err := os.ReadFile(credentialPath)
		if err != nil {
			report("EXPECTED restore failure: " + err.Error())
			return
		}
		access, err := dialog.RestoreAccess(data)
		if err != nil {
			report("EXPECTED restore failure: " + err.Error())
			return
		}
		if accept([]gui.FileAccess{access}, nil) {
			report("PASS restored/read")
		}
	}), button("Invalid credential", func() {
		access, err := dialog.RestoreAccess([]byte("invalid credential"))
		if access != nil || !errors.Is(err, os.ErrInvalid) {
			panic(fmt.Sprintf("invalid accepted: %v %v", access, err))
		}
		report("PASS invalid credential")
	}), button("Reset credential", func() {
		closeCurrent()
		for _, path := range []string{credentialPath, filepath.Join(state, "results.log")} {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				report("FAIL reset: " + err.Error())
				return
			}
		}
		report("PASS reset")
	}), button("Exit", win.Destroy))
	root.AddChild(status)
	root.AddChild(gui.NewLabel("Close and relaunch, then Restore saved to verify cross-process access."))
	root.AddChild(&probe{})
	win.SetWidget(root)
	win.ConnectDestroy(func() { closeCurrent(); app.Quit() })
	if err := win.Show(); err != nil {
		log.Fatal(err)
	}
	app.Run()
}
