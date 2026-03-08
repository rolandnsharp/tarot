package main

import (
	"io"
	"math"
	"math/rand"
	"sync"

	"github.com/ebitengine/oto/v3"
)

const (
	sampleRate = 48000
	channels   = 2
	bitDepth   = 2 // 16-bit = 2 bytes
)

var (
	otoCtx     *oto.Context
	otoCtxOnce sync.Once
)

// initAudio creates the shared Oto audio context. Call once at startup.
// Skipped when silentMode is set.
func initAudio() {
	if silentMode {
		return
	}
	otoCtxOnce.Do(func() {
		op := &oto.NewContextOptions{
			SampleRate:   sampleRate,
			ChannelCount: channels,
			Format:       oto.FormatSignedInt16LE,
		}
		var readyCh chan struct{}
		var err error
		otoCtx, readyCh, err = oto.NewContext(op)
		if err != nil {
			return
		}
		<-readyCh
	})
}

// --- Per-deck voice configuration ---

type deckVoice struct {
	roots      []float64    // candidate root freqs (one picked randomly)
	harmonics  [][2]float64 // {freq_ratio, amplitude} layered on root
	lfoRate    float64      // breathing LFO speed (Hz)
	lfoDepth   float64      // 0–1, amplitude modulation depth
	scale      []int        // semitone intervals for melody notes
	noteGapMin float64      // seconds between melody notes (min)
	noteGapMax float64      // seconds between melody notes (max)
	attackMs   float64      // melody note attack (ms)
	decayMs    float64      // melody note decay (ms)
	melodyVol  float64      // melody amplitude
}

var defaultVoice = deckVoice{
	roots:      []float64{73.42, 82.41, 87.31, 110.0},
	harmonics:  [][2]float64{{1, 0.15}, {2, 0.08}, {1.498, 0.06}, {1.002, 0.04}},
	lfoRate:    0.1,
	lfoDepth:   0.2,
	scale:      []int{0, 2, 4, 7, 9, 12, 14, 16},
	noteGapMin: 1,
	noteGapMax: 4,
	attackMs:   200,
	decayMs:    800,
	melodyVol:  0.12,
}

var deckVoices = map[string]deckVoice{
	"cyberpunk": {
		roots:      []float64{110.0, 138.59}, // A2, C#3
		harmonics:  [][2]float64{{1, 0.15}, {2, 0.08}, {3, 0.05}, {2.01, 0.06}},
		lfoRate:    0.3,
		lfoDepth:   0.25,
		scale:      []int{0, 3, 5, 6, 7, 10, 12, 15},
		noteGapMin: 0.5,
		noteGapMax: 2,
		attackMs:   100,
		decayMs:    400,
		melodyVol:  0.14,
	},
	"nouveau": {
		roots:      []float64{155.56, 103.83}, // Eb3, Ab2
		harmonics:  [][2]float64{{1, 0.15}, {2, 0.08}, {1.5, 0.06}},
		lfoRate:    0.07,
		lfoDepth:   0.2,
		scale:      []int{0, 2, 4, 7, 9, 12, 14, 16},
		noteGapMin: 2,
		noteGapMax: 5,
		attackMs:   250,
		decayMs:    1200,
		melodyVol:  0.10,
	},
	"gothic": {
		roots:      []float64{65.41, 73.42}, // C2, D2
		harmonics:  [][2]float64{{1, 0.15}, {2, 0.08}, {3, 0.04}, {1.5, 0.06}, {2.5, 0.03}},
		lfoRate:    0.04,
		lfoDepth:   0.15,
		scale:      []int{0, 2, 3, 5, 7, 8, 12},
		noteGapMin: 3,
		noteGapMax: 6,
		attackMs:   300,
		decayMs:    1500,
		melodyVol:  0.10,
	},
	"ember": {
		roots:      []float64{65.41, 87.31}, // C2, F2
		harmonics:  [][2]float64{{1, 0.15}, {2, 0.10}, {3, 0.05}, {1.5, 0.04}},
		lfoRate:    0.05,
		lfoDepth:   0.18,
		scale:      []int{0, 2, 4, 7, 9, 12, 14},
		noteGapMin: 3,
		noteGapMax: 6,
		attackMs:   250,
		decayMs:    1200,
		melodyVol:  0.10,
	},
	"neon": {
		roots:      []float64{55.0, 82.41}, // A1, E2
		harmonics:  [][2]float64{{1, 0.12}, {2, 0.10}, {3, 0.06}, {5, 0.03}},
		lfoRate:    0.12,
		lfoDepth:   0.25,
		scale:      []int{0, 3, 5, 7, 10, 12, 15},
		noteGapMin: 1,
		noteGapMax: 3,
		attackMs:   80,
		decayMs:    600,
		melodyVol:  0.10,
	},
}

func voiceForDeck(deck string) deckVoice {
	if v, ok := deckVoices[deck]; ok {
		return v
	}
	return defaultVoice
}

// --- Drone/pad generator ---

type droneReader struct {
	root      float64
	harmonics [][2]float64
	lfoRate   float64
	lfoDepth  float64
	sample    uint64
}

func newDroneReader(voice deckVoice) *droneReader {
	return &droneReader{
		root:      voice.roots[rand.Intn(len(voice.roots))],
		harmonics: voice.harmonics,
		lfoRate:   voice.lfoRate,
		lfoDepth:  voice.lfoDepth,
	}
}

func (d *droneReader) rootFreq() float64 {
	return d.root
}

func (d *droneReader) Read(buf []byte) (int, error) {
	n := len(buf) / (channels * bitDepth) * (channels * bitDepth)
	for i := 0; i < n; i += channels * bitDepth {
		t := float64(d.sample) / float64(sampleRate)

		var val float64
		for _, h := range d.harmonics {
			val += math.Sin(2*math.Pi*d.root*h[0]*t) * h[1]
		}

		// Breathing LFO
		lfo := (1.0 - d.lfoDepth) + d.lfoDepth*math.Sin(2*math.Pi*d.lfoRate*t)
		val *= lfo

		s := int16(val * 32767)
		buf[i] = byte(s)
		buf[i+1] = byte(s >> 8)
		buf[i+2] = byte(s)
		buf[i+3] = byte(s >> 8)
		d.sample++
	}
	return n, nil
}

// --- Melodic layer ---

func semitoneToFreq(root float64, semitones int) float64 {
	return root * math.Pow(2, float64(semitones)/12.0)
}

type melodyReader struct {
	root        float64
	scale       []int
	melodyVol   float64
	sample      uint64
	noteFreq    float64
	noteStart   uint64
	noteDur     uint64
	nextNoteAt  uint64
	noteOn      bool
	attackSamps uint64
	decaySamps  uint64
	gapMinSamps int
	gapRangeSamps int
}

func newMelodyReader(root float64, voice deckVoice) *melodyReader {
	m := &melodyReader{
		root:          root,
		scale:         voice.scale,
		melodyVol:     voice.melodyVol,
		attackSamps:   uint64(voice.attackMs / 1000.0 * sampleRate),
		decaySamps:    uint64(voice.decayMs / 1000.0 * sampleRate),
		gapMinSamps:   int(voice.noteGapMin * sampleRate),
		gapRangeSamps: int((voice.noteGapMax - voice.noteGapMin) * sampleRate),
	}
	m.scheduleNext()
	return m
}

func (m *melodyReader) scheduleNext() {
	gap := m.gapMinSamps
	if m.gapRangeSamps > 0 {
		gap += rand.Intn(m.gapRangeSamps)
	}
	m.nextNoteAt = m.sample + uint64(gap)
	m.noteOn = false
}

func (m *melodyReader) triggerNote() {
	semi := m.scale[rand.Intn(len(m.scale))]
	m.noteFreq = semitoneToFreq(m.root, semi)
	m.noteStart = m.sample
	m.noteDur = m.attackSamps + m.decaySamps
	m.noteOn = true
}

func (m *melodyReader) envelope() float64 {
	elapsed := m.sample - m.noteStart
	if elapsed < m.attackSamps {
		return float64(elapsed) / float64(m.attackSamps)
	}
	decay := elapsed - m.attackSamps
	if decay >= m.decaySamps {
		return 0
	}
	return 1.0 - float64(decay)/float64(m.decaySamps)
}

func (m *melodyReader) Read(buf []byte) (int, error) {
	n := len(buf) / (channels * bitDepth) * (channels * bitDepth)
	for i := 0; i < n; i += channels * bitDepth {
		if !m.noteOn && m.sample >= m.nextNoteAt {
			m.triggerNote()
		}

		var val float64
		if m.noteOn {
			t := float64(m.sample) / float64(sampleRate)
			env := m.envelope()
			val = math.Sin(2*math.Pi*m.noteFreq*t) * env * m.melodyVol

			if m.sample-m.noteStart >= m.noteDur {
				m.scheduleNext()
			}
		}

		s := int16(val * 32767)
		buf[i] = byte(s)
		buf[i+1] = byte(s >> 8)
		buf[i+2] = byte(s)
		buf[i+3] = byte(s >> 8)
		m.sample++
	}
	return n, nil
}

// --- Mixer ---

type mixReader struct {
	sources []io.Reader
	tmp     []byte
}

func newMixReader(sources ...io.Reader) *mixReader {
	return &mixReader{sources: sources}
}

func (m *mixReader) Read(buf []byte) (int, error) {
	if len(m.tmp) < len(buf) {
		m.tmp = make([]byte, len(buf))
	}

	// Zero the output buffer
	for i := range buf {
		buf[i] = 0
	}

	totalRead := 0
	for _, src := range m.sources {
		n, err := src.Read(m.tmp[:len(buf)])
		if n > totalRead {
			totalRead = n
		}
		// Sum int16 samples
		for i := 0; i+1 < n; i += 2 {
			existing := int16(buf[i]) | int16(buf[i+1])<<8
			incoming := int16(m.tmp[i]) | int16(m.tmp[i+1])<<8
			sum := int32(existing) + int32(incoming)
			// Clamp
			if sum > 32767 {
				sum = 32767
			}
			if sum < -32768 {
				sum = -32768
			}
			buf[i] = byte(int16(sum))
			buf[i+1] = byte(int16(sum) >> 8)
		}
		if err != nil && err != io.EOF {
			return totalRead, err
		}
	}
	return totalRead, nil
}
