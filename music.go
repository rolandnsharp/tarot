package main

import (
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// MusicPlayer manages background WAV playback via aplay.
type MusicPlayer struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	stopCh  chan struct{}
	running bool
}

// findMusicDir locates the music/ directory relative to the executable,
// then falls back to the working directory.
func findMusicDir() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Join(filepath.Dir(exe), "music")
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return "music"
}

// pickRandomTrack returns the full path to a random .wav file in the music dir.
func pickRandomTrack() string {
	dir := findMusicDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var wavs []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".wav") {
			wavs = append(wavs, filepath.Join(dir, e.Name()))
		}
	}
	if len(wavs) == 0 {
		return ""
	}
	return wavs[rand.Intn(len(wavs))]
}

// Play picks a random track and loops it in the background.
// Silently does nothing if aplay is not found or no music files exist.
func (p *MusicPlayer) Play() {
	p.Stop()

	track := pickRandomTrack()
	if track == "" {
		return
	}

	aplayPath, err := exec.LookPath("aplay")
	if err != nil {
		return
	}

	p.mu.Lock()
	p.stopCh = make(chan struct{})
	p.running = true
	stopCh := p.stopCh
	p.mu.Unlock()

	go func() {
		for {
			select {
			case <-stopCh:
				return
			default:
			}

			cmd := exec.Command(aplayPath, "-q", track)
			cmd.Stderr = nil
			cmd.Stdout = nil

			p.mu.Lock()
			p.cmd = cmd
			p.mu.Unlock()

			if err := cmd.Start(); err != nil {
				return
			}

			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()

			select {
			case <-stopCh:
				cmd.Process.Kill()
				<-done
				return
			case <-done:
				// Track finished, loop again
			}
		}
	}()
}

// Stop halts any current playback.
func (p *MusicPlayer) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return
	}

	close(p.stopCh)
	p.running = false

	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
		p.cmd = nil
	}
}
