package main

import (
	"bytes"
	"math"
	"math/rand"
	"sync"
)

// renderSamples pre-renders stereo int16LE samples into a bytes.Reader.
func renderSamples(samples []int16) *bytes.Reader {
	buf := make([]byte, len(samples)*2*channels)
	for i, s := range samples {
		off := i * channels * bitDepth
		// Left
		buf[off] = byte(s)
		buf[off+1] = byte(s >> 8)
		// Right
		buf[off+2] = byte(s)
		buf[off+3] = byte(s >> 8)
	}
	return bytes.NewReader(buf)
}

// newCardFlipSound generates a soft ~180ms thud: gentle filtered noise with a muted tone.
func newCardFlipSound() *bytes.Reader {
	n := int(0.18 * sampleRate)
	samples := make([]int16, n)
	var prevOut float64
	for i := range samples {
		t := float64(i) / float64(n)
		// Smooth fade-in then long decay — avoids the harsh snap
		var env float64
		if t < 0.05 {
			env = t / 0.05
		} else {
			env = 1.0 - (t-0.05)/0.95
		}
		env *= env // gentle curve

		noise := rand.Float64()*2 - 1
		// Low-pass filter the noise for a softer texture
		alpha := 0.08
		prevOut = prevOut*(1-alpha) + noise*alpha

		freq := 400.0 - 200.0*t
		tSec := float64(i) / float64(sampleRate)
		tone := math.Sin(2 * math.Pi * freq * tSec)

		val := (prevOut*0.5 + tone*0.5) * env * 0.35
		samples[i] = int16(val * 32767)
	}
	return renderSamples(samples)
}

// newPaperSlideSound generates a ~300ms gentle filtered noise swoosh.
func newPaperSlideSound() *bytes.Reader {
	n := int(0.30 * sampleRate)
	samples := make([]int16, n)
	var prevOut float64
	for i := range samples {
		t := float64(i) / float64(n)

		// Softer envelope with gradual ramps
		var env float64
		switch {
		case t < 0.3:
			env = t / 0.3
		case t < 0.5:
			env = 1.0
		default:
			env = (1.0 - t) / 0.5
		}

		noise := rand.Float64()*2 - 1
		alpha := 0.08 // heavier filtering for softer sound
		prevOut = prevOut*(1-alpha) + noise*alpha

		val := prevOut * env * 0.3
		samples[i] = int16(val * 32767)
	}
	return renderSamples(samples)
}

// newWordChimeSound generates a ~60ms tonal ping tuned to a harmonic of the given root.
func newWordChimeSound(root float64) *bytes.Reader {
	n := int(0.06 * sampleRate)
	samples := make([]int16, n)
	// Target ~400-800 Hz for an audible bell-like chime regardless of root.
	// Pick the lowest harmonic that lands at or above 400 Hz.
	harm := math.Ceil(400.0 / root)
	if rand.Intn(3) == 0 {
		harm += 1.0 // occasional higher ping for variety
	}
	freq := root * harm * (1.0 + (rand.Float64()-0.5)*0.02) // +/- 1% detune
	for i := range samples {
		t := float64(i) / float64(n)
		// Fast attack (5ms), smooth exponential decay
		var env float64
		attackT := 0.08 // 5ms / 60ms ≈ 0.08
		if t < attackT {
			env = t / attackT
		} else {
			// Exponential decay for natural ring-out
			env = math.Exp(-6.0 * (t - attackT))
		}

		tSec := float64(i) / float64(sampleRate)
		val := math.Sin(2*math.Pi*freq*tSec) * env * 0.10
		samples[i] = int16(val * 32767)
	}
	return renderSamples(samples)
}

// sfxBytes pre-renders an SFX into a byte slice for repeated playback.
func sfxBytes(name string) []byte {
	var r *bytes.Reader
	switch name {
	case "card-flip":
		r = newCardFlipSound()
	case "paper-slide":
		r = newPaperSlideSound()
	default:
		return nil
	}
	buf := make([]byte, r.Len())
	_, _ = r.Read(buf)
	return buf
}

// sfxCache holds pre-rendered SFX byte slices so we only generate once.
var sfxCache struct {
	once    [2]bool
	buffers [2][]byte
}

func cachedSFX(name string) []byte {
	idx := -1
	switch name {
	case "card-flip":
		idx = 0
	case "paper-slide":
		idx = 1
	}
	if idx < 0 {
		return nil
	}
	if !sfxCache.once[idx] {
		sfxCache.buffers[idx] = sfxBytes(name)
		sfxCache.once[idx] = true
	}
	return sfxCache.buffers[idx]
}

// chimeCache holds pre-rendered word chime bytes keyed by deck name.
var chimeCache struct {
	mu   sync.Mutex
	data map[string][]byte
}

// cachedWordChime returns a pre-rendered chime for the given deck.
// A new random chime is generated on first call per deck.
func cachedWordChime(deck string) []byte {
	chimeCache.mu.Lock()
	defer chimeCache.mu.Unlock()
	if chimeCache.data == nil {
		chimeCache.data = make(map[string][]byte)
	}
	if buf, ok := chimeCache.data[deck]; ok {
		return buf
	}
	voice := voiceForDeck(deck)
	root := voice.roots[rand.Intn(len(voice.roots))]
	r := newWordChimeSound(root)
	buf := make([]byte, r.Len())
	_, _ = r.Read(buf)
	chimeCache.data[deck] = buf
	return buf
}
