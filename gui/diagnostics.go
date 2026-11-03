package gui

import (
	"fmt"
	"sort"
	"time"
	"weak"

	"github.com/golang-gui/goui/core/geometry"
	"github.com/golang-gui/goui/internal/diagnostic"
)

const (
	diagnosticRoots   = 64
	diagnosticWidgets = 4096
)

// diagnosticCollector is the GUI's private weak-reference adapter. Control,
// history and public reporting live outside gui; this adapter knows private GUI
// identities and cached metadata. Source queries never call user/native APIs.
type diagnosticCollector struct {
	session                      *diagnostic.Session
	app                          *application
	roots                        map[weak.Pointer[rootBase]]*diagnostic.RootRecord
	widgets                      map[weak.Pointer[WidgetBase]]*diagnostic.Widget
	droppedRoots, droppedWidgets uint64
}

func diagnosticCollectorFor(s *diagnostic.Session) *diagnosticCollector {
	if d, ok := s.Source().(*diagnosticCollector); ok {
		return d
	}
	app, ok := s.Owner().(*application)
	if !ok || app == nil {
		return nil
	}
	d := &diagnosticCollector{session: s, app: app, roots: make(map[weak.Pointer[rootBase]]*diagnostic.RootRecord)}
	if s.WidgetsEnabled() {
		d.widgets = make(map[weak.Pointer[WidgetBase]]*diagnostic.Widget)
	}
	s.SetSource(d)
	return d
}

// Keep the disabled gate small enough to inline. No map, weak reference, source
// dispatch or clock is touched until widget collection is explicitly enabled.
func diagnosticWidget(w *WidgetBase) *diagnostic.Widget {
	if s := diagnostic.Current(); s != nil && s.WidgetsEnabled() {
		return diagnosticWidgetFor(s, w)
	}
	return nil
}

func diagnosticWidgetFor(s *diagnostic.Session, w *WidgetBase) *diagnostic.Widget {
	if d := diagnosticCollectorFor(s); d != nil {
		return d.widget(w)
	}
	return nil
}

func diagnosticRoot(b *rootBase) *diagnostic.RootRecord {
	if s := diagnostic.Current(); s != nil && s.Owner() == b.app {
		if d := diagnosticCollectorFor(s); d != nil {
			return d.root(b)
		}
	}
	return nil
}

func (d *diagnosticCollector) root(b *rootBase) *diagnostic.RootRecord {
	if b.app != d.app {
		return nil
	}
	key := weak.Make(b)
	if r := d.roots[key]; r != nil {
		return r
	}
	if len(d.roots) == diagnosticRoots {
		d.droppedRoots++
		return nil
	}
	r := &diagnostic.RootRecord{Root: diagnostic.Root{Ref: d.session.NextRef()}}
	d.roots[key] = r
	return r
}

func (d *diagnosticCollector) widget(w *WidgetBase) *diagnostic.Widget {
	if !d.session.WidgetsEnabled() {
		return nil
	}
	key := weak.Make(w)
	if s := d.widgets[key]; s != nil {
		return s
	}
	if len(d.widgets) == diagnosticWidgets {
		d.droppedWidgets++
		return nil
	}
	s := &diagnostic.Widget{Ref: d.session.NextRef()}
	d.widgets[key] = s
	return s
}

func (b *rootBase) beginDiagnosticFrame() *diagnostic.FrameRecord {
	if s := diagnostic.Current(); s != nil && s.Owner() == b.app {
		if d := diagnosticCollectorFor(s); d != nil {
			if r := d.root(b); r != nil {
				if b.painter != nil {
					r.Backend = b.painter.Name()
				}
				if r.Kind == "" {
					d.rootMetadata(b, r)
				}
				return s.BeginFrame(r)
			}
		}
	}
	return nil
}

// Metadata reads cached GUI values, never native title/position APIs.
func (d *diagnosticCollector) rootMetadata(b *rootBase, r *diagnostic.RootRecord) {
	r.Size = geometry.Size{Width: b.width, Height: b.height}
	for _, w := range d.app.windows {
		if &w.rootBase == b {
			r.Kind, r.ID, r.Title = "window", w.id, w.title
			return
		}
		for _, p := range w.popovers {
			if &p.rootBase == b {
				r.Kind, r.Title = "popover", w.title
				return
			}
		}
	}
}

func (d *diagnosticCollector) Snapshot(now time.Time) diagnostic.GUIData {
	out := diagnostic.GUIData{DroppedRoots: d.droppedRoots, DroppedWidgets: d.droppedWidgets}
	pruneRoots := len(d.roots) == diagnosticRoots
	for key, r := range d.roots {
		b := key.Value()
		closed, suspended := b == nil || b.frameDestroyed, false
		if !closed {
			d.rootMetadata(b, r)
			suspended = b.frameSuspended
		}
		out.Roots = append(out.Roots, r.Snapshot(now, closed, suspended))
		// A query may reclaim closed entries after publishing their final values.
		// Recording at capacity never scans the table or changes live objects.
		if closed && pruneRoots {
			delete(d.roots, key)
		}
	}
	for key, s := range d.widgets {
		w := key.Value()
		if w == nil || w.destroyed {
			delete(d.widgets, key)
			continue
		}
		v := *s
		v.ID, v.Name, v.Bounds = w.id, w.name, w.rect
		host := w.parentRoot
		for p := w.parentWidget; p != nil; p = p.base().parentWidget {
			base := p.base()
			r := base.rect
			v.Bounds.X += r.X
			v.Bounds.Y += r.Y
			if host == nil {
				host = base.parentRoot
			}
		}
		if w.self != nil {
			v.Type = fmt.Sprintf("%T", w.self)
		}
		if owner, ok := host.(interface{ rootState() *rootBase }); ok {
			if r := d.roots[weak.Make(owner.rootState())]; r != nil {
				v.RootRef = r.Ref
			}
		}
		out.Widgets = append(out.Widgets, v)
	}
	sort.Slice(out.Roots, func(i, j int) bool { return out.Roots[i].Ref < out.Roots[j].Ref })
	sort.Slice(out.Widgets, func(i, j int) bool { return out.Widgets[i].Ref < out.Widgets[j].Ref })
	return out
}

// layoutObservation is a private value scope. Hosts mark layout boundaries;
// only rootBase knows whether recording is active or which frame receives time.
type layoutObservation struct {
	frame   *diagnostic.FrameRecord
	started time.Time
}

func (b *rootBase) observeLayout() layoutObservation {
	if f := b.frameCapture; f != nil {
		return layoutObservation{frame: f, started: time.Now()}
	}
	return layoutObservation{}
}

func (s layoutObservation) end() {
	if s.frame != nil {
		s.frame.Layout += time.Since(s.started)
	}
}
