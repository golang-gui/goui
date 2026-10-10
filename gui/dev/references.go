package dev

import (
	"fmt"
	"strconv"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/gui"
	"github.com/golang-gui/goui/platform/events"
)

// inputHost is a public GUI capability, not another event dispatcher.
type inputHost interface {
	DispatchEvent(events.Event) error
	CommitText(string) error
}

type rootReference struct {
	ref    string
	host   inputHost
	owner  gui.Window // detects a popover being discovered under a different owner
	closed signal.Handle
}

// references is owned by the server and accessed exclusively on the GUI thread.
// Root references borrow GUI objects; semantic node references are only labels
// for the published snapshot, never a way to invoke widget methods.
type references struct {
	next    uint64
	roots   map[inputHost]*rootReference
	targets map[string]*rootReference
}

func (r *references) allocate() string {
	r.next++
	return strconv.FormatUint(r.next, 10)
}

func (r *references) add(host inputHost, owner gui.Window) *rootReference {
	if entry := r.roots[host]; entry != nil {
		if entry.owner == owner {
			return entry
		}
		r.remove(entry)
	}
	if r.roots == nil {
		r.roots = make(map[inputHost]*rootReference)
		r.targets = make(map[string]*rootReference)
	}
	entry := &rootReference{ref: r.allocate(), host: host, owner: owner}
	r.roots[host], r.targets[entry.ref] = entry, entry
	if p, ok := host.(gui.Popover); ok {
		entry.closed = p.ConnectClosed(func() { r.remove(entry) })
	} else if w, ok := host.(gui.Window); ok {
		entry.closed = w.ConnectDestroy(func() { r.remove(entry) })
	}
	return entry
}

func (r *references) remove(entry *rootReference) {
	if r.roots[entry.host] != entry {
		return
	}
	delete(r.roots, entry.host)
	delete(r.targets, entry.ref)
	if entry.closed != nil {
		entry.closed.Disconnect()
		entry.closed = nil
	}
}

// refresh discovers public GUI surfaces afresh on every request. Close signals
// also invalidate a hide/show cycle between requests, even on the same object.
func (r *references) refresh(app gui.Application) []gui.Window {
	windows := app.Windows()
	live := make(map[inputHost]bool)
	for _, w := range windows {
		r.add(w, nil)
		live[w] = true
		for _, p := range w.Popovers() {
			if p.Visible() {
				r.add(p, w)
				live[p] = true
			}
		}
	}
	for host, entry := range r.roots {
		if !live[host] {
			r.remove(entry)
		}
	}
	return windows
}

func (r *references) resolve(ref string) (*rootReference, error) {
	if entry := r.targets[ref]; entry != nil {
		return entry, nil
	}
	return nil, fmt.Errorf("target ref %q is unavailable; use a current Window or Popover ref", ref)
}

type applicationSnapshot struct {
	Windows []windowSnapshot `json:"windows"`
}

type windowSnapshot struct {
	gui.WindowInfo
	Ref      string            `json:"ref"`
	Widget   *widgetSnapshot   `json:"widget"`
	Controls *widgetSnapshot   `json:"controls,omitempty"`
	Popovers []popoverSnapshot `json:"popovers,omitempty"`
}

type popoverSnapshot struct {
	Ref    string          `json:"ref"`
	Modal  bool            `json:"modal"`
	Widget *widgetSnapshot `json:"widget"`
}

type widgetSnapshot struct {
	gui.WidgetInfo
	Ref      string           `json:"ref"`
	Children []widgetSnapshot `json:"children"`
}

func (r *references) widget(info gui.WidgetInfo) widgetSnapshot {
	node := widgetSnapshot{WidgetInfo: info, Ref: r.allocate()}
	// Preserve the semantic producer's filtering/order, not Widget.Children.
	// Clear the embedded subtree so only the labelled copy is retained.
	node.WidgetInfo.Children = nil
	for _, child := range info.Children {
		node.Children = append(node.Children, r.widget(child))
	}
	return node
}

func (r *references) snapshot(windows []gui.Window) applicationSnapshot {
	result := applicationSnapshot{Windows: make([]windowSnapshot, 0, len(windows))}
	for _, w := range windows {
		info := w.Snapshot()
		entry := r.roots[w]
		if entry == nil { // A custom Snapshot may reenter and destroy its host.
			continue
		}
		snapshot := windowSnapshot{WindowInfo: info, Ref: entry.ref}
		if w.Widget() != nil {
			node := r.widget(info.Widget)
			snapshot.Widget = &node
		}
		if info.Controls != nil {
			node := r.widget(*info.Controls)
			snapshot.Controls = &node
		}
		for _, p := range w.Popovers() {
			entry := r.roots[p]
			if entry == nil || !p.Visible() {
				continue
			}
			popup := popoverSnapshot{Ref: entry.ref, Modal: p.Modal()}
			if content := p.Widget(); content != nil {
				node := r.widget(content.Snapshot())
				popup.Widget = &node
			}
			if r.roots[p] == entry {
				snapshot.Popovers = append(snapshot.Popovers, popup)
			}
		}
		if r.roots[w] != nil {
			result.Windows = append(result.Windows, snapshot)
		}
	}
	return result
}

func (entry *rootReference) dispatch(event events.Event) error {
	if entry.owner != nil {
		if _, key := event.(events.KeyEvent); key {
			p := entry.host.(gui.Popover)
			if !p.Modal() {
				return fmt.Errorf("popover does not own modal keyboard input")
			}
		}
	}
	// The popover's public entry checks actual modal ownership. Sending a key
	// to the owner instead could silently deliver it to a DIFFERENT popover.
	return entry.host.DispatchEvent(event)
}
