package diagnostic

import (
	"time"

	"github.com/golang-gui/goui/core/geometry"
)

// Options selects additional collection. Frame timings and resource
// counters are always collected after StartCapture; widget counters opt in.
type Options struct {
	Widgets bool `json:"widgets"`
}

// Resources counts observed operations, including offscreen work.
// PixelBytes is supplied RGBA payload, not GPU memory or native allocation size.
// Counts include resources created before capture; they do not imply live totals.
type Resources struct {
	LayoutsCreated         uint64 `json:"layoutsCreated"`
	LayoutsResized         uint64 `json:"layoutsResized"`
	LayoutsDestroyed       uint64 `json:"layoutsDestroyed"`
	TextRasterizations     uint64 `json:"textRasterizations"`
	ImagesCreated          uint64 `json:"imagesCreated"`
	ImagesUpdated          uint64 `json:"imagesUpdated"`
	ImagesDestroyed        uint64 `json:"imagesDestroyed"`
	PixelBytes             uint64 `json:"pixelBytes"`
	TextCacheHits          uint64 `json:"textCacheHits"`
	TextCacheMisses        uint64 `json:"textCacheMisses"`
	TextCacheInvalidations uint64 `json:"textCacheInvalidations"`
	IconGenerations        uint64 `json:"iconGenerations"`
	DrawingGenerations     uint64 `json:"drawingGenerations"`
}

func resourcesFromValues(v Values) Resources {
	return Resources{v[LayoutCreate], v[LayoutResize], v[LayoutDestroy],
		v[TextRasterize], v[ImageCreate], v[ImageUpdate], v[ImageDestroy],
		v[ImageBytes], v[TextCacheHit], v[TextCacheMiss], v[TextCacheInvalidate],
		v[IconGenerate], v[DrawingGenerate]}
}

// Frame describes CPU work. EndReturned means Painter.End returned;
// it does not confirm GPU execution, a successful present or compositor timing.
// Completed additionally requires drawing to finish without cancellation or
// panic. Durations are nanoseconds in JSON. Paint excludes Begin and End.
type Frame struct {
	RootRef     uint64        `json:"rootRef"`
	Started     time.Time     `json:"started"`
	Total       time.Duration `json:"total"`
	Callbacks   time.Duration `json:"callbacks"`
	Styles      time.Duration `json:"styles"`
	Layout      time.Duration `json:"layout"`
	Begin       time.Duration `json:"begin"`
	Paint       time.Duration `json:"paint"`
	End         time.Duration `json:"end"`
	EndReturned bool          `json:"endReturned"`
	Completed   bool          `json:"completed"`
}

// Root summarizes one capture-local root. FPS estimates recent
// completed CPU-frame intervals; insufficient samples or Idle yield zero.
type Root struct {
	Ref            uint64        `json:"ref"`
	Kind           string        `json:"kind"`
	ID             string        `json:"id,omitempty"`
	Title          string        `json:"title,omitempty"`
	Size           geometry.Size `json:"size"`
	Closed         bool          `json:"closed"`
	Frames         uint64        `json:"frames"`
	Completed      uint64        `json:"completed"`
	Aborted        uint64        `json:"aborted"`
	PaintRequests  uint64        `json:"paintRequests"`
	LayoutRequests uint64        `json:"layoutRequests"`
	Offscreen      uint64        `json:"offscreen"`
	FPS            float64       `json:"fps"`
	Idle           bool          `json:"idle"`
	Backend        string        `json:"backend,omitempty"`
}

// Widget records framework work, not direct calls to a custom
// Measure/Paint implementation. Bounds are root-local DIP at snapshot time.
// Refs belong to this capture and are independent of application IDs/dev refs.
type Widget struct {
	Ref                 uint64             `json:"ref"`
	RootRef             uint64             `json:"rootRef,omitempty"`
	ID                  string             `json:"id,omitempty"`
	Name                string             `json:"name,omitempty"`
	Type                string             `json:"type"`
	Bounds              geometry.Rectangle `json:"bounds"`
	MeasureQueries      uint64             `json:"measureQueries"`
	MeasureHits         uint64             `json:"measureHits"`
	MeasureCalls        uint64             `json:"measureCalls"`
	PaintCalls          uint64             `json:"paintCalls"`
	OffscreenPaintCalls uint64             `json:"offscreenPaintCalls"`
	StyleChangedCalls   uint64             `json:"styleChangedCalls"`
	PaintRequests       uint64             `json:"paintRequests"`
	LayoutRequests      uint64             `json:"layoutRequests"`
}

// Snapshot is an independent, bounded copy. Resource counts are
// process-wide, not exclusively attributable to a root, widget or CPU frame.
type Snapshot struct {
	Active         bool      `json:"active"`
	Started        time.Time `json:"started"`
	Captured       time.Time `json:"captured"`
	Options        Options   `json:"options"`
	Resources      Resources `json:"resources"`
	Roots          []Root    `json:"roots"`
	Widgets        []Widget  `json:"widgets,omitempty"`
	Frames         []Frame   `json:"frames"`
	DroppedRoots   uint64    `json:"droppedRoots"`
	DroppedWidgets uint64    `json:"droppedWidgets"`
}
