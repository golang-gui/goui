package x11

import (
	"net"
	"sync"
	"unsafe"

	"github.com/golang-gui/goui/platform/common"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

type Platform struct {
	xConn *xgb.Conn
	setup *xproto.SetupInfo
}

var (
	newOnce  sync.Once
	platform *Platform
)

func NewPlatform() (p *Platform, err error) {
	newOnce.Do(func() {
		p = new(Platform)
		p.xConn, err = xgb.NewConn()
		if err != nil {
			return
		}
		p.setup = xproto.Setup(p.xConn)
		platform = p
	})
	return
}

func (p *Platform) Name() string {
	return "x11"
}

func (p *Platform) NewEventQueue() (common.EventQueue, error) {
	return newEventQueue()
}

//func (p *Platform) NewWindow(handler events.EventHandler) (common.Window, error) {
//	return newWindow(handler)
//}

func (p *Platform) getEventChan() (eventChan <-chan any) {
	type hackXConn struct {
		host                string
		conn                net.Conn
		display             string
		DisplayNumber       int
		DefaultScreen       int
		SetupBytes          []byte
		setupResourceIdBase uint32
		setupResourceIdMask uint32
		eventChan           chan any
	}
	conn := (*hackXConn)(unsafe.Pointer(p.xConn))
	return conn.eventChan
}
