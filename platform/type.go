package platform

import (
	"github.com/golang-gui/goui/platform/common"
	"github.com/golang-gui/goui/platform/events"
)

type (
	Window       = common.Window
	EventQueue   = common.EventQueue
	EventHandler = events.EventHandler

	Event = events.Event
)
