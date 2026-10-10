// Package dev is GOUI's opt-in local development protocol server, for tools and AI
// agents that inspect snapshots, dispatch platform events and commit text.
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
// app whether it uses the ui layer or gui directly. A bare port or :port binds
// loopback; an explicit host is used as provided. Use it only for local development.
// It adapts public GUI hosts and is not part of the core widget kernel.
//
// GET /goui/dev/snapshot labels every window, visible popover and semantic node
// with a dev-owned ref, independent of the application's optional id. Window
// refs last for their lifetime; hiding/releasing a popover invalidates its ref;
// semantic node refs last only for the published snapshot. Refs are not reused
// within the server and are not persistent across process restarts.
//
// POST /goui/dev/event accepts {"target":"1","type":"pointer_down",...}.
// POST /goui/dev/text accepts {"target":"1","text":"hello"}.
// Both targets must be Window/Popover refs from a snapshot, not program IDs or
// semantic node refs. Pointer coordinates are host-local DIP. Text uses current
// editable logical focus; it does not select a widget, activate a native window,
// call SetText or simulate an input-method composition. Successful responses
// confirm dispatch, not completed painting. Queued canceled requests do not run;
// already-started input cannot be rolled back or safely retried automatically.
package dev
