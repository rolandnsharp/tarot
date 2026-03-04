package main

import (
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg time.Time
type typewriterMsg time.Time

type model struct {
	cards    []Card
	anim     AnimState
	width    int
	height   int
	quitting bool

	// Question input
	question  string
	textInput textinput.Model

	// LLM reading
	config        *Config
	readingCh     <-chan streamMsg
	readingBuffer string // full text received from LLM so far
	readingText   string // text revealed to user (typewriter)
	revealIndex   int    // how many bytes of buffer are revealed
	readingDone   bool
	readingErr    error
	readingEpoch  int    // incremented on each new reading to ignore stale messages
}

func newTextInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "ask the cards a question ... or don't ..."
	ti.CharLimit = 200
	ti.Width = 50
	ti.Focus() // sets focus state; cmd discarded (cursor won't blink but input works)
	return ti
}

func initialModel() model {
	anim := NewAnimState()
	anim.Phase = PhaseQuestion
	cfg, _ := LoadConfig()
	return model{
		cards:     DrawThree(),
		anim:      anim,
		config:    cfg,
		textInput: newTextInput(),
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(frameInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

const typewriterInterval = 150 * time.Millisecond // ~200 wpm speaking pace

func typewriterCmd() tea.Cmd {
	return tea.Tick(typewriterInterval, func(t time.Time) tea.Msg {
		return typewriterMsg(t)
	})
}

func (m model) Init() tea.Cmd {
	return tea.WindowSize()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// In question phase, route keys to text input
		if m.anim.Phase == PhaseQuestion {
			switch msg.String() {
			case "ctrl+c", "esc":
				m.quitting = true
				return m, tea.Quit
			case "enter":
				m.question = m.textInput.Value()
				m.readingEpoch++
				m.anim.Start()
				cmds := []tea.Cmd{tickCmd()}
				if m.config != nil {
					m.readingCh = startReading(*m.config, m.cards, m.question)
					cmds = append(cmds, waitForStream(m.readingCh, m.readingEpoch))
				}
				return m, tea.Batch(cmds...)
			}
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "r":
			// Reshuffle and reset reading
			m.cards = DrawThree()
			m.anim = NewAnimState()
			m.anim.Phase = PhaseQuestion
			m.anim.ScreenW = m.width
			m.anim.ScreenH = m.height
			m.question = ""
			m.textInput = newTextInput()
			m.readingCh = nil
			m.readingBuffer = ""
			m.readingText = ""
			m.revealIndex = 0
			m.readingDone = false
			m.readingErr = nil
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.anim.ScreenW = msg.Width
		m.anim.ScreenH = msg.Height
		return m, nil

	case tickMsg:
		if m.anim.Phase == PhaseReading {
			return m, nil // no more animation ticks needed
		}
		m.anim.Tick()
		if m.anim.Phase == PhaseDisplay && m.config != nil {
			m.anim.Phase = PhaseReading
			return m, typewriterCmd()
		}
		if m.anim.Phase == PhaseDisplay {
			return m, nil
		}
		return m, tickCmd()

	case typewriterMsg:
		if m.revealIndex < len(m.readingBuffer) {
			// Skip any leading whitespace, then advance to end of next word
			i := m.revealIndex
			for i < len(m.readingBuffer) && m.readingBuffer[i] == ' ' {
				i++
			}
			for i < len(m.readingBuffer) && m.readingBuffer[i] != ' ' {
				i++
			}
			m.revealIndex = i
			m.readingText = m.readingBuffer[:m.revealIndex]
			return m, typewriterCmd()
		}
		if !m.readingDone {
			// Buffer exhausted, wait for more from LLM
			return m, typewriterCmd()
		}
		// All text revealed
		m.readingText = m.readingBuffer
		return m, nil

	case streamResult:
		if msg.epoch != m.readingEpoch {
			return m, nil // stale message from previous reading
		}
		switch inner := msg.msg.(type) {
		case readingChunkMsg:
			m.readingBuffer += inner.Text
			return m, waitForStream(m.readingCh, m.readingEpoch)
		case readingDoneMsg:
			m.readingDone = true
			m.readingCh = nil
			return m, nil
		case readingErrMsg:
			m.readingErr = inner.Err
			m.readingCh = nil
			return m, nil
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder

	s.WriteString("\n\n")

	switch m.anim.Phase {
	case PhaseIdle:
		s.WriteString(m.renderIdle())
	case PhaseQuestion:
		s.WriteString(m.renderQuestion())
	case PhaseShuffle:
		s.WriteString(m.renderShuffle())
	default:
		s.WriteString(m.renderCards())
	}

	s.WriteString("\n")

	// Reading text
	if m.anim.Phase == PhaseReading {
		s.WriteString("\n" + m.renderReading())
	}

	if m.anim.Phase == PhaseDisplay && m.config == nil {
		hint := configHintStyle.Width(m.width).Render("Add a tarot.md file to enable AI readings")
		s.WriteString("\n" + hint)
	}

	// Show help hint when reading is fully revealed or cards displayed without config
	if (m.anim.Phase == PhaseReading && m.readingDone && m.revealIndex >= len(m.readingBuffer)) ||
		(m.anim.Phase == PhaseDisplay) {
		help := helpStyle.Width(m.width).Render("r reshuffle · q quit")
		s.WriteString("\n\n" + help)
	}

	return s.String()
}

func (m model) renderIdle() string {
	back := GetCardBack()
	return lipgloss.Place(m.width, 24, lipgloss.Center, lipgloss.Center, back)
}

func (m model) renderQuestion() string {
	input := m.textInput.View()
	content := lipgloss.JoinVertical(lipgloss.Center, input)
	return lipgloss.Place(m.width, 6, lipgloss.Center, lipgloss.Center, content)
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

	// Sort by Z so cards with lower Z draw first (behind)
	sort.Slice(m.anim.WashCards, func(i, j int) bool {
		return m.anim.WashCards[i].Z < m.anim.WashCards[j].Z
	})

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

	return rendered
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
			nameViews[i] = cardNameStyle.Width(cardW).Render("— " + m.cards[i].Name + " —")
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

	text := m.readingText
	if text == "" && m.readingBuffer == "" {
		return readingWaitStyle.Width(m.width).Render("✦ Consulting the oracle... ✦")
	}
	if text == "" {
		return ""
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

	// Add cursor if still revealing
	if m.revealIndex < len(m.readingBuffer) || !m.readingDone {
		wrapped += readingCursorStyle.Render("█")
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
