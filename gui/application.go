package gui

import (
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"time"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/internal/diagnostic"
	"github.com/golang-gui/goui/platform"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/style"
	"github.com/xuges/gothread"
)

type Application interface {
	Platform() platform.Platform
	// Typography is ready during Activate and subsequent UI callbacks. During
	// mobile preparation it is nil; desktop pre-Run creation remains supported
	// while callers migrate to ConnectActivate.
	Typography() typography.Context
	// Clipboard returns a clipboard view; unavailable access reports an error.
	Clipboard() Clipboard
	// Settings returns system settings as usable values. Never nil.
	Settings() Settings
	// FileDialog returns the system file service (never nil). Unavailable
	// services report the native creation error when an operation is requested.
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
	// NewSurface adopts the system-provided main host after Activate. It may
	// precede native geometry or a drawable. Only one live system Surface is
	// supported; backends without WindowSource return ErrUnsupported.
	NewSurface() (Surface, error)
	NewWindow(options *WindowOptions) (Window, error)
	// NewTimer creates an inactive timer permanently bound to this application.
	// Creation does not register a task or allocate waiting resources. Call on
	// the GUI thread; the timer can be started before Run, but not after Quit.
	NewTimer() *Timer
	// ConnectActivate runs fn on the native UI thread after application services
	// are ready. Connect before Run. The initial activation is asynchronous;
	// focus changes and drawable replacement do not activate the application.
	ConnectActivate(func()) signal.Handle
	// ConnectShutdown runs once on the UI thread before event-loop exit. Release
	// UI-owned connections and resources here. It is not a cancelable request.
	ConnectShutdown(func()) signal.Handle
	// Run starts UI dispatch and waits until shutdown callbacks have completed.
	// Call from the startup goroutine used by NewApplication. On mobile this
	// goroutine waits while native UI callbacks execute elsewhere. Initialization
	// errors are returned without activating the application. Run is one-shot.
	Run() error
	// Quit stops timer scheduling and requests event-loop exit. It may be called
	// from any goroutine; an already-started timer signal may finish.
	Quit()
	// QuitOnLastWindowClosed reports whether the app quits when its last window
	// is closed (default true).
	QuitOnLastWindowClosed() bool
	SetQuitOnLastWindowClosed(bool)
	Post(func())
	// Surfaces returns borrowed live top-level hosts, including managed windows.
	Surfaces() []Surface
	Windows() []Window
	Snapshot() ApplicationInfo
}

var (
	App            Application
	ErrAppNil      = errors.New("application is not created")
	ErrAppNotReady = errors.New("gui: application services are not ready")
	ErrAppRunOnce  = errors.New("gui: application Run may be called only once")
)

// NewApplication creates the application with a stable, case-sensitive identity
// such as "com.example.Editor". Empty leaves identity unspecified. The ID is
// immutable and distinct from window IDs, titles and display names. Supplying an
// ID does not install desktop entries, icons or an application bundle.
// Call from main, connect Activate, then Run. Desktop main keeps the startup
// thread; mobile main only prepares startup. Runtime GUI work belongs to the
// native UI thread delivered by Activate. Background work uses Post.
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
	platform      platform.Platform
	loop          platform.EventLoop
	timers        *timerScheduler
	typo          typography.Context
	clipboard     Clipboard
	settings      *settings
	fileDialog    FileDialog
	style         style.StyleSheet
	styleChanged  signal.Signal0 // internal host invalidation; no new public event API
	windows       []*window
	systemSurface *surface
	windowSource  platform.WindowSource
	dragSession   *guiDragSession
	nextDragID    uint64

	quitOnLastWindowClosed bool
	activateSignal         signal.Signal0
	shutdownSignal         signal.Signal0
	servicesReady          bool
	shutdownDone           bool
	ran                    bool
	quitting               atomic.Bool
	uiThread               int
	runError               error
	stopSettings           func()
}

func newApplication(appId string) (*application, error) {
	plat, err := platform.NewPlatform(platform.DefaultName(), appId)
	if err != nil {
		return nil, fmt.Errorf("create platform: %w", err)
	}

	return prepareApplication(plat)
}

// prepareApplication keeps startup preparation separate from UI-owned services.
func prepareApplication(plat platform.Platform) (*application, error) {
	loop, err := plat.NewEventLoop()
	if err != nil {
		plat.Destroy()
		return nil, fmt.Errorf("create event loop: %w", err)
	}

	app := &application{
		platform: plat, loop: loop,
		timers:   newTimerScheduler(loop.Post, time.Now),
		settings: newSettings(nil), clipboard: newClipboard(nil),
		fileDialog:             newFileDialog(nil, loop, ErrAppNotReady),
		quitOnLastWindowClosed: true,
	}
	app.windowSource, _ = plat.(platform.WindowSource)
	if app.windowSource == nil {
		app.uiThread = gothread.GetId()
		if err := app.initializeServices(); err != nil {
			loop.Destroy()
			plat.Destroy()
			return nil, err
		}
	}
	// This is the first queued application task, ahead of pre-Run Post calls.
	loop.Post(app.activate)
	return app, nil
}

func (a *application) initializeServices() error {
	if a.servicesReady {
		return nil
	}
	typo, err := a.platform.NewTypography()
	if err != nil {
		return fmt.Errorf("create typography: %w", err)
	}
	a.typo = typo
	clip, _ := a.platform.NewClipboard()
	a.clipboard = newClipboard(clip)
	nativeSettings, err := a.platform.NewSettings()
	if err != nil {
		nativeSettings = nil
	}
	a.settings.settings = nativeSettings
	a.settings.current = a.settings.snapshot()
	dialog, dialogErr := a.platform.NewFileDialog()
	a.fileDialog = newFileDialog(dialog, a.loop, dialogErr)
	a.servicesReady = true
	return nil
}

func (a *application) activate() {
	if a.quitting.Load() {
		return
	}
	a.uiThread = gothread.GetId()
	if err := a.initializeServices(); err != nil {
		a.runError = err
		a.Quit()
		return
	}
	if a.windowSource != nil {
		a.systemSurface = newSystemSurface(a)
		if err := a.windowSource.SetWindowHandler(a.acceptSystemWindow); err != nil {
			a.runError = fmt.Errorf("receive system surface: %w", err)
			a.Quit()
			return
		}
	}
	a.stopSettings = a.settings.watch(a)
	a.activateSignal.Emit()
}

func (a *application) ConnectActivate(fn func()) signal.Handle {
	if fn == nil || a.quitting.Load() {
		return signal.Handles(nil)
	}
	return a.activateSignal.Connect(func() {
		if !a.quitting.Load() {
			fn()
		}
	})
}

func (a *application) ConnectShutdown(fn func()) signal.Handle {
	if fn == nil || a.shutdownDone {
		return signal.Handles(nil)
	}
	return a.shutdownSignal.Connect(fn)
}

func (a *application) shutdown() {
	if a.shutdownDone {
		return
	}
	a.shutdownDone = true
	a.quitting.Store(true)
	if a.stopSettings != nil {
		a.stopSettings()
		a.stopSettings = nil
	}
	a.shutdownSignal.Emit()
	if a.systemSurface != nil {
		a.systemSurface.Destroy()
	}
	if a.windowSource != nil {
		_ = a.windowSource.SetWindowHandler(nil)
	}
	if dialog, ok := a.fileDialog.(*fileDialog); ok {
		dialog.close()
	}
	diagnostic.StopOwner(a)
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

// FileDialog returns the file dialog service, including native failure results.
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
	for _, host := range a.Surfaces() {
		invalidateStyleSubtree(host.Widget())
		if win, ok := host.(*window); ok && win.controls != nil {
			invalidateStyleSubtree(win.controls)
		}
		host.RequestLayout()
	}
	a.styleChanged.Emit()
}

func (a *application) NewWindow(options *WindowOptions) (Window, error) {
	if a.quitting.Load() {
		return nil, platform.ErrUnavailable
	}
	if !a.servicesReady {
		return nil, ErrAppNotReady
	}
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

func (a *application) Run() error {
	if a.ran {
		return ErrAppRunOnce
	}
	a.ran = true
	defer a.quitting.Store(true)
	if a.timers != nil {
		defer a.timers.finish()
		a.timers.start()
	}
	a.loop.Run()
	if !a.shutdownDone {
		if a.uiThread != gothread.GetId() {
			return errors.Join(a.runError, errors.New("gui: native loop exited without UI-thread shutdown"))
		}
		// Desktop native exit is already on the owning UI thread.
		a.shutdown()
	}
	return a.runError
}

func (a *application) Quit() {
	if a.quitting.Swap(true) {
		return
	}
	if a.timers != nil {
		a.timers.close()
	}
	a.loop.Post(func() {
		a.shutdown()
		a.loop.Quit()
	})
}

func (a *application) QuitOnLastWindowClosed() bool {
	return a.quitOnLastWindowClosed
}

func (a *application) SetQuitOnLastWindowClosed(v bool) {
	a.quitOnLastWindowClosed = v
}

func (a *application) Post(task func()) {
	if task != nil && !a.quitting.Load() {
		a.loop.Post(func() {
			if !a.quitting.Load() {
				task()
			}
		})
	}
}

func (a *application) Windows() []Window {
	windows := make([]Window, 0, len(a.windows))
	for _, win := range a.windows {
		windows = append(windows, win)
	}
	return windows
}

func (a *application) Surfaces() []Surface {
	result := make([]Surface, 0, len(a.windows)+1)
	for _, w := range a.windows {
		if !w.destroyed {
			result = append(result, w)
		}
	}
	if s := a.systemSurface; s != nil && s.claimed && !s.destroyed {
		result = append(result, s)
	}
	return result
}
func (a *application) Snapshot() ApplicationInfo {
	info := ApplicationInfo{Surfaces: make([]SurfaceInfo, 0)}
	for _, s := range a.Surfaces() {
		info.Surfaces = append(info.Surfaces, s.Snapshot())
	}
	return info
}
func (a *application) acceptSystemWindow(native platform.Window) platform.EventHandler {
	s := a.systemSurface
	if s == nil || s.destroyed || a.quitting.Load() {
		native.Destroy()
		return nil
	}
	s.bind(native)
	return s.onEvent
}
func (a *application) NewSurface() (Surface, error) {
	if a.quitting.Load() {
		return nil, platform.ErrUnavailable
	}
	if !a.servicesReady {
		return nil, ErrAppNotReady
	}
	if a.windowSource == nil {
		return nil, platform.ErrUnsupported
	}
	s := a.systemSurface
	if s != nil && s.claimed && !s.destroyed {
		return nil, platform.ErrUnsupported
	}
	if s == nil || s.destroyed {
		s = newSystemSurface(a)
		a.systemSurface = s
		if err := a.windowSource.SetWindowHandler(a.acceptSystemWindow); err != nil {
			a.systemSurface = nil
			return nil, err
		}
	}
	s.claimed = true
	if err := s.ensurePainter(); err != nil {
		s.Destroy()
		return nil, err
	}
	if s.visible {
		s.resumeFrames(s.platformWindow)
	}
	s.RequestLayout()
	return s, nil
}
func (a *application) removeSurface(host Surface) {
	if win, ok := host.(*window); ok {
		a.removeWindow(win)
		return
	}
	if a.systemSurface == host {
		a.systemSurface = nil
		if a.windowSource != nil {
			_ = a.windowSource.SetWindowHandler(nil)
		}
	}
}

func (a *application) removeWindow(win *window) {
	index := slices.Index(a.windows, win)
	if index >= 0 {
		a.windows = slices.Delete(a.windows, index, index+1)
	}
	if a.quitOnLastWindowClosed && len(a.windows) == 0 && (a.systemSurface == nil || !a.systemSurface.claimed || a.systemSurface.destroyed) {
		a.Quit()
	}
}
