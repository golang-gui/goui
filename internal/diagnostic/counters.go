// Package diagnostic contains opt-in operation counters shared by the GUI and
// native backends. Recording never calls user code or changes resource ownership.
package diagnostic

type Kind uint8

const (
	LayoutCreate Kind = iota
	LayoutResize
	LayoutDestroy
	TextRasterize
	ImageCreate
	ImageUpdate
	ImageDestroy
	ImageBytes
	TextCacheHit
	TextCacheMiss
	TextCacheInvalidate
	IconGenerate
	DrawingGenerate
	Count
)

type Values [Count]uint64

func (s *Session) Values() (v Values) {
	for i := range v {
		v[i] = s.values[i].Load()
	}
	return
}

// Record has no allocation or clock read, including when collection is active.
// Values are operation counts within a capture, not current live-resource sizes.
func Record(kind Kind, n uint64) {
	if s := current.Load(); s != nil {
		s.values[kind].Add(n)
	}
}

// RecordImage records one successful image operation and its pixel payload.
func RecordImage(kind Kind, width, height int) {
	if s := current.Load(); s != nil {
		s.values[kind].Add(1)
		s.values[ImageBytes].Add(uint64(width) * uint64(height) * 4)
	}
}
