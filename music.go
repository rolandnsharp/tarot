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

// findDir locates a named directory relative to the executable,
// then falls back to the working directory.
func findDir(name string) string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Join(filepath.Dir(exe), name)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return name
}

// PlaySFX plays a one-shot sound effect from the sounds/ directory.
// Silently does nothing if aplay or the file is not found.
func PlaySFX(name string) {
	aplayPath, err := exec.LookPath("aplay")
	if err != nil {
		return
	}
	path := filepath.Join(findDir("sounds"), name)
	if _, err := os.Stat(path); err != nil {
		return
	}
	cmd := exec.Command(aplayPath, "-q", path)
	cmd.Stderr = nil
	cmd.Stdout = nil
	go cmd.Run()
}

// SFXLoop loops a sound effect from the sounds/ directory until stopped.
type SFXLoop struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	stopCh  chan struct{}
	running bool
}

func NewSFXLoop(name string) *SFXLoop {
	s := &SFXLoop{}
	s.Start(name)
	return s
}

func (s *SFXLoop) Start(name string) {
	s.Stop()

	aplayPath, err := exec.LookPath("aplay")
	if err != nil {
		return
	}
	path := filepath.Join(findDir("sounds"), name)
	if _, err := os.Stat(path); err != nil {
		return
	}

	s.mu.Lock()
	s.stopCh = make(chan struct{})
	s.running = true
	stopCh := s.stopCh
	s.mu.Unlock()

	go func() {
		for {
			select {
			case <-stopCh:
				return
			default:
			}

			cmd := exec.Command(aplayPath, "-q", path)
			cmd.Stderr = nil
			cmd.Stdout = nil

			s.mu.Lock()
			s.cmd = cmd
			s.mu.Unlock()

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
			}
		}
	}()
}

func (s *SFXLoop) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	close(s.stopCh)
	s.running = false

	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd = nil
	}
}

func findMusicDir() string {
	return findDir("music")
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
