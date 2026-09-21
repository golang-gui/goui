// Package textedit provides plain-text documents, edit history, preedit
// projections, paragraph height indexes and Cluster-based editing geometry.
//
// It owns no Widgets, native layouts, windows, timers or input controllers.
// Geometry consumes typography metrics supplied by the caller; native resource
// creation and destruction remain the caller's responsibility. Positions are
// UTF-8 byte offsets and geometry is in layout-local DIP coordinates.
//
// Mutable objects are sequential, not safe for concurrent access. Separate
// views may share a Model but must keep their selection, Projection, Geometry
// and HeightIndex independent. Model change signals are synchronous.
package textedit
