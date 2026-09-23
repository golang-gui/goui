// Package dragdrop contains the portable facts exchanged by native drag and
// drop backends. It deliberately has no knowledge of widgets or GUI policy.
package dragdrop

import (
	"fmt"
	"image"
	"math"
	"mime"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/golang-gui/goui/core/geometry"
)

// Action is a set of actions while advertising a source or target, and a
// single action when reporting a completed operation.
type Action uint8

const (
	Copy Action = 1 << iota
	Move
	Link
)

const AllActions = Copy | Move | Link

func (a Action) ValidSet() bool { return a&^AllActions == 0 }

func (a Action) ValidResult() bool {
	return a.ValidSet() && (a == 0 || a&(a-1) == 0)
}

type Format string

const (
	FormatText  Format = "text"
	FormatFiles Format = "files"
	FormatURLs  Format = "urls"
	// FormatLocalMarker is an internal native session marker. It carries no Go
	// object or local-format name and is only usable after SourceID validation.
	FormatLocalMarker Format = "goui-local-marker"
)

// MIMEFormat returns an empty Format for an invalid media type or one reserved
// for the built-in text/URI representations.
func MIMEFormat(mediaType string) Format {
	canonical, ok := canonicalMIME(mediaType)
	if !ok || reservedMIME(canonical) {
		return ""
	}
	return Format("mime:" + canonical)
}

// Data is a finite, copyable collection of portable representations. An empty
// representation is distinct from an absent one. It is not a live data stream.
type Data struct {
	text  *string
	files []string
	urls  []string
	mimes map[string][]byte
}

const (
	MaxDataBytes = 64 << 20
	MaxItems     = 100000
)

func (d *Data) SetText(value string) { d.text = &value }

func (d *Data) Text() (string, bool) {
	if d == nil || d.text == nil {
		return "", false
	}
	return *d.text, true
}

func (d *Data) SetFiles(paths []string) { d.files = slices.Clone(paths) }

func (d *Data) Files() ([]string, bool) {
	if d == nil || d.files == nil {
		return nil, false
	}
	return slices.Clone(d.files), true
}

func (d *Data) SetURLs(urls []string) { d.urls = slices.Clone(urls) }

func (d *Data) URLs() ([]string, bool) {
	if d == nil || d.urls == nil {
		return nil, false
	}
	return slices.Clone(d.urls), true
}

// SetBytes stores one custom MIME representation. Invalid or reserved media
// types are retained so Validate can report a source configuration error.
func (d *Data) SetBytes(mediaType string, value []byte) {
	if d.mimes == nil {
		d.mimes = make(map[string][]byte)
	}
	if canonical, ok := canonicalMIME(mediaType); ok {
		mediaType = canonical
	}
	d.mimes[mediaType] = slices.Clone(value)
}

func (d *Data) Bytes(mediaType string) ([]byte, bool) {
	if d == nil {
		return nil, false
	}
	if canonical, ok := canonicalMIME(mediaType); ok {
		mediaType = canonical
	}
	value, ok := d.mimes[mediaType]
	return slices.Clone(value), ok
}

// Formats returns a deterministic list of present representations. It does
// not validate them; call Validate before offering the data natively.
func (d *Data) Formats() []Format {
	if d == nil {
		return nil
	}
	var formats []Format
	if d.text != nil {
		formats = append(formats, FormatText)
	}
	if d.files != nil {
		formats = append(formats, FormatFiles)
	}
	if d.urls != nil {
		formats = append(formats, FormatURLs)
	}
	keys := make([]string, 0, len(d.mimes))
	for key := range d.mimes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		formats = append(formats, Format("mime:"+key))
	}
	return formats
}

// Clone freezes the slice and byte representations for one drag session.
// Like Data itself, it does not own the files identified by paths.
func (d *Data) Clone() *Data {
	if d == nil {
		return nil
	}
	copy := &Data{files: slices.Clone(d.files), urls: slices.Clone(d.urls)}
	if d.text != nil {
		copy.SetText(*d.text)
	}
	if d.mimes != nil {
		copy.mimes = make(map[string][]byte, len(d.mimes))
		for key, value := range d.mimes {
			copy.mimes[key] = slices.Clone(value)
		}
	}
	return copy
}

// Validate checks the finite source representations before a native session
// begins. The backend must still validate externally supplied data on read.
func (d *Data) Validate() error {
	if d == nil {
		return nil // A GUI-local representation may be the only payload.
	}
	total := int64(0)
	add := func(size int) error {
		if size > MaxDataBytes || total+int64(size) > MaxDataBytes {
			return fmt.Errorf("dragdrop: portable data exceeds %d bytes", MaxDataBytes)
		}
		total += int64(size)
		return nil
	}
	if d.text != nil {
		if !utf8.ValidString(*d.text) || strings.ContainsRune(*d.text, 0) {
			return fmt.Errorf("dragdrop: invalid UTF-8 text or NUL")
		}
		if err := add(len(*d.text)); err != nil {
			return err
		}
	}
	if d.files != nil {
		if len(d.files) == 0 || len(d.files) > MaxItems {
			return fmt.Errorf("dragdrop: invalid file count %d", len(d.files))
		}
		for _, path := range d.files {
			if err := validateFilePath(path); err != nil {
				return err
			}
			if err := add(len(path)); err != nil {
				return err
			}
		}
	}
	if d.urls != nil {
		if len(d.urls) == 0 || len(d.urls) > MaxItems {
			return fmt.Errorf("dragdrop: invalid URL count %d", len(d.urls))
		}
		for _, raw := range d.urls {
			if err := validateURL(raw); err != nil {
				return err
			}
			if err := add(len(raw)); err != nil {
				return err
			}
		}
	}
	for mediaType, value := range d.mimes {
		canonical, ok := canonicalMIME(mediaType)
		if !ok || canonical != mediaType || reservedMIME(mediaType) {
			return fmt.Errorf("dragdrop: invalid or reserved MIME type %q", mediaType)
		}
		if err := add(len(value)); err != nil {
			return err
		}
	}
	if d.files != nil && d.urls != nil {
		if len(d.files) != len(d.urls) {
			return fmt.Errorf("dragdrop: files and URLs conflict")
		}
		for i, path := range d.files {
			actual, err := FilePathFromURL(d.urls[i])
			if err != nil || actual != path {
				return fmt.Errorf("dragdrop: files and URLs conflict at index %d", i)
			}
		}
	}
	return nil
}

// Preview is a caller-owned image until Begin copies it. Scale is pixels per
// DIP; zero requests the source surface's current scale.
type Preview struct {
	Image   image.Image
	Scale   float32
	Hotspot geometry.Point
}

// Validate rejects bad scale/hotspot and images beyond the same 16384-pixel
// dimension bound used by RenderWidget. Native backends still copy and convert
// pixels before returning from Begin; an image.Image is not retained by them.
func (p Preview) Validate() error {
	finite := func(value float32) bool {
		return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
	}
	if !finite(p.Scale) || p.Scale < 0 || !finite(p.Hotspot.X) || !finite(p.Hotspot.Y) {
		return fmt.Errorf("dragdrop: invalid preview scale or hotspot")
	}
	if p.Image != nil {
		bounds := p.Image.Bounds()
		if bounds.Dx() <= 0 || bounds.Dy() <= 0 || bounds.Dx() > 16384 || bounds.Dy() > 16384 {
			return fmt.Errorf("dragdrop: invalid preview image bounds %v", bounds)
		}
	}
	return nil
}

// Result records the observable terminal state of a native drag session.
// A zero Action without Canceled or Err means no target accepted the drop.
type Result struct {
	Action   Action
	Canceled bool
	Err      error
}

// Offer is a native incoming transfer. Read starts exactly one format read;
// the backend later emits one data event, possibly before Read returns. Finish
// acknowledges the final action once. Calls are thread-affine and only valid
// while the native session is alive.
type Offer interface {
	ID() uint64
	// SourceID is nonzero only after the backend verified an active local source.
	SourceID() uint64
	Formats() []Format
	Read(Format) error
	Finish(Action) error
}

func (r Result) Validate() error {
	if !r.Action.ValidResult() {
		return fmt.Errorf("dragdrop: invalid result action %d", r.Action)
	}
	if r.Action != 0 && (r.Canceled || r.Err != nil) {
		return fmt.Errorf("dragdrop: completed action conflicts with cancel or error")
	}
	return nil
}

func canonicalMIME(mediaType string) (string, bool) {
	t, params, err := mime.ParseMediaType(mediaType)
	if err != nil || t == "" || !strings.Contains(t, "/") {
		return "", false
	}
	if len(params) != 0 {
		return mime.FormatMediaType(t, params), true
	}
	return t, true
}

func reservedMIME(mediaType string) bool {
	t, _, _ := mime.ParseMediaType(mediaType)
	return t == "text/plain" || t == "text/uri-list"
}
