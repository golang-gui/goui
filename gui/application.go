package gui

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
)

type Application interface {
	Platform() platform.Platform
	Typography() typography.Context
	// Clipboard returns the system clipboard, or nil if it is unavailable.
	Clipboard() Clipboard
	// Settings returns system settings as usable values. Never nil.
	Settings() Settings
	// FileDialog returns the system file dialog, or nil if it is unavailable.
	FileDialog() FileDialog
	// OpenURL asks the system to open an absolute URL with its registered handler.
	// Call on the GUI thread. Errors are returned without logging or displaying
	// a dialog. Success means request acceptance, not external app completion.
	OpenURL(rawURL string) error
	// OpenPath asks the default application to open a file or directory. Relative
	// paths use the working directory; no shell expansion is performed. Like
	// OpenURL, it runs on the GUI thread and returns the native request result.
	OpenPath(path string) error
	StyleSheet() style.StyleSheet
	SetStyleSheet(style.StyleSheet)
	NewWindow(options *WindowOptions) (Window, error)
	// NewTimer creates an inactive timer permanently bound to this application.
	// Creation does not register a task or allocate waiting resources. Call on
	// the GUI thread; the timer can be started before Run, but not after Quit.
	NewTimer() *Timer
	// Run processes native events and GUI timers on the owning thread. Returning
	// from Run stops timer scheduling and waits for its background waiter to exit.
	Run()
	// Quit stops timer scheduling and requests event-loop exit. It may be called
	// from any goroutine; an already-started timer signal may finish.
	Quit()
	// QuitOnLastWindowClosed reports whether the app quits when its last window
	// is closed (default true).
	QuitOnLastWindowClosed() bool
	SetQuitOnLastWindowClosed(bool)
	Post(func())
	Windows() []Window
	Snapshot() ApplicationInfo
	DispatchWindowEvent(windowID string, event events.Event) error
}

var (
	App       Application
	ErrAppNil = errors.New("application is not created")
)

// NewApplication creates the application with a stable, case-sensitive identity
// such as "com.example.Editor". Empty leaves identity unspecified. The ID is
// immutable and distinct from window IDs, titles and display names. Supplying an
// ID does not install desktop entries, icons or an application bundle.
func NewApplication(appId string) (Application, error) {
	if App != nil {
		return App, nil
	}

	app, err := newApplication(appId)
	if err != nil {
		return nil, err
	}
	App = app
	return app, nil
}

type application struct {
	platform     platform.Platform
	loop         platform.EventLoop
	timers       *timerScheduler
	typo         typography.Context
	clipboard    Clipboard
	settings     *settings
	fileDialog   FileDialog
	style        style.StyleSheet
	styleChanged signal.Signal0 // internal host invalidation; no new public event API
	windows      []*window
	dragSession  *guiDragSession
	nextDragID  uint64

	quitOnLastWindowClosed bool
}

func newApplication(appId string) (*application, error) {
	plat, err := platform.NewPlatform(platform.DefaultName(), appId)
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
	platClip, clipErr := plat.NewClipboard()
	if clipErr != nil {
		// TODO: log the error once the framework has logging.
	}

	platSettings, settingsErr := plat.NewSettings()
	if settingsErr != nil {
		platSettings = nil // getters then always fall back
	}

	platFileDlg, _ := plat.NewFileDialog()

	app := &application{
		platform:   plat,
		loop:       loop,
		timers:     newTimerScheduler(loop.Post, time.Now),
		typo:       typo,
		clipboard:  newClipboard(platClip),
		settings:   newSettings(platSettings),
		fileDialog: newFileDialog(platFileDlg, loop),

		quitOnLastWindowClosed: true,
	}
	return app, nil
}

func (a *application) Platform() platform.Platform {
	return a.platform
}

func (a *application) Typography() typography.Context {
	return a.typo
}

func (a *application) Clipboard() Clipboard {
	return a.clipboard
}

func (a *application) Settings() Settings {
	return a.settings
}

// FileDialog returns the system file dialog, or nil if it is unavailable.
func (a *application) FileDialog() FileDialog {
	return a.fileDialog
}

func (a *application) OpenURL(rawURL string) error {
	return a.platform.OpenURL(rawURL)
}

func (a *application) OpenPath(path string) error {
	return a.platform.OpenPath(path)
}

// StyleSheet is the app's custom style sheet, or nil when none is set.
func (a *application) StyleSheet() style.StyleSheet {
	return a.style
}

func (a *application) SetStyleSheet(sheet style.StyleSheet) {
	a.style = sheet
	for _, win := range a.windows {
		invalidateStyleSubtree(win.Widget())
		if win.controls != nil {
			invalidateStyleSubtree(win.controls)
		}
		win.requestLayout()
	}
	a.styleChanged.Emit()
}

func (a *application) NewWindow(options *WindowOptions) (Window, error) {
	opt := defaultWindowOptions
	if options != nil {
		opt = *options
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	win, err := newWindow(a, opt.normalized())
	if err != nil {
		return nil, err
	}
	a.windows = append(a.windows, win)
	return win, nil
}

func (a *application) NewTimer() *Timer {
	return &Timer{app: a}
}

func (a *application) startTimer(t *Timer, interval time.Duration, once bool) error {
	// The scheduler receives only a callback. Binding it to the public Timer's
	// signal, and keeping its task handle, are application responsibilities.
	run, err := a.timers.schedule(t.run, interval, once, t.timeout.Emit)
	if err != nil {
		return err
	}
	t.run = run
	return nil
}

func (a *application) stopTimer(t *Timer) {
	a.timers.cancel(t.run)
	t.run = nil
}

func (a *application) timerActive(t *Timer) bool {
	return a.timers.active(t.run)
}

func (a *application) Run() {
	if a.timers != nil {
		defer a.timers.finish()
	}
	stop := a.settings.watch(a)
	defer stop()
	if a.timers != nil {
		a.timers.start()
	}
	a.loop.Run()
}

func (a *application) Quit() {
	if a.timers != nil {
		a.timers.close()
	}
	a.loop.Quit()
}

func (a *application) QuitOnLastWindowClosed() bool {
	return a.quitOnLastWindowClosed
}

func (a *application) SetQuitOnLastWindowClosed(v bool) {
	a.quitOnLastWindowClosed = v
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
	if a.quitOnLastWindowClosed && len(a.windows) == 0 {
		a.Quit()
	}
}
