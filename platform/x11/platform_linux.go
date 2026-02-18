package x11

import (
	"sync"

	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
	"github.com/golang-gui/goui/platform/x11/libx"
)

type Platform struct {
	display *libx.Display
	atoms   struct {
		WM_STATE          libx.Atom
		WM_PROTOCOLS      libx.Atom
		WM_DELETE_WINDOW  libx.Atom
		_NET_WM_NAME      libx.Atom
		_NET_WM_ICON      libx.Atom
		_NET_WM_ICON_NAME libx.Atom
	}
}

var (
	newOnce  sync.Once
	platform *Platform
)

func NewPlatform() (p *Platform, err error) {
	newOnce.Do(func() {
		p = new(Platform)
		p.display, err = libx.OpenDisplay("")
		if err != nil {
			return
		}

		// intern atoms
		internAtom := func(name string) libx.Atom {
			atom, _ := libx.InternAtom(p.display, name, false)
			return atom
		}

		p.atoms.WM_STATE = internAtom("WM_STATE")
		p.atoms.WM_PROTOCOLS = internAtom("WM_PROTOCOLS")
		p.atoms.WM_DELETE_WINDOW = internAtom("WM_DELETE_WINDOW")
		p.atoms._NET_WM_NAME = internAtom("_NET_WM_NAME")
		p.atoms._NET_WM_ICON = internAtom("_NET_WM_ICON")
		p.atoms._NET_WM_ICON_NAME = internAtom("_NET_WM_ICON_NAME")

		platform = p
	})
	if err != nil {
		return nil, err
	}
	return platform, nil
}

func (p *Platform) Name() string {
	return "x11"
}

func (p *Platform) Destroy() {

}

func (p *Platform) NewEventQueue() (common.EventQueue, error) {
	return newEventQueue()
}

func (p *Platform) NewWindow(handler events.EventHandler) (common.Window, error) {
	return newWindow(handler)
}

func (p *Platform) NewImage(width, height int) (common.Image, error) {
	panic("TODO impl")
}
