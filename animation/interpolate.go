package animation

// Float32 interpolates or extrapolates a float32 value.
func Float32(from, to float32, progress float64) float32 {
	return float32(float64(from) + (float64(to)-float64(from))*progress)
}

// Float64 interpolates or extrapolates a float64 value.
func Float64(from, to float64, progress float64) float64 { return from + (to-from)*progress }
