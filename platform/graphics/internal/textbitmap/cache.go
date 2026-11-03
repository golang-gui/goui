package textbitmap

import (
	"image"
	"math"

	"github.com/golang-gui/goui/core/signal"
	"github.com/golang-gui/goui/internal/diagnostic"
	"github.com/golang-gui/goui/platform/typography"
)

// ImageCache keeps a bounded set of painter-native images for each layout.
// It owns the cached values and releases them when the layout changes, is
// destroyed, or the cache itself is destroyed. It is thread-affine like the
// Painter that owns it.
type ImageCache[T any] struct {
	capacity int
	release  func(T)
	records  map[typography.TextLayout]*layoutImages[T]
	age      uint64
}

type layoutImages[T any] struct {
	layout  typography.TextLayout
	changed signal.Handle
	destroy signal.Handle
	entries []imageEntry[T]
}

type imageEntry[T any] struct {
	scale  uint32
	region image.Rectangle
	image  T
	age    uint64
}

func NewImageCache[T any](capacity int, release func(T)) *ImageCache[T] {
	if capacity < 1 {
		capacity = 1
	}
	return &ImageCache[T]{
		capacity: capacity,
		release:  release,
		records:  make(map[typography.TextLayout]*layoutImages[T]),
	}
}

func (c *ImageCache[T]) Lookup(layout typography.TextLayout, scale float32, region image.Rectangle) (image T, ok bool) {
	if c == nil || layout == nil {
		return image, false
	}
	record := c.records[layout]
	if record == nil {
		diagnostic.Record(diagnostic.TextCacheMiss, 1)
		return image, false
	}
	key := math.Float32bits(scale)
	for i := range record.entries {
		entry := &record.entries[i]
		if entry.scale == key && entry.region == region {
			entry.age = c.nextAge()
			diagnostic.Record(diagnostic.TextCacheHit, 1)
			return entry.image, true
		}
	}
	diagnostic.Record(diagnostic.TextCacheMiss, 1)
	return image, false
}

func (c *ImageCache[T]) Store(layout typography.TextLayout, scale float32, region image.Rectangle, image T) {
	if c == nil || layout == nil {
		if c != nil {
			c.releaseImage(image)
		}
		return
	}

	record := c.records[layout]
	if record == nil {
		record = &layoutImages[T]{layout: layout}
		record.changed = layout.ConnectChanged(func() {
			c.invalidate(record)
		})
		record.destroy = layout.ConnectDestroy(func() {
			c.remove(record)
		})
		c.records[layout] = record
	}

	key := math.Float32bits(scale)
	for i := range record.entries {
		entry := &record.entries[i]
		if entry.scale == key && entry.region == region {
			old := entry.image
			entry.image = image
			entry.age = c.nextAge()
			c.releaseImage(old)
			return
		}
	}

	entry := imageEntry[T]{scale: key, region: region, image: image, age: c.nextAge()}
	if len(record.entries) < c.capacity {
		record.entries = append(record.entries, entry)
		return
	}

	lru := 0
	for i := 1; i < len(record.entries); i++ {
		if record.entries[i].age < record.entries[lru].age {
			lru = i
		}
	}
	diagnostic.Record(diagnostic.TextCacheInvalidate, 1)
	old := record.entries[lru].image
	record.entries[lru] = entry
	c.releaseImage(old)
}

func (c *ImageCache[T]) Destroy() {
	if c == nil {
		return
	}
	for _, record := range c.records {
		record.changed.Disconnect()
		record.destroy.Disconnect()
		c.releaseEntries(record)
	}
	clear(c.records)
}

func (c *ImageCache[T]) invalidate(record *layoutImages[T]) {
	if c == nil || record == nil || c.records[record.layout] != record {
		return
	}
	c.releaseEntries(record)
}

func (c *ImageCache[T]) remove(record *layoutImages[T]) {
	if c == nil || record == nil || c.records[record.layout] != record {
		return
	}
	delete(c.records, record.layout)
	record.changed.Disconnect()
	record.destroy.Disconnect()
	c.releaseEntries(record)
}

func (c *ImageCache[T]) releaseEntries(record *layoutImages[T]) {
	diagnostic.Record(diagnostic.TextCacheInvalidate, uint64(len(record.entries)))
	for _, entry := range record.entries {
		c.releaseImage(entry.image)
	}
	record.entries = record.entries[:0]
}

func (c *ImageCache[T]) releaseImage(image T) {
	if c.release != nil {
		c.release(image)
	}
}

func (c *ImageCache[T]) nextAge() uint64 {
	c.age++
	return c.age
}
