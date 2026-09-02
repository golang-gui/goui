package graphics

import "image"

// Image is a painter-native, fixed-size raster resource.
//
// Images are created by Painter.NewImage and are bound to the Painter that
// created them. They are thread-affine like their owner Painter. Destroy is
// idempotent, but must not be called between the owner's Begin and End calls.
// Painter.Destroy also destroys any images that are still alive and is likewise
// invalid during an active frame.
type Image interface {
	// Size returns the immutable raster dimensions in physical pixels.
	Size() (width, height int)
	// Update replaces all pixels without changing Size. It snapshots src before
	// returning and must not be called during an active Painter frame.
	Update(src image.Image) error
	Destroy()
}
