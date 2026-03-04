package main

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg time.Time
type connectMsg struct{}

type model struct {
	cards    []Card
	anim     AnimState
	width    int
	height   int
	quitting bool

	// LLM reading
	config      *Config
	readingCh   <-chan streamMsg
	readingText strings.Builder
	readingDone bool
	readingErr  error
}

func initialModel() model {
	anim := NewAnimState()
	anim.Start()
	cfg, _ := LoadConfig()
	return model{
		cards:  DrawThree(),
		anim:   anim,
		config: cfg,
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
			// Reshuffle and reset reading
			m.cards = DrawThree()
			m.anim = NewAnimState()
			m.anim.Start()
			m.readingCh = nil
			m.readingText.Reset()
			m.readingDone = false
			m.readingErr = nil
			return m, tickCmd()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.anim.ScreenW = msg.Width
		m.anim.ScreenH = msg.Height
		return m, nil

	case tickMsg:
		if m.anim.Phase == PhaseReading {
			return m, nil // no more ticks needed
		}
		m.anim.Tick()
		if m.anim.Phase == PhaseDisplay && m.config != nil {
			// Enter reading phase as soon as cards are revealed
			m.anim.Phase = PhaseReading
			return m, tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
				return connectMsg{}
			})
		}
		if m.anim.Phase == PhaseDisplay {
			// No config — stop ticking
			return m, nil
		}
		return m, tickCmd()

	case connectMsg:
		m.readingCh = startReading(*m.config, m.cards)
		return m, waitForStream(m.readingCh)

	case readingChunkMsg:
		m.readingText.WriteString(msg.Text)
		return m, waitForStream(m.readingCh)

	case readingDoneMsg:
		m.readingDone = true
		m.readingCh = nil
		return m, nil

	case readingErrMsg:
		m.readingErr = msg.Err
		m.readingCh = nil
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

	// Reading text
	if m.anim.Phase == PhaseReading {
		s.WriteString(m.renderReading())
	}

	if m.anim.Phase == PhaseDisplay && m.config == nil {
		hint := configHintStyle.Width(m.width).Render("Add a tarot.md file to enable AI readings")
		s.WriteString("\n" + hint)
	}

	return s.String()
}

func (m model) renderIdle() string {
	back := GetCardBack()
	return lipgloss.Place(m.width, 24, lipgloss.Center, lipgloss.Center, back)
}

func (m model) renderShuffle() string {
	if len(m.anim.WashCards) == 0 || m.width == 0 || m.height == 0 {
		return "\n"
	}

	bufW := m.width
	bufH := (m.height - 5) * 2 // pixel rows (leave room for title/help)
	if bufH < washCardH {
		bufH = washCardH + 4
	}

	// Create pixel buffer filled with transparent
	buf := make([][]byte, bufH)
	for r := range buf {
		row := make([]byte, bufW)
		for c := range row {
			row[c] = '.'
		}
		buf[r] = row
	}

	// Composite each wash card onto the buffer
	grid := m.anim.WashGrid
	for _, wc := range m.anim.WashCards {
		sx := int(wc.X)
		sy := int(wc.Y)
		for py, gridRow := range grid {
			dy := sy + py
			if dy < 0 || dy >= bufH {
				continue
			}
			for px, ch := range gridRow {
				dx := sx + px
				if dx < 0 || dx >= bufW {
					continue
				}
				if ch != '.' {
					buf[dy][dx] = ch
				}
			}
		}
	}

	rendered := RenderPixelBuffer(buf)

	shuffleText := subtitleStyle.Width(m.width).Render("✦ Shuffling... ✦")

	return rendered + "\n" + shuffleText
}

func (m model) renderCards() string {
	var cardViews [3]string
	var nameViews [3]string
	var posViews [3]string

	// Card width is 28 chars (pixel art width)
	cardW := 28

	for i := 0; i < 3; i++ {
		if !m.anim.CardVisible(i) {
			// Empty placeholder matching card dimensions
			cardViews[i] = strings.Repeat(" ", cardW)
			nameViews[i] = strings.Repeat(" ", cardW)
			posViews[i] = strings.Repeat(" ", cardW)
			continue
		}

		if m.anim.CardRevealed(i) {
			// Show card face
			cardViews[i] = GetCardArt(m.cards[i])
			nameViews[i] = cardNameStyle.Width(cardW).Render(m.cards[i].Name)
			posViews[i] = positionStyle.Width(cardW).Render("— " + positions[i] + " —")
		} else {
			// Show card back
			cardViews[i] = GetCardBack()
			nameViews[i] = strings.Repeat(" ", cardW)
			posViews[i] = strings.Repeat(" ", cardW)
		}
	}

	gap := "   "
	cardRow := lipgloss.JoinHorizontal(lipgloss.Top, cardViews[0], gap, cardViews[1], gap, cardViews[2])
	posRow := lipgloss.JoinHorizontal(lipgloss.Top, posViews[0], gap, posViews[1], gap, posViews[2])
	nameRow := lipgloss.JoinHorizontal(lipgloss.Top, nameViews[0], gap, nameViews[1], gap, nameViews[2])

	// Center everything
	content := lipgloss.JoinVertical(lipgloss.Center,
		posRow,
		cardRow,
		nameRow,
	)

	return lipgloss.Place(m.width, 0, lipgloss.Center, lipgloss.Top, content)
}

func (m model) renderReading() string {
	if m.readingErr != nil {
		wait := readingWaitStyle.Width(m.width).Render("✦ The oracle is silent ✦")
		errMsg := readingErrStyle.Width(m.width).Render(m.readingErr.Error())
		return wait + "\n" + errMsg
	}

	text := m.readingText.String()
	if text == "" {
		return readingWaitStyle.Width(m.width).Render("✦ Consulting the oracle... ✦")
	}

	// Word-wrap to a comfortable width
	wrapW := 80
	if m.width-4 < wrapW {
		wrapW = m.width - 4
	}
	if wrapW < 20 {
		wrapW = 20
	}
	wrapped := wordWrap(text, wrapW)

	// Add streaming cursor if not done
	if !m.readingDone {
		wrapped += readingCursorStyle.Render("_")
	}

	rendered := readingStyle.Render(wrapped)

	// Auto-scroll: show only the last N lines that fit available height
	// Reserve space for help text (2 lines)
	availH := m.height - 30
	if availH < 4 {
		availH = 4
	}
	lines := strings.Split(rendered, "\n")
	if len(lines) > availH {
		lines = lines[len(lines)-availH:]
	}

	return lipgloss.Place(m.width, 0, lipgloss.Center, lipgloss.Top, strings.Join(lines, "\n"))
}

func wordWrap(text string, width int) string {
	var result strings.Builder
	for _, paragraph := range strings.Split(text, "\n") {
		if result.Len() > 0 {
			result.WriteByte('\n')
		}
		col := 0
		words := strings.Fields(paragraph)
		for i, word := range words {
			wl := len(word)
			if col+wl > width && col > 0 {
				result.WriteByte('\n')
				col = 0
			} else if i > 0 && col > 0 {
				result.WriteByte(' ')
				col++
			}
			result.WriteString(word)
			col += wl
		}
	}
	return result.String()
}
