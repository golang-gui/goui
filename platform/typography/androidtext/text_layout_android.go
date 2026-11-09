package androidtext

import (
	"image/color"
	"math"
	"slices"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/internal/diagnostic"
	"github.com/golang-gui/goui/platform/android/libs/jni"
	"github.com/golang-gui/goui/platform/typography"
	"github.com/golang-gui/goui/platform/typography/internal/lifecycle"
	"github.com/golang-gui/goui/platform/typography/utils"
)

type TextLayout struct {
	native        layoutPeer
	fonts         *fontCollection
	text          string
	format        typography.TextFormat
	width, height float32
	positions     utils.StringPosition
	life          lifecycle.Layout
	valid         bool
	lines         []typography.TextLine
	clusters      []typography.TextCluster
}

func (t *TextLayout) Destroy() {
	if !t.life.BeginDestroy() {
		return
	}
	e := frame()
	defer e.PopLocalFrame(0)
	err := t.native.close(e)
	e.DeleteGlobalRef(jni.Object(t.native))
	t.native = 0
	t.fonts.release(e)
	t.fonts = nil
	t.lines = nil
	t.clusters = nil
	diagnostic.Record(diagnostic.LayoutDestroy, 1)
	if err != nil {
		panic(err)
	}
}
func (t *TextLayout) Text() string                           { return t.text }
func (t *TextLayout) Format() typography.TextFormat          { return t.format }
func (t *TextLayout) Size() (float32, float32)               { return t.width, t.height }
func (t *TextLayout) ConnectChanged(fn func()) signal.Handle { return t.life.ConnectChanged(fn) }
func (t *TextLayout) ConnectDestroy(fn func()) signal.Handle { return t.life.ConnectDestroy(fn) }
func (t *TextLayout) changed(metrics bool) {
	if metrics {
		t.valid = false
		t.lines = nil
		t.clusters = nil
	}
	t.life.Changed()
}
func (t *TextLayout) SetSize(width, height float32) {
	if t.width == width && t.height == height {
		return
	}
	if err := dimensions(width, height); err != nil {
		panic(err)
	}
	e := frame()
	defer e.PopLocalFrame(0)
	if err := t.native.setSize(e, width, height); err != nil {
		panic(err)
	}
	t.width = width
	t.height = height
	diagnostic.Record(diagnostic.LayoutResize, 1)
	t.changed(true)
}
func (t *TextLayout) SetTextAlignment(value typography.TextAlignment) {
	if t.format.TextAlign == value {
		return
	}
	if err := alignment(value); err != nil {
		panic(err)
	}
	e := frame()
	defer e.PopLocalFrame(0)
	if err := t.native.setAlignment(e, int32(value)); err != nil {
		panic(err)
	}
	t.format.TextAlign = value
	t.changed(true)
}
func (t *TextLayout) SetWrapMode(value typography.WrapMode) {
	if t.format.WrapMode == value {
		return
	}
	if err := wrapping(value); err != nil {
		panic(err)
	}
	e := frame()
	defer e.PopLocalFrame(0)
	if err := t.native.setWrap(e, int32(value)); err != nil {
		panic(err)
	}
	t.format.WrapMode = value
	t.changed(true)
}
func (t *TextLayout) SetTabWidth(value float32) {
	if t.format.TabWidth == value {
		return
	}
	if err := utils.ValidateTabWidth(value); err != nil {
		panic(err)
	}
	e := frame()
	defer e.PopLocalFrame(0)
	if err := t.native.setTabWidth(e, value); err != nil {
		panic(err)
	}
	t.format.TabWidth = value
	t.changed(true)
}
func (t *TextLayout) textRange(start, length int) (int32, int32) {
	utils.CheckTextRange(t.text, start, length)
	return int32(t.positions.ToUtf16(start)), int32(t.positions.ToUtf16(start + length))
}
func (t *TextLayout) SetTextFont(start, length int, font typography.FontInfo) {
	a, b := t.textRange(start, length)
	font, err := resolveFont(font)
	if err != nil {
		panic(err)
	}
	e := frame()
	defer e.PopLocalFrame(0)
	face, err := t.fonts.resolve(e, font)
	if err != nil {
		panic(err)
	}
	if err = t.native.setFont(e, a, b, face, font.Size*96/72*nativeUnits); err != nil {
		panic(err)
	}
	t.changed(true)
}
func (t *TextLayout) SetTextColor(start, length int, c color.Color) {
	a, b := t.textRange(start, length)
	e := frame()
	defer e.PopLocalFrame(0)
	if err := t.native.setColor(e, a, b, argb(c)); err != nil {
		panic(err)
	}
	t.changed(false)
}
func (t *TextLayout) SetUnderline(start, length int, enabled bool) {
	a, b := t.textRange(start, length)
	e := frame()
	defer e.PopLocalFrame(0)
	if err := t.native.setUnderline(e, a, b, enabled); err != nil {
		panic(err)
	}
	t.changed(false)
}
func (t *TextLayout) SetStrikethrough(start, length int, enabled bool) {
	a, b := t.textRange(start, length)
	e := frame()
	defer e.PopLocalFrame(0)
	if err := t.native.setStrike(e, a, b, enabled); err != nil {
		panic(err)
	}
	t.changed(false)
}
func (t *TextLayout) SetPlaceholder(start int, metrics *typography.PlaceholderMetrics) {
	utils.CheckPlaceholder(t.text, start, metrics)
	var value typography.PlaceholderMetrics
	var present int32
	if metrics != nil {
		value = *metrics
		if float64(value.Width)*64 > math.MaxInt32 || float64(value.Height)*64 > math.MaxInt32 {
			panic("androidtext: placeholder exceeds native coordinate range")
		}
		present = 1
	}
	e := frame()
	defer e.PopLocalFrame(0)
	if err := t.native.setPlaceholder(e, int32(t.positions.ToUtf16(start)), present, value.Width, value.Height, value.Baseline); err != nil {
		panic(err)
	}
	t.changed(true)
}
func (t *TextLayout) prepare() {
	if t.valid {
		return
	}
	e := frame()
	defer e.PopLocalFrame(0)
	data, err := t.native.metrics(e)
	if err != nil {
		panic(err)
	}
	li, lf, ci, cf := data.lineIndices, data.lineCoordinates, data.clusterIndices, data.clusterCoordinates
	if len(li)%2 != 0 || len(lf) != len(li)/2*5 || len(ci)%4 != 0 || len(ci) != len(cf) {
		panic("androidtext: invalid metrics packet")
	}
	lines := make([]typography.TextLine, len(li)/2)
	clusters := make([]typography.TextCluster, len(ci)/4)
	for i := range lines {
		a := t.positions.ToUtf8(int(li[i*2]))
		b := t.positions.ToUtf8(int(li[i*2] + li[i*2+1]))
		f := lf[i*5:]
		lines[i] = typography.TextLine{Start: a, Length: b - a, X: f[0] / nativeUnits, Y: f[1] / nativeUnits, Width: f[2] / nativeUnits, Height: f[3] / nativeUnits, Baseline: f[4] / nativeUnits}
	}
	for i := range clusters {
		ints := ci[i*4:]
		f := cf[i*4:]
		a := t.positions.ToUtf8(int(ints[0]))
		b := t.positions.ToUtf8(int(ints[0] + ints[1]))
		line := int(ints[2])
		if a < 0 || b < a || line < 0 || line >= len(lines) {
			panic("androidtext: invalid cluster range")
		}
		clusters[i] = typography.TextCluster{Start: a, Length: b - a, X: f[0] / nativeUnits, Y: f[1] / nativeUnits, Width: f[2] / nativeUnits, Height: f[3] / nativeUnits, LineIndex: line, Direction: typography.TextDirection(ints[3])}
		lines[line].Clusters = append(lines[line].Clusters, clusters[i])
	}
	t.lines = lines
	t.clusters = clusters
	t.valid = true
}
func (t *TextLayout) MeasureMetrics() ([]typography.TextLine, []typography.TextCluster) {
	t.prepare()
	lines, clusters := slices.Clone(t.lines), slices.Clone(t.clusters)
	offset := 0
	for i := range lines {
		count := len(lines[i].Clusters)
		if count > 0 {
			lines[i].Clusters = clusters[offset : offset+count]
		}
		offset += count
	}
	return lines, clusters
}
func (t *TextLayout) MeasureSize() (width, height float32) {
	t.prepare()
	for _, line := range t.lines {
		width = max(width, line.Width)
		height = max(height, line.Y+line.Height)
	}
	return
}

var _ typography.TextLayout = (*TextLayout)(nil)
