package main

import (
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg time.Time

type model struct {
	cards    []Card
	anim     AnimState
	width    int
	height   int
	quitting bool
}

func initialModel() model {
	anim := NewAnimState()
	anim.Start()
	return model{
		cards: DrawThree(),
		anim:  anim,
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(frameInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), tea.WindowSize())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "r":
			// Reshuffle
			m.cards = DrawThree()
			m.anim = NewAnimState()
			m.anim.Start()
			return m, tickCmd()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		m.anim.Tick()
		if m.anim.Phase != PhaseDisplay {
			return m, tickCmd()
		}
		// Once in display phase, keep ticking briefly to settle springs
		settled := true
		for i := 0; i < 3; i++ {
			if math.Abs(m.anim.SpringY[i]) > 0.01 || math.Abs(m.anim.SpringVY[i]) > 0.01 {
				settled = false
			}
		}
		if !settled {
			return m, tickCmd()
		}
		return m, nil
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder

	// Title
	title := titleStyle.Width(m.width).Render("✦ T A R O T ✦")
	s.WriteString(title + "\n")
	subtitle := subtitleStyle.Width(m.width).Render("Three-Card Spread")
	s.WriteString(subtitle + "\n\n")

	switch m.anim.Phase {
	case PhaseIdle:
		s.WriteString(m.renderIdle())
	case PhaseShuffle:
		s.WriteString(m.renderShuffle())
	default:
		s.WriteString(m.renderCards())
	}

	s.WriteString("\n")

	// Help text
	if m.anim.Phase == PhaseDisplay {
		help := helpStyle.Width(m.width).Render("r: new reading  •  q: quit")
		s.WriteString("\n" + help)
	} else {
		s.WriteString("\n")
	}

	return s.String()
}

func (m model) renderIdle() string {
	back := GetCardBack()
	return lipgloss.Place(m.width, 24, lipgloss.Center, lipgloss.Center, back)
}

func (m model) renderShuffle() string {
	frame := m.anim.ShuffleFrame()

	// Create 3 card backs with offset jitter to simulate shuffling
	offsets := [4][3]int{
		{0, 0, 0},
		{2, -1, 1},
		{-1, 2, -2},
		{1, -2, 1},
	}

	var cards [3]string
	for i := 0; i < 3; i++ {
		off := offsets[frame][i]
		padding := strings.Repeat(" ", max(0, off))
		back := GetCardBack()
		cards[i] = padding + back
	}

	// Stack them with slight overlap
	stacked := lipgloss.JoinHorizontal(lipgloss.Top, cards[0], " ", cards[1], " ", cards[2])

	shuffleText := subtitleStyle.Width(m.width).Render("✦ Shuffling... ✦")

	return lipgloss.Place(m.width, 24, lipgloss.Center, lipgloss.Center, stacked) + "\n" + shuffleText
}

func (m model) renderCards() string {
	var cardViews [3]string
	var nameViews [3]string
	var meaningViews [3]string
	var posViews [3]string

	// Card width is 28 chars (pixel art width)
	cardW := 28

	for i := 0; i < 3; i++ {
		if !m.anim.CardVisible(i) {
			// Empty placeholder matching card dimensions
			cardViews[i] = strings.Repeat(" ", cardW)
			nameViews[i] = strings.Repeat(" ", cardW)
			meaningViews[i] = strings.Repeat(" ", cardW)
			posViews[i] = strings.Repeat(" ", cardW)
			continue
		}

		if m.anim.CardRevealed(i) {
			// Show card face
			cardViews[i] = GetCardArt(m.cards[i])
			nameViews[i] = cardNameStyle.Width(cardW).Render(m.cards[i].Name)
			meaningViews[i] = meaningStyle.Width(cardW).Render(m.cards[i].Upright)
			posViews[i] = positionStyle.Width(cardW).Render("— " + positions[i] + " —")
		} else {
			// Show card back
			cardViews[i] = GetCardBack()
			nameViews[i] = strings.Repeat(" ", cardW)
			meaningViews[i] = strings.Repeat(" ", cardW)
			posViews[i] = strings.Repeat(" ", cardW)
		}
	}

	gap := "   "
	cardRow := lipgloss.JoinHorizontal(lipgloss.Top, cardViews[0], gap, cardViews[1], gap, cardViews[2])
	posRow := lipgloss.JoinHorizontal(lipgloss.Top, posViews[0], gap, posViews[1], gap, posViews[2])
	nameRow := lipgloss.JoinHorizontal(lipgloss.Top, nameViews[0], gap, nameViews[1], gap, nameViews[2])
	meaningRow := lipgloss.JoinHorizontal(lipgloss.Top, meaningViews[0], gap, meaningViews[1], gap, meaningViews[2])

	// Center everything
	content := lipgloss.JoinVertical(lipgloss.Center,
		posRow,
		cardRow,
		nameRow,
		meaningRow,
	)

	return lipgloss.Place(m.width, 0, lipgloss.Center, lipgloss.Top, content)
}
