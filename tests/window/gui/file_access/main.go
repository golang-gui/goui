// File-access credential window checks using public GUI APIs, without DevServer.
//
// Environment: Linux X11 with zenity/kdialog/yad, or a Windows/macOS desktop.
// Run: go run ./tests/window/gui/file_access. For macOS sandbox checks, use
// cmd/goui to build, package and sign an .app, then launch through Finder/open;
// go run does not exercise the application sandbox.
// Initially no resources are open. Logs show HOME, UserConfigDir, UserCacheDir,
// actual backend and scale. Prepare a separate temporary directory outside the
// app with a.txt and b.txt containing arbitrary data; keep at least one entry.
// Use disposable files and save targets, avoiding existing application data.
// Steps and expected results:
//  1. Cancel Open file: CANCEL without error. Select a.txt: PASS selected/read/credential,
//     readable content, saved credential and a filename-only Name.
//  2. Open multiple selects two readable files and shows count=2. Open folder
//     reports entries and a credential for the directory itself.
//  3. Save file to a nonexistent target: selection alone creates nothing; the
//     program then writes fixed content, saves a credential and shows PASS save.
//     The file remains for inspection.
//  4. Close access: LocalPath/Credential return os.ErrClosed, repeated Close
//     succeeds and Name remains. Restore saved reacquires access and shows
//     PASS restored/read.
//  5. Restart the same program (same signed .app on macOS), Restore saved and
//     read the external file/directory again, refreshing and saving credentials.
//     Exit, remove that disposable resource, restart and restore: EXPECTED restore
//     failure, without a picker or privilege elevation.
//  6. Invalid credential rejects corrupt input with PASS invalid credential.
//     Reset credential removes only this example's credentials and log.
//
// Platform differences: Windows/Linux path credentials do not track renames;
// native macOS bookmarks may track moves. In a macOS sandbox, internal config/cache
// belong in the app container; external access follows picker authorization.
// Side effects: the chosen Save target and access.json/results.log under
// UserConfigDir/org.golang-gui.FileAccessTest. Open accesses are closed on exit;
// no system settings are changed. Reset: Reset credential, then restart.
// Exit: Exit or close the main window.
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
