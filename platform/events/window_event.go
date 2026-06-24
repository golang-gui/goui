package events

type CloseEvent struct{}

func (e CloseEvent) Type() EventType {
	return CloseRequest
}

func (e CloseEvent) isEvent() {}

type SizeEvent struct {
	Width  uint
	Height uint
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

type ScaleEvent struct {
	ScaleFactor float64
}

func (e ScaleEvent) Type() EventType {
	return Scale
}

func (e ScaleEvent) isEvent() {}

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
	_ Event = ScaleEvent{}
	_ Event = FocusEvent{}
)
