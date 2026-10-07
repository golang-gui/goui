package animation

func Linear(progress float64) float64      { return progress }
func EaseInCubic(progress float64) float64 { return progress * progress * progress }
func EaseOutCubic(progress float64) float64 {
	p := 1 - progress
	return 1 - p*p*p
}
func EaseInOutCubic(progress float64) float64 {
	if progress < .5 {
		return 4 * progress * progress * progress
	}
	p := -2*progress + 2
	return 1 - p*p*p/2
}
