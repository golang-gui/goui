package lifecycle

import (
	"github.com/golang-gui/goui/core/signal"
)

// Layout centralizes the notification and idempotent-destroy state shared by
// the platform TextLayout implementations. Like TextLayout itself, it is
// thread-affine; the signals' synchronization only provides safe handles.
type Layout struct {
	changed   signal.Signal0
	destroyed signal.Signal0
	isDead    bool
}

func (l *Layout) ConnectChanged(fn func()) signal.Handle {
	return l.changed.Connect(fn)
}

func (l *Layout) ConnectDestroy(fn func()) signal.Handle {
	return l.destroyed.Connect(fn)
}

func (l *Layout) Changed() {
	if !l.isDead {
		l.changed.Emit()
	}
}

// BeginDestroy marks the layout dead and emits its destroy notification. It
// returns false after the first call so native cleanup remains idempotent.
func (l *Layout) BeginDestroy() bool {
	if l.isDead {
		return false
	}
	l.isDead = true
	l.destroyed.Emit()
	return true
}

func (l *Layout) IsDestroyed() bool {
	return l.isDead
}
