package typography

type Context interface {
	Name() string
	Destroy()
	AddFont(fontFile string) error
	NewTextLayout(text string, format TextFormat, width, height float32) (TextLayout, error)
}
