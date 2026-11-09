package androidtext

import (
	"fmt"
	"strings"

	"github.com/golang-gui/goui/platform/android/libs/jni"
	"github.com/golang-gui/goui/platform/android/sdk"
	"github.com/golang-gui/goui/platform/typography"
)

// fontCollection owns global references shared by its Context and live layouts.
// All access is thread-affine. Releasing Context does not invalidate its layouts.
type fontCollection struct {
	owners   int
	families map[string][]registeredFace
	cache    map[fontKey]sdk.Typeface
}
type registeredFace struct {
	font                 sdk.Font
	width, weight, slant int32
}
type fontKey struct {
	family               string
	weight, width, style int32
}

func newFontCollection() *fontCollection {
	return &fontCollection{owners: 1, families: make(map[string][]registeredFace), cache: make(map[fontKey]sdk.Typeface)}
}
func (f *fontCollection) release(e jni.Env) {
	f.owners--
	if f.owners != 0 {
		return
	}
	f.clearCache(e)
	for _, faces := range f.families {
		for _, face := range faces {
			e.DeleteGlobalRef(jni.Object(face.font))
		}
	}
	f.families = nil
}
func (f *fontCollection) clearCache(e jni.Env) {
	for key, face := range f.cache {
		e.DeleteGlobalRef(jni.Object(face))
		delete(f.cache, key)
	}
}
func buildFace(e jni.Env, path string, index, width int32) (face registeredFace, err error) {
	if e.PushLocalFrame(16) < 0 {
		return face, fmt.Errorf("androidtext: font local frame: %w", e.Exception())
	}
	defer e.PopLocalFrame(0)
	file, err := sdk.NewFile(e, path)
	if err != nil {
		return face, err
	}
	builder, err := sdk.NewFontBuilder(e, file)
	if err != nil {
		return face, err
	}
	builder, err = builder.SetTtcIndex(e, index)
	if err != nil {
		return face, err
	}
	font, err := builder.Build(e)
	if err != nil {
		return face, err
	}
	style, err := font.GetStyle(e)
	if err != nil {
		return face, err
	}
	face.weight, err = style.GetWeight(e)
	if err != nil {
		return face, err
	}
	face.slant, err = style.GetSlant(e)
	if err != nil {
		return face, err
	}
	ref, err := retain(e, jni.Object(font))
	face.font = sdk.Font(ref)
	face.width = width
	return face, err
}
func (f *fontCollection) add(e jni.Env, path string, metadata []fontFace) (err error) {
	added := make([]registeredFace, 0, len(metadata))
	defer func() {
		if err != nil {
			for _, face := range added {
				e.DeleteGlobalRef(jni.Object(face.font))
			}
		}
	}()
	for _, meta := range metadata {
		face, buildErr := buildFace(e, path, meta.index, meta.width)
		if buildErr != nil {
			return buildErr
		}
		added = append(added, face)
	}
	// Commit only after every native face has been created successfully.
	f.clearCache(e)
	for i, meta := range metadata {
		name := strings.ToLower(meta.family)
		face := added[i]
		old := f.families[name]
		next := make([]registeredFace, 0, len(old)+1)
		for _, previous := range old {
			if previous.width == face.width && previous.weight == face.weight && previous.slant == face.slant {
				e.DeleteGlobalRef(jni.Object(previous.font))
			} else {
				next = append(next, previous)
			}
		}
		f.families[name] = append(next, face)
	}
	return nil
}
func (f *fontCollection) resolve(e jni.Env, font typography.FontInfo) (sdk.Typeface, error) {
	key := fontKey{strings.ToLower(font.Family), int32(font.Weight), int32(font.Width), int32(font.Style)}
	if face := f.cache[key]; face != 0 {
		return face, nil
	}
	var face sdk.Typeface
	var err error
	if faces := f.families[key.family]; len(faces) > 0 {
		chosen := faces[0].width
		for _, candidate := range faces {
			if abs(candidate.width-key.width) < abs(chosen-key.width) {
				chosen = candidate.width
			}
		}
		var builder sdk.FontFamilyBuilder
		for _, candidate := range faces {
			if candidate.width == chosen {
				if builder == 0 {
					builder, err = sdk.NewFontFamilyBuilder(e, candidate.font)
				} else {
					builder, err = builder.AddFont(e, candidate.font)
				}
				if err != nil {
					return 0, err
				}
			}
		}
		family, err := builder.Build(e)
		if err != nil {
			return 0, err
		}
		fallback, err := sdk.NewTypefaceCustomFallbackBuilder(e, family)
		if err != nil {
			return 0, err
		}
		fallback, err = fallback.SetSystemFallback(e, "sans-serif")
		if err != nil {
			return 0, err
		}
		slant := sdk.FontSlantUpright
		if key.style != 0 {
			slant = sdk.FontSlantItalic
		}
		style, err := sdk.NewFontStyle(e, key.weight, slant)
		if err != nil {
			return 0, err
		}
		fallback, err = fallback.SetStyle(e, style)
		if err != nil {
			return 0, err
		}
		face, err = fallback.Build(e)
	} else {
		requested := font.Family
		if requested == "" {
			requested = "sans-serif"
		}
		if (requested == "sans-serif" || requested == "sans") && key.width < 5 {
			requested = "sans-serif-condensed"
		}
		var base sdk.Typeface
		base, err = sdk.CreateTypeface(e, requested, 0)
		if err == nil {
			face, err = sdk.CreateTypefaceWithWeight(e, base, key.weight, key.style != 0)
		}
	}
	if err != nil {
		return 0, err
	}
	ref, err := retain(e, jni.Object(face))
	if err != nil {
		return 0, err
	}
	f.cache[key] = sdk.Typeface(ref)
	return sdk.Typeface(ref), nil
}
func abs(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}
