// Package portal implements Linux desktop portal requests without a GUI event loop.
package portal

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/golang-gui/goui/platform/common"
)

const (
	service       = "org.freedesktop.portal.Desktop"
	desktopPath   = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	requestPrefix = "/org/freedesktop/portal/desktop/request/"
	chooser       = "org.freedesktop.portal.FileChooser"
	request       = "org.freedesktop.portal.Request"
	startupLimit  = 10 * time.Second
	closeLimit    = time.Second
)

// ErrUnsupported permits a helper fallback only before a picker was accepted.
var ErrUnsupported = errors.New("desktop portal file chooser unavailable")

// Action selects the native file-chooser operation.
type Action uint8

const (
	OpenFile Action = iota
	OpenDirectory
	SaveFile
)

// FileChooser owns a lazily opened private session-bus connection. Open may run
// concurrently; Close prevents new work and asynchronously drains cancellation.
// It never accesses a platform window or invokes application callbacks.
type FileChooser struct {
	mutex      sync.Mutex
	conn       *dbus.Conn
	connecting chan struct{}
	closed     bool
	stop       chan struct{}
	active     sync.WaitGroup
}

func NewFileChooser() *FileChooser { return &FileChooser{stop: make(chan struct{})} }

func (f *FileChooser) Close() {
	f.mutex.Lock()
	if f.closed {
		f.mutex.Unlock()
		return
	}
	f.closed = true
	close(f.stop)
	f.mutex.Unlock()
	go func() {
		f.active.Wait()
		f.mutex.Lock()
		conn := f.conn
		f.conn = nil
		f.mutex.Unlock()
		if conn != nil {
			_ = conn.Close()
		}
	}()
}

func (f *FileChooser) connection(ctx context.Context) (*dbus.Conn, error) {
	for {
		f.mutex.Lock()
		if f.closed {
			f.mutex.Unlock()
			return nil, common.ErrUnavailable
		}
		if f.conn != nil && f.conn.Connected() {
			conn := f.conn
			f.mutex.Unlock()
			return conn, nil
		}
		if ready := f.connecting; ready != nil {
			f.mutex.Unlock()
			select {
			case <-ready:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-f.stop:
				return nil, common.ErrUnavailable
			}
		}
		f.connecting = make(chan struct{})
		f.mutex.Unlock()

		conn, err := connect()
		f.mutex.Lock()
		closed := f.closed
		if err == nil && !closed {
			f.conn = conn
		}
		close(f.connecting)
		f.connecting = nil
		f.mutex.Unlock()
		if closed {
			if conn != nil {
				_ = conn.Close()
			}
			return nil, common.ErrUnavailable
		}
		return conn, err
	}
}

func connect() (*dbus.Conn, error) {
	conn, err := dbus.SessionBusPrivateNoAutoStartup()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnsupported, err)
	}
	// Auth has no context API. Closing the private transport also bounds it.
	timer := time.AfterFunc(startupLimit, func() { _ = conn.Close() })
	err = conn.Auth(nil)
	if err == nil {
		err = conn.Hello()
	}
	ctx, cancel := context.WithTimeout(context.Background(), startupLimit)
	defer cancel()
	if err == nil {
		err = conn.AddMatchSignalContext(ctx, dbus.WithMatchSender(service),
			dbus.WithMatchInterface(request), dbus.WithMatchMember("Response"),
			dbus.WithMatchPathNamespace(dbus.ObjectPath(strings.TrimSuffix(requestPrefix, "/"))))
	}
	if err == nil {
		err = conn.AddMatchSignalContext(ctx, dbus.WithMatchSender("org.freedesktop.DBus"),
			dbus.WithMatchInterface("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged"),
			dbus.WithMatchArg(0, service))
	}
	if !timer.Stop() && err == nil {
		err = context.DeadlineExceeded
	}
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("%w: connect: %v", ErrUnsupported, err)
	}
	return conn, nil
}

// Open waits for a selection or cancellation. Call it outside the platform thread.
// opts and its slices must belong to this call. A cancelled operation closes its
// remote request, including a handle returned after local cancellation.
func (f *FileChooser) Open(ctx context.Context, action Action, parent string, opts common.DialogOptions) ([]string, error) {
	f.mutex.Lock()
	if f.closed {
		f.mutex.Unlock()
		return nil, common.ErrUnavailable
	}
	f.active.Add(1)
	f.mutex.Unlock()
	defer f.active.Done()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	method, options, err := parameters(action, parent, opts)
	if err != nil {
		return nil, err
	}
	conn, err := f.connection(ctx)
	if err != nil {
		return nil, err
	}
	signals := make(chan *dbus.Signal, 16)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)
	startup, cancel := context.WithTimeout(ctx, startupLimit)
	owner, err := portalOwner(startup, conn)
	if err == nil {
		var version dbus.Variant
		err = conn.Object(owner, desktopPath).CallWithContext(startup,
			"org.freedesktop.DBus.Properties.Get", 0, chooser, "version").Store(&version)
		if err == nil {
			v, ok := version.Value().(uint32)
			if !ok || v == 0 {
				err = errors.New("invalid FileChooser version")
			} else if action == OpenDirectory && v < 3 {
				err = fmt.Errorf("%w: directory selection requires FileChooser version 3", ErrUnsupported)
			}
		}
	}
	cancel()
	if err != nil {
		if unsupported(err) {
			return nil, fmt.Errorf("%w: %v", ErrUnsupported, err)
		}
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case <-f.stop:
		return nil, common.ErrUnavailable
	default:
	}
	token := "goui_" + rand.Text()
	options["handle_token"] = dbus.MakeVariant(token)
	expected := dbus.ObjectPath(requestPrefix + strings.ReplaceAll(strings.TrimPrefix(conn.Names()[0], ":"), ".", "_") + "/" + token)
	// The method deadline bounds startup, not the user's interaction time. It is
	// intentionally independent of ctx so cancellation can still recover its handle.
	startup, cancel = context.WithTimeout(context.Background(), startupLimit)
	defer cancel()
	call := conn.Object(owner, desktopPath).GoWithContext(startup, chooser+"."+method, 0, nil, parent, opts.Title, options)
	var handle dbus.ObjectPath
	early := make(map[dbus.ObjectPath]*dbus.Signal)
	for {
		select {
		case reply := <-call.Done:
			if err := reply.Store(&handle); err != nil {
				closeRequest(conn, owner, expected)
				if unsupported(err) {
					return nil, fmt.Errorf("%w: %v", ErrUnsupported, err)
				}
				return nil, fmt.Errorf("portal file chooser: %w", err)
			}
			cancel()
			if !handle.IsValid() || !strings.HasPrefix(string(handle), requestPrefix) {
				closeRequest(conn, owner, expected)
				return nil, errors.New("portal returned an invalid request path")
			}
			if signal := early[handle]; signal != nil {
				return response(action, opts.AllowMultiple, signal.Body)
			}
			early = nil
		case signal, ok := <-signals:
			if !ok {
				return nil, errors.New("portal connection closed")
			}
			if signal.Name == "org.freedesktop.DBus.NameOwnerChanged" && len(signal.Body) == 3 && signal.Body[1] == owner {
				return nil, errors.New("portal service disappeared")
			}
			if signal.Sender != owner || signal.Name != request+".Response" {
				continue
			}
			if handle != "" && signal.Path == handle {
				return response(action, opts.AllowMultiple, signal.Body)
			}
			if early != nil {
				early[signal.Path] = signal
			}
		case <-ctx.Done():
			cancelRequest(conn, owner, expected, handle, call)
			return nil, ctx.Err()
		case <-f.stop:
			cancelRequest(conn, owner, expected, handle, call)
			return nil, common.ErrUnavailable
		case <-conn.Context().Done():
			return nil, errors.New("portal connection closed")
		}
	}
}

func cancelRequest(conn *dbus.Conn, owner string, expected, handle dbus.ObjectPath, call *dbus.Call) {
	if handle != "" {
		closeRequest(conn, owner, handle)
		return
	}
	closeRequest(conn, owner, expected)
	// Go cancellation does not dismiss a remote dialog. Recover the actual handle
	// while the bounded startup call completes, then close even if it arrived late.
	reply := <-call.Done
	if reply.Store(&handle) == nil && handle.IsValid() {
		closeRequest(conn, owner, handle)
	}
}

func closeRequest(conn *dbus.Conn, owner string, path dbus.ObjectPath) {
	ctx, cancel := context.WithTimeout(context.Background(), closeLimit)
	defer cancel()
	_ = conn.Object(owner, path).CallWithContext(ctx, request+".Close", 0).Err
}

func portalOwner(ctx context.Context, conn *dbus.Conn) (string, error) {
	var owner string
	err := conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.GetNameOwner", 0, service).Store(&owner)
	if err != nil && unsupported(err) {
		if err = conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.StartServiceByName", 0, service, uint32(0)).Err; err == nil {
			err = conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.GetNameOwner", 0, service).Store(&owner)
		}
	}
	return owner, err
}

func unsupported(err error) bool {
	var native dbus.Error
	if !errors.As(err, &native) {
		return false
	}
	switch native.Name {
	case "org.freedesktop.DBus.Error.ServiceUnknown", "org.freedesktop.DBus.Error.NameHasNoOwner",
		"org.freedesktop.DBus.Error.UnknownInterface", "org.freedesktop.DBus.Error.UnknownMethod",
		"org.freedesktop.DBus.Error.UnknownProperty":
		return true
	}
	return false
}

type filter struct {
	Name       string
	Conditions []condition
}

type condition struct {
	Kind    uint32
	Pattern string
}

func parameters(action Action, parent string, opts common.DialogOptions) (string, map[string]dbus.Variant, error) {
	if strings.ContainsRune(opts.InitialDir, 0) || strings.ContainsRune(opts.DefaultName, 0) {
		return "", nil, errors.New("file dialog option contains NUL")
	}
	method := "OpenFile"
	options := map[string]dbus.Variant{"modal": dbus.MakeVariant(parent != "")}
	switch action {
	case OpenFile:
		options["multiple"] = dbus.MakeVariant(opts.AllowMultiple)
		options["directory"] = dbus.MakeVariant(false)
	case OpenDirectory:
		options["multiple"] = dbus.MakeVariant(false)
		options["directory"] = dbus.MakeVariant(true)
	case SaveFile:
		method = "SaveFile"
		if opts.DefaultName != "" {
			options["current_name"] = dbus.MakeVariant(opts.DefaultName)
		}
	default:
		return "", nil, errors.New("unknown file dialog action")
	}
	if opts.InitialDir != "" {
		options["current_folder"] = dbus.MakeVariant(append([]byte(opts.InitialDir), 0))
	}
	if len(opts.Filters) > 0 && action != OpenDirectory {
		filters := make([]filter, 0, len(opts.Filters))
		for _, item := range opts.Filters {
			entry := filter{Name: item.Name, Conditions: make([]condition, 0, len(item.Patterns))}
			for _, pattern := range item.Patterns {
				entry.Conditions = append(entry.Conditions, condition{Pattern: pattern})
			}
			filters = append(filters, entry)
		}
		options["filters"] = dbus.MakeVariant(filters)
	}
	return method, options, nil
}

func response(action Action, multiple bool, body []any) ([]string, error) {
	var code uint32
	var results map[string]dbus.Variant
	if err := dbus.Store(body, &code, &results); err != nil {
		return nil, fmt.Errorf("invalid portal response: %w", err)
	}
	if code == 1 {
		return nil, nil
	}
	if code != 0 {
		return nil, fmt.Errorf("portal file chooser failed (response %d)", code)
	}
	var uris []string
	value, ok := results["uris"]
	if !ok || value.Store(&uris) != nil || len(uris) == 0 || ((action != OpenFile || !multiple) && len(uris) != 1) {
		return nil, errors.New("portal returned an invalid file selection")
	}
	paths := make([]string, 0, len(uris))
	for _, uri := range uris {
		u, err := url.Parse(uri)
		if err != nil || u.Scheme != "file" || (u.Host != "" && u.Host != "localhost") || u.User != nil ||
			u.Opaque != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(uri, "#") ||
			!filepath.IsAbs(u.Path) || strings.ContainsRune(u.Path, 0) {
			return nil, fmt.Errorf("invalid local file URI %q", uri)
		}
		paths = append(paths, u.Path)
	}
	return paths, nil
}
