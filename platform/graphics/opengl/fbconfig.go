package opengl

import "math"

type FBConfig struct {
	PixelFormat
	Handle uintptr
}

func ChooseFBConfig(desired PixelFormat, alternatives []FBConfig) (closest FBConfig) {
	var missing, colorDiff, extraDiff uint32
	var leastMissing uint32 = math.MaxUint32
	var leastColorDiff uint32 = math.MaxUint32
	var leastExtraDiff uint32 = math.MaxUint32

	for _, current := range alternatives {
		if desired.Stereo && !current.Stereo {
			// Stereo is a hard constraint
			continue
		}

		// Count number of missing buffers
		{
			missing = 0

			if desired.AlphaBits > 0 && current.AlphaBits == 0 {
				missing++
			}

			if desired.DepthBits > 0 && current.DepthBits == 0 {
				missing++
			}

			if desired.StencilBits > 0 && current.StencilBits == 0 {
				missing++
			}

			if desired.AuxBuffers > 0 && current.AuxBuffers < desired.AuxBuffers {
				missing += uint32(desired.AuxBuffers - current.AuxBuffers)
			}

			if desired.Samples > 0 && current.Samples == 0 {
				// Technically, several multisampling buffers could be
				// involved, but that's a lower level implementation detail and
				// not important to us here, so we count them as one
				missing++
			}

			if desired.Transparent != current.Transparent {
				missing++
			}
		}

		// These polynomials make many small channel size differences matter
		// less than one large channel size difference

		// Calculate color channel size difference value
		{
			colorDiff = 0

			if desired.RedBits != DontCare {
				colorDiff += uint32((desired.RedBits - current.RedBits) *
					(desired.RedBits - current.RedBits))
			}

			if desired.GreenBits != DontCare {
				colorDiff += uint32((desired.GreenBits - current.GreenBits) *
					(desired.GreenBits - current.GreenBits))
			}

			if desired.BlueBits != DontCare {
				colorDiff += uint32((desired.BlueBits - current.BlueBits) *
					(desired.BlueBits - current.BlueBits))
			}
		}

		// Calculate non-color channel size difference value
		{
			extraDiff = 0

			if desired.AlphaBits != DontCare {
				extraDiff += uint32((desired.AlphaBits - current.AlphaBits) *
					(desired.AlphaBits - current.AlphaBits))
			}

			if desired.DepthBits != DontCare {
				extraDiff += uint32((desired.DepthBits - current.DepthBits) *
					(desired.DepthBits - current.DepthBits))
			}

			if desired.StencilBits != DontCare {
				extraDiff += uint32((desired.StencilBits - current.StencilBits) *
					(desired.StencilBits - current.StencilBits))
			}

			if desired.AccumRedBits != DontCare {
				extraDiff += uint32((desired.AccumRedBits - current.AccumRedBits) *
					(desired.AccumRedBits - current.AccumRedBits))
			}

			if desired.AccumGreenBits != DontCare {
				extraDiff += uint32((desired.AccumGreenBits - current.AccumGreenBits) *
					(desired.AccumGreenBits - current.AccumGreenBits))
			}

			if desired.AccumBlueBits != DontCare {
				extraDiff += uint32((desired.AccumBlueBits - current.AccumBlueBits) *
					(desired.AccumBlueBits - current.AccumBlueBits))
			}

			if desired.AccumAlphaBits != DontCare {
				extraDiff += uint32((desired.AccumAlphaBits - current.AccumAlphaBits) *
					(desired.AccumAlphaBits - current.AccumAlphaBits))
			}

			if desired.Samples != DontCare {
				extraDiff += uint32((desired.Samples - current.Samples) *
					(desired.Samples - current.Samples))
			}

			if desired.SRGB && !current.SRGB {
				extraDiff++
			}
		}

		// Figure out if the current one is better than the best one found so far
		// Least number of missing buffers is the most important heuristic,
		// then color buffer size match and lastly size match for other buffers

		if missing < leastMissing {
			closest = current
		} else if missing == leastMissing {
			if (colorDiff < leastColorDiff) ||
				(colorDiff == leastColorDiff && extraDiff < leastExtraDiff) {
				closest = current
			}
		}

		if current == closest {
			leastMissing = missing
			leastColorDiff = colorDiff
			leastExtraDiff = extraDiff
		}
	}
	return
}
