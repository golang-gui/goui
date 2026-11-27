package main

import (
	"fmt"
	"image/color"
	"time"
)

// track is one fictional song. Its two colors tint the record label, the
// window's ambient light, the spectrum and the progress fill.
type track struct {
	Title, Artist, Album string
	Length               time.Duration
	Colors               [2]uint32 // RGB hex, light to deep
}

func (t track) colors() albumColors {
	return albumColors{a: rgb(t.Colors[0]), b: rgb(t.Colors[1])}
}

func length(minutes, seconds int) time.Duration {
	return time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
}

var tracks = []track{
	{"Midnight Drive", "Neon Harbor", "Low Tide", length(3, 42), [2]uint32{0x7B61FF, 0xFF6FB5}},
	{"Glass Garden", "Mira Vale", "Hothouse", length(4, 5), [2]uint32{0x1FC8A9, 0x7BD84A}},
	{"Paper Planes", "The Kites", "Weekend Air", length(3, 18), [2]uint32{0xFF8A3D, 0xFFC93C}},
	{"Slow Orbit", "Lumen", "Satellites", length(4, 31), [2]uint32{0x3A8DFF, 0x5CE1E6}},
	{"Velvet Hour", "June Arden", "Velvet Hour", length(3, 56), [2]uint32{0xE0457B, 0x9B5DE5}},
	{"Static Bloom", "Pale Signal", "Bloom", length(3, 27), [2]uint32{0x00B894, 0x0984E3}},
	{"Northern Lines", "Fjell", "Atlas", length(4, 12), [2]uint32{0x5B6CFF, 0x2BD9C4}},
	{"Sunday Loop", "Coffee Club", "Loops", length(2, 58), [2]uint32{0xF2994A, 0xEB5757}},
}

// albumColors is the pair of tones that every tinted surface interpolates.
type albumColors struct{ a, b color.NRGBA }

func lerpColors(from, to albumColors, t float32) albumColors {
	return albumColors{a: mix(from.a, to.a, t), b: mix(from.b, to.b, t)}
}

// player is simulated playback. The position is derived from the wall clock
// while playing, so every frame can read an exact value without a timer.
type player struct {
	index   int
	playing bool
	offset  time.Duration // position when since was recorded
	since   time.Time
	liked   map[int]bool
}

func newPlayer() *player { return &player{liked: map[int]bool{1: true}} }

func (p *player) current() track { return tracks[p.index] }

// position is the playback position at now, within the current track.
func (p *player) position(now time.Time) time.Duration {
	pos := p.offset
	if p.playing && now.After(p.since) {
		pos += now.Sub(p.since)
	}
	return min(max(pos, 0), p.current().Length)
}

func (p *player) toggle(now time.Time) {
	if p.playing {
		p.offset = p.position(now)
	}
	p.playing = !p.playing
	p.since = now
}

// seek moves to a fraction of the current track, clamped to [0, 1].
func (p *player) seek(fraction float64, now time.Time) {
	fraction = min(max(fraction, 0), 1)
	p.offset = time.Duration(fraction * float64(p.current().Length))
	p.since = now
}

// jump starts track i from the beginning; indices wrap around the queue.
func (p *player) jump(i int, now time.Time) {
	n := len(tracks)
	p.index = (i%n + n) % n
	p.offset = 0
	p.since = now
}

func (p *player) next(now time.Time) { p.jump(p.index+1, now) }

// previous restarts the track once it has played a few seconds, as music
// players do; otherwise it returns to the previous track.
func (p *player) previous(now time.Time) {
	if p.position(now) > 3*time.Second {
		p.offset, p.since = 0, now
		return
	}
	p.jump(p.index-1, now)
}

// advance moves past finished tracks, carrying the overrun into the next one,
// and reports whether the current track changed.
func (p *player) advance(now time.Time) bool {
	if !p.playing {
		return false
	}
	changed := false
	for {
		elapsed := p.offset + max(0, now.Sub(p.since))
		length := p.current().Length
		if elapsed < length {
			return changed
		}
		p.index = (p.index + 1) % len(tracks)
		p.offset = 0
		p.since = now.Add(-(elapsed - length))
		changed = true
	}
}

// formatDuration formats whole seconds as m:ss.
func formatDuration(d time.Duration) string {
	s := max(0, int(d/time.Second))
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}
