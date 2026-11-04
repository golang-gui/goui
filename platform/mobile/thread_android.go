package mobile

import "github.com/golang-gui/goui/platform/internal/androidruntime"

func init() { enterThread = androidruntime.EnterThread }
