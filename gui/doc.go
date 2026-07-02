// Package gui implements GOUI's imperative widget kernel.
//
// It owns applications, windows, widgets, layout participation, painting, event
// dispatch, focus state, signals, and semantic snapshots. Package gui is below
// package ui and must not import it. Automation should inspect snapshots and
// dispatch platform events through Application.DispatchWindowEvent or
// Window.DispatchEvent instead of bypassing widget behavior.
package gui
