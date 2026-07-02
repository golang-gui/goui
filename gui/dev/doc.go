// Package dev exposes GOUI's local development protocol for tools and AI
// agents.
//
// It is an adapter around gui.Application. It may inspect snapshots and
// dispatch platform events, but it should not become part of the core widget
// kernel.
package dev
