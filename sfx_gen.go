package main

import (
	"bytes"
	"math"
	"math/rand"
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
