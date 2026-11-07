package main

import (
	"strings"
	"testing"
)

// On any screen the question box must show, with its prompt, within the screen.
func TestQuestionBoxFitsSmallScreens(t *testing.T) {
	ActiveDeck = "neon"
	for _, size := range [][2]int{{40, 20}, {50, 12}, {100, 40}} {
		w, h := size[0], size[1]
		m := initialModel()
		m.width, m.height = w, h
		m.textInput.Width = inputWidth(w)
		view := m.View()
		lines := strings.Split(view, "\n")
		if len(lines) > h {
			t.Errorf("%dx%d: view has %d lines", w, h, len(lines))
		}
		if !strings.Contains(view, "uery the neon grid") {
			t.Errorf("%dx%d: no prompt in view", w, h)
		}
		if hint := strings.Contains(view, "zoom out"); hint != (w < fullViewW || h < fullViewH) {
			t.Errorf("%dx%d: zoom hint shown = %v", w, h, hint)
		}
	}
}
