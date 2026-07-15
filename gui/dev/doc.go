// Package dev is GOUI's opt-in local development protocol server, for tools and AI
// agents that inspect snapshots and dispatch platform events.
//
// It is self-contained: import it for its side effect and set GOUI_DEV_ADDR to
// enable it. There is no construction API — the server starts itself.
//
//	import _ "github.com/golang-gui/goui/gui/dev"
//	GOUI_DEV_ADDR=:8888 ./app   // omit the variable to disable
//
// Two orthogonal switches keep it out of shipped binaries:
//   - the import controls linkage — without it, net/http is never linked in;
//   - GOUI_DEV_ADDR controls activation — without it, nothing listens.
//
// The server resolves the running application from gui.App on demand, so it may
// start before the app exists (answering 503 until it does) and works for any GOUI
// app whether it uses the ui layer or gui directly. It binds loopback only. It is
// an adapter over gui.Application and is not part of the core widget kernel.
package dev
