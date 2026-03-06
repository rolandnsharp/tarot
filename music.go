package main

import (
	"bytes"
	"io"
	"sync"
	"time"
)

// MusicPlayer manages background procedural audio via Oto.
type MusicPlayer struct {
	mu      sync.Mutex
	player  *otoPlayer
	playing bool
}

// otoPlayer wraps an Oto player and keeps references to the synth
// readers so they aren't garbage collected while playing.
type otoPlayer struct {
	closer io.Closer
}

func (o *otoPlayer) Close() {
	if o.closer != nil {
		o.closer.Close()
	}
}

// Play starts procedural ambient music voiced for the given deck.
func (p *MusicPlayer) Play(deck string) {
	p.Stop()

	if otoCtx == nil {
		return
	}

	voice := voiceForDeck(deck)
	drone := newDroneReader(voice)
	melody := newMelodyReader(drone.rootFreq(), voice)
	mix := newMixReader(drone, melody)

	player := otoCtx.NewPlayer(mix)
	player.Play()

	p.mu.Lock()
	p.player = &otoPlayer{closer: player}
	p.playing = true
	p.mu.Unlock()
}

// Stop halts any current playback.
func (p *MusicPlayer) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.playing {
		return
	}

	if p.player != nil {
		p.player.Close()
		p.player = nil
	}
	p.playing = false
}

// activeSFX tracks playing SFX players to prevent GC.
var activeSFX struct {
	mu      sync.Mutex
	players []io.Closer
}

// PlaySFX plays a one-shot procedural sound effect.
// Supported names: "card-flip", "paper-slide".
func PlaySFX(name string) {
	if otoCtx == nil {
		return
	}

	data := cachedSFX(name)
	if data == nil {
		return
	}

	// Create a new bytes.Reader each time (each playback needs its own read position).
	r := bytes.NewReader(data)

	player := otoCtx.NewPlayer(r)
	player.Play()

	// Hold reference to prevent GC until done.
	activeSFX.mu.Lock()
	activeSFX.players = append(activeSFX.players, player)
	activeSFX.mu.Unlock()

	go func() {
		// Wait until playback finishes.
		for player.IsPlaying() {
			time.Sleep(10 * time.Millisecond)
		}
		// Remove from active list.
		activeSFX.mu.Lock()
		for i, p := range activeSFX.players {
			if p == player {
				activeSFX.players = append(activeSFX.players[:i], activeSFX.players[i+1:]...)
				break
			}
		}
		activeSFX.mu.Unlock()
	}()
}

// PlayWordChime plays a short tonal ping tuned to the active deck's root.
func PlayWordChime(deck string) {
	if otoCtx == nil {
		return
	}

	data := cachedWordChime(deck)
	if data == nil {
		return
	}

	r := bytes.NewReader(data)
	player := otoCtx.NewPlayer(r)
	player.Play()

	activeSFX.mu.Lock()
	activeSFX.players = append(activeSFX.players, player)
	activeSFX.mu.Unlock()

	go func() {
		for player.IsPlaying() {
			time.Sleep(10 * time.Millisecond)
		}
		activeSFX.mu.Lock()
		for i, p := range activeSFX.players {
			if p == player {
				activeSFX.players = append(activeSFX.players[:i], activeSFX.players[i+1:]...)
				break
			}
		}
		activeSFX.mu.Unlock()
	}()
}
