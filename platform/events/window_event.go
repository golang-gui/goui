package events

type CloseEvent struct{}

func (e CloseEvent) Type() EventType {
	return CloseRequest
}

func (e CloseEvent) isEvent() {}

type SizeEvent struct {
	// Width, Height are the logical (DIP) size used for layout and hit-testing.
	Width  float32
	Height float32
	// PixelWidth, PixelHeight are the exact physical (backing) pixel size used
	// for the painter surface. scale = PixelWidth / Width.
	PixelWidth  float32
	PixelHeight float32
}

func (e SizeEvent) Type() EventType {
	return Size
}

func (e SizeEvent) isEvent() {}

type PaintEvent struct{}

func (e PaintEvent) Type() EventType {
	return Paint
}

func (e PaintEvent) isEvent() {}

type FocusEvent struct {
	Focused bool
}

func (e FocusEvent) Type() EventType {
	return Focus
}

func (e FocusEvent) isEvent() {}

var (
	_ Event = CloseEvent{}
	_ Event = SizeEvent{}
	_ Event = PaintEvent{}
	_ Event = FocusEvent{}
)
