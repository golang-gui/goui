package gui

import (
	"errors"
	"fmt"
	"slices"

	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/typography"
)

type Application interface {
	Platform() platform.Platform
	Typography() typography.Context
	// Clipboard returns the system clipboard, or nil if it is unavailable.
	Clipboard() platform.Clipboard
	NewWindow() (Window, error)
	Run()
	Quit()
	Post(func())
	Windows() []Window
	Snapshot() ApplicationInfo
	DispatchWindowEvent(windowID string, event events.Event) error
}

var (
	App       Application
	ErrAppNil = errors.New("application is not created")
)

func NewApplication() (Application, error) {
	if App != nil {
		return App, nil
	}

	app, err := newApplication()
	if err != nil {
		return nil, err
	}
	App = app
	return app, nil
}

type application struct {
	platform  platform.Platform
	loop      platform.EventLoop
	typo      typography.Context
	clipboard platform.Clipboard
	windows   []*window
}

func newApplication() (*application, error) {
	plat, err := platform.NewPlatform(platform.DefaultName())
	if err != nil {
		return nil, fmt.Errorf("create platform: %w", err)
	}

	loop, err := plat.NewEventLoop()
	if err != nil {
		plat.Destroy()
		return nil, fmt.Errorf("create event loop: %w", err)
	}

	typo, err := plat.NewTypography()
	if err != nil {
		loop.Destroy()
		plat.Destroy()
		return nil, fmt.Errorf("create typography: %w", err)
	}

	// Clipboard is optional: unlike typography, the application remains usable
	// without one, so a creation failure is non-fatal — keep it nil and carry on.
	// TODO: log the error once the framework has logging.
	clip, clipErr := plat.NewClipboard()
	if clipErr != nil {
		clip = nil
	}

	return &application{
		platform:  plat,
		loop:      loop,
		typo:      typo,
		clipboard: clip,
	}, nil
}

func (a *application) Platform() platform.Platform {
	return a.platform
}

func (a *application) Typography() typography.Context {
	return a.typo
}

func (a *application) Clipboard() platform.Clipboard {
	return a.clipboard
}

func (a *application) NewWindow() (Window, error) {
	win, err := newWindow(a)
	if err != nil {
		return nil, err
	}
	a.windows = append(a.windows, win)
	return win, nil
}

func (a *application) Run() {
	a.loop.Run()
}

func (a *application) Quit() {
	a.loop.Quit()
}

func (a *application) Post(task func()) {
	a.loop.Post(task)
}

func (a *application) Windows() []Window {
	windows := make([]Window, 0, len(a.windows))
	for _, win := range a.windows {
		windows = append(windows, win)
	}
	return windows
}

func (a *application) Snapshot() ApplicationInfo {
	info := ApplicationInfo{
		Windows: make([]WindowInfo, 0, len(a.windows)),
	}
	for _, win := range a.windows {
		info.Windows = append(info.Windows, win.Snapshot())
	}
	return info
}

func (a *application) DispatchWindowEvent(windowID string, event events.Event) error {
	for _, win := range a.windows {
		if win.ID() == windowID {
			return win.DispatchEvent(event)
		}
	}
	return fmt.Errorf("window %q not found", windowID)
}

func (a *application) removeWindow(win *window) {
	index := slices.Index(a.windows, win)
	if index >= 0 {
		a.windows = slices.Delete(a.windows, index, index+1)
	}
}
