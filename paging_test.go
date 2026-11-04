package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// A long reading on a short terminal must be split into pages that each fit,
// that together hold every word exactly once, and that never scroll.
func TestReadingPages(t *testing.T) {
	words := make([]string, 400)
	for i := range words {
		words[i] = "word"
	}
	text := strings.Join(words[:150], " ") + "\n\n" + strings.Join(words[150:], " ")
	m := model{width: 100, height: 40, viewPage: -1, readingBuffer: text, readingDone: true}

	var pages []string
	for step := 0; step < 10000; step++ {
		next, _ := m.Update(typewriterMsg(m.twGen))
		m = next.(model)
		if m.pageFull || (m.revealIndex == len(m.readingBuffer)) {
			page := pageText(m.readingText)
			if n := strings.Count(wordWrap(page, m.readingWrapW()), "\n") + 1; n > m.readingAvailH() {
				t.Fatalf("page %d has %d lines, room for %d", len(pages)+1, n, m.readingAvailH())
			}
			pages = append(pages, page)
			if !m.pageFull {
				break
			}
			m.turnPage()
		}
	}
	got := len(strings.Fields(strings.Join(pages, " ")))
	if got != 400 {
		t.Fatalf("pages hold %d words, want 400", got)
	}
	if len(pages) < 2 {
		t.Fatalf("expected several pages, got %d", len(pages))
	}

	// Going back shows the first page; space returns to the current one.
	m.anim.Phase = PhaseReading
	next, _ := m.Update(keyMsg("left"))
	m = next.(model)
	if m.viewPage != len(m.pageHistory)-1 {
		t.Fatalf("left: viewPage %d", m.viewPage)
	}
	next, _ = m.Update(keyMsg(" "))
	m = next.(model)
	if m.viewPage != -1 {
		t.Fatalf("space: viewPage %d, want current (-1)", m.viewPage)
	}
}

// Space mid-page reveals the rest of the page at once.
func TestSpaceRevealsPage(t *testing.T) {
	m := model{width: 100, height: 40, viewPage: -1, readingBuffer: strings.Repeat("word ", 300), readingDone: true}
	m.anim.Phase = PhaseReading
	next, _ := m.Update(keyMsg(" "))
	m = next.(model)
	if !m.pageFull || m.revealIndex == 0 {
		t.Fatalf("space did not fill the page: full=%v reveal=%d", m.pageFull, m.revealIndex)
	}
}

func keyMsg(k string) tea.KeyMsg {
	switch k {
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}
