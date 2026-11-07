package main

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg time.Time
type typewriterMsg int // generation; stale ticks from an old chain are ignored

type model struct {
	cards    []Card
	anim     AnimState
	music    *MusicPlayer
	width    int
	height   int
	quitting bool
	// backlog reports bytes sent to an SSH client but not yet acknowledged; nil
	// when running locally.
	backlog func() int

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
	readingEpoch  int // incremented on each new reading to ignore stale messages

	// Pages: the reading is shown a page at a time under the cards, so text never
	// scrolls away. A full page waits for space; earlier pages can be revisited.
	pageStart   int   // byte index in readingBuffer where the current page starts
	pageHistory []int // start indices of earlier pages
	pageFull    bool  // current page has no room for the next word
	viewPage    int   // -1 = current page, otherwise index into pageHistory
	twGen       int   // typewriter tick-chain generation
}

func newTextInput() textinput.Model {
	theme := deckTheme()
	ti := textinput.New()
	ti.Placeholder = theme.Placeholder
	ti.CharLimit = 200
	ti.Width = 50
	ti.TextStyle = lipgloss.NewStyle().Foreground(theme.InputText).Background(theme.InputBg)
	ti.PromptStyle = lipgloss.NewStyle().Foreground(theme.InputText).Background(theme.InputBg)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(theme.InputDim).Background(theme.InputBg)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(theme.InputText).Background(theme.InputBg)
	ti.Focus() // sets focus state; cmd discarded (cursor won't blink but input works)
	return ti
}

func initialModel() model {
	cfg, _ := LoadConfig()
	if cfg != nil && cfg.Deck != "" {
		ActiveDeck = cfg.Deck
	} else {
		// No deck configured — pick a random available deck
		if decks := ListDecks(); len(decks) > 0 {
			ActiveDeck = decks[rand.Intn(len(decks))]
		}
	}
	initAudio()
	anim := NewAnimState()
	anim.Phase = PhaseQuestion
	mp := &MusicPlayer{}
	mp.Play(ActiveDeck)
	return model{
		cards:     DrawThree(),
		anim:      anim,
		music:     mp,
		config:    cfg,
		textInput: newTextInput(),
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(frameInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// The screen a reading needs in full: three cards side by side (3 x 28 columns and
// two gaps), and the cards above at least a few lines of text.
const (
	fullViewW = 90
	fullViewH = 34
)

// maxLinkBacklog is how many unacknowledged bytes a client may have before the next
// shuffle frame waits: about two frames, enough to keep a fast link busy at 20 fps
// while a slow one gets fewer frames instead of a growing queue.
const maxLinkBacklog = 64 << 10

const typewriterInterval = 150 * time.Millisecond // ~200 wpm speaking pace

func typewriterCmd(gen int) tea.Cmd {
	return tea.Tick(typewriterInterval, func(time.Time) tea.Msg {
		return typewriterMsg(gen)
	})
}

// startTypewriter begins a new tick chain; any older chain dies at its next tick.
func (m *model) startTypewriter() tea.Cmd {
	m.twGen++
	return typewriterCmd(m.twGen)
}

// nextWordEnd returns the end of the word after index i (leading spaces skipped).
func (m model) nextWordEnd(i int) int {
	b := m.readingBuffer
	for i < len(b) && b[i] == ' ' {
		i++
	}
	for i < len(b) && b[i] != ' ' {
		i++
	}
	return i
}

func (m model) readingWrapW() int {
	w := 80
	if m.width-4 < w {
		w = m.width - 4
	}
	if w < 20 {
		w = 20
	}
	return w
}

// readingAvailH is the number of text lines that fit under the cards.
func (m model) readingAvailH() int {
	h := m.height - 30
	if h < 4 {
		h = 4
	}
	return h
}

func pageText(s string) string { return strings.TrimLeft(s, " \n") }

// fits reports whether the current page, revealed up to end, fits the text area.
func (m model) fits(end int) bool {
	text := pageText(m.readingBuffer[m.pageStart:end])
	return strings.Count(wordWrap(text, m.readingWrapW()), "\n")+1 <= m.readingAvailH()
}

// revealing reports whether the current page is still being typed out.
func (m model) revealing() bool {
	return !m.pageFull && (m.revealIndex < len(m.readingBuffer) || !m.readingDone)
}

// turnPage starts a new page at the first word not yet shown.
func (m *model) turnPage() tea.Cmd {
	m.pageHistory = append(m.pageHistory, m.pageStart)
	i := m.revealIndex
	for i < len(m.readingBuffer) && (m.readingBuffer[i] == ' ' || m.readingBuffer[i] == '\n') {
		i++
	}
	m.pageStart, m.revealIndex = i, i
	m.readingText = ""
	m.pageFull = false
	return m.startTypewriter()
}

// revealPage shows at once as much buffered text as fits on the current page.
func (m *model) revealPage() {
	for m.revealIndex < len(m.readingBuffer) {
		next := m.nextWordEnd(m.revealIndex)
		if !m.fits(next) && m.revealIndex > m.pageStart {
			m.pageFull = true
			break
		}
		m.revealIndex = next
	}
	m.readingText = m.readingBuffer[m.pageStart:m.revealIndex]
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
				m.music.Stop()
				m.quitting = true
				return m, tea.Quit
			case "enter":
				m.question = m.textInput.Value()
				m.readingEpoch++
				m.anim.Start()
				PlaySFX("paper-slide")
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
			m.music.Stop()
			m.quitting = true
			return m, tea.Quit
		case " ", "enter", "pgdown":
			if m.anim.Phase == PhaseReading {
				switch {
				case m.viewPage >= 0: // back on an earlier page: return to the current one
					m.viewPage = -1
				case m.pageFull:
					return m, m.turnPage()
				case m.revealing():
					m.revealPage() // fast readers: show the rest of this page now
				}
				return m, nil
			}
		case "left", "b", "pgup":
			if m.anim.Phase == PhaseReading {
				if m.viewPage < 0 && len(m.pageHistory) > 0 {
					m.viewPage = len(m.pageHistory) - 1
				} else if m.viewPage > 0 {
					m.viewPage--
				}
				return m, nil
			}
		case "right", "f":
			if m.anim.Phase == PhaseReading && m.viewPage >= 0 {
				m.viewPage++
				if m.viewPage >= len(m.pageHistory) {
					m.viewPage = -1
				}
				return m, nil
			}
		case "r":
			// Random deck if none pinned in config
			if m.config == nil || m.config.Deck == "" {
				if decks := ListDecks(); len(decks) > 0 {
					ActiveDeck = decks[rand.Intn(len(decks))]
				}
			}
			m.music.Play(ActiveDeck) // new deck-voiced track
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
			m.pageStart, m.pageHistory, m.pageFull, m.viewPage = 0, nil, false, -1
			m.twGen++ // stop the old reading's typewriter
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.anim.ScreenW = msg.Width
		m.anim.ScreenH = msg.Height
		m.textInput.Width = inputWidth(msg.Width)
		return m, nil

	case tickMsg:
		if m.anim.Phase == PhaseReading {
			return m, nil // no more animation ticks needed
		}
		// Hold the shuffle while the link is still delivering earlier frames: the
		// view stays the same, so nothing new is sent, and the time-based motion
		// catches up on the next frame the link can take.
		if m.anim.Phase == PhaseShuffle && m.backlog != nil && m.backlog() > maxLinkBacklog {
			return m, tickCmd()
		}
		prevDealt := m.anim.CardsDealt
		prevShown := m.anim.CardsShown
		m.anim.Tick()
		if m.anim.CardsDealt > prevDealt {
			PlaySFX("card-flip")
		}
		if m.anim.CardsShown > prevShown {
			PlaySFX("card-flip")
		}
		if m.anim.Phase == PhaseDisplay && m.config != nil {
			m.anim.Phase = PhaseReading
			m.viewPage = -1
			return m, m.startTypewriter()
		}
		if m.anim.Phase == PhaseDisplay {
			return m, nil
		}
		return m, tickCmd()

	case typewriterMsg:
		if int(msg) != m.twGen || m.pageFull {
			return m, nil // stale chain, or the page is full and waits for space
		}
		if m.revealIndex < len(m.readingBuffer) {
			next := m.nextWordEnd(m.revealIndex)
			if !m.fits(next) && m.revealIndex > m.pageStart {
				m.pageFull = true
				return m, nil
			}
			m.revealIndex = next
			m.readingText = m.readingBuffer[m.pageStart:m.revealIndex]
			PlayWordChime(ActiveDeck)
			return m, typewriterCmd(m.twGen)
		}
		if !m.readingDone {
			// Buffer exhausted, wait for more from LLM
			return m, typewriterCmd(m.twGen)
		}
		return m, nil // all text revealed

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
		return fmt.Sprintf("\033]111\a\033[?30h")
	}

	var s strings.Builder

	// Set terminal background (works over SSH too)
	s.WriteString(fmt.Sprintf("\033]11;%s\a", deckTheme().Background))
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
		hint := configHintStyle().Width(m.width).Render("Add a tarot.md file to enable AI readings")
		s.WriteString("\n" + hint)
	}

	// Show help hint during reading or when cards displayed without config
	if m.anim.Phase == PhaseReading {
		var keys []string
		page := len(m.pageHistory) + 1
		if m.viewPage >= 0 {
			page = m.viewPage + 1
			keys = append(keys, "space current page")
		} else if m.pageFull {
			keys = append(keys, "space next page ▾")
		} else if m.revealing() {
			keys = append(keys, "space show page")
		}
		if (m.viewPage < 0 && len(m.pageHistory) > 0) || m.viewPage > 0 {
			keys = append(keys, "← back")
		}
		keys = append(keys, "r reshuffle", "q quit")
		helpText := strings.Join(keys, " · ")
		if len(m.pageHistory) > 0 || m.pageFull {
			helpText = fmt.Sprintf("page %d · %s", page, helpText)
		}
		help := helpStyle().Width(m.width).Render(helpText)
		s.WriteString("\n\n" + help)
	} else if m.anim.Phase == PhaseDisplay {
		help := helpStyle().Width(m.width).Render("r reshuffle · q quit")
		s.WriteString("\n\n" + help)
	}

	if m.anim.Phase == PhaseQuestion {
		return keepTop(s.String(), m.height)
	}
	return s.String()
}

func (m model) renderIdle() string {
	back := GetCardBack()
	return lipgloss.Place(m.width, 24, lipgloss.Center, lipgloss.Center, back)
}

// inputWidth fits the question box to the terminal, leaving room for the "> " prompt.
func inputWidth(termW int) int {
	w := termW - 4
	if w > 50 {
		w = 50
	}
	if w < 10 {
		w = 10
	}
	return w
}

func (m model) renderQuestion() string {
	input := m.textInput.View()
	w := m.textInput.Width + 2 // +2 for "> " prompt
	inputBox := lipgloss.NewStyle().Width(w).MaxHeight(1).Render(input)
	padded := lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).
		PaddingTop(1).Render(inputBox)

	gap := "\n\n\n\n\n"
	if m.width < fullViewW || m.height < fullViewH {
		hint := fmt.Sprintf("small screen: zoom out to about %d x %d to see all three cards", fullViewW, fullViewH)
		padded += "\n\n" + configHintStyle().Width(m.width).Render(hint)
		gap = "\n\n"
	}

	back := GetCardBack()

	return padded + gap + lipgloss.Place(m.width, 0, lipgloss.Center, lipgloss.Top, back)
}

// keepTop cuts a view to the screen's height from the bottom. Left alone, the renderer
// keeps the bottom of a view that's too tall, which would lose the question box.
func keepTop(view string, height int) string {
	lines := strings.Split(view, "\n")
	if height > 0 && len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
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

	// Sort by Z so cards with lower Z draw first (behind)
	sort.Slice(m.anim.WashCards, func(i, j int) bool {
		return m.anim.WashCards[i].Z < m.anim.WashCards[j].Z
	})

	return m.renderShuffleRGB(bufW, bufH)
}

func (m model) renderShuffleRGB(bufW, bufH int) string {
	buf := make([][][3]uint8, bufH)
	for r := range buf {
		buf[r] = make([][3]uint8, bufW) // zero-value = (0,0,0) = transparent
	}

	grid := m.anim.WashRGB
	gridH := len(grid)
	gridW := 0
	if gridH > 0 {
		gridW = len(grid[0])
	}
	for _, wc := range m.anim.WashCards {
		sx := int(wc.X)
		sy := int(wc.Y)
		for py := 0; py < gridH; py++ {
			dy := sy + py
			if dy < 0 || dy >= bufH {
				continue
			}
			for px := 0; px < gridW; px++ {
				dx := sx + px
				if dx < 0 || dx >= bufW {
					continue
				}
				p := grid[py][px]
				if p[0] != 0 || p[1] != 0 || p[2] != 0 {
					buf[dy][dx] = p
				}
			}
		}
	}

	return RenderRGBPixelBuffer(buf)
}

func (m model) renderCards() string {
	var cardViews [3]string

	// Card width from the card back frame
	cardW := 28 // default
	if len(m.anim.WashRGB) > 0 && len(m.anim.WashRGB[0]) > 0 {
		cardW = len(m.anim.WashRGB[0])
	}

	for i := 0; i < 3; i++ {
		if !m.anim.CardVisible(i) {
			// Empty placeholder matching card dimensions
			cardViews[i] = strings.Repeat(" ", cardW)
			continue
		}

		if m.anim.CardRevealed(i) {
			// Show card face (name is embedded in the card art)
			cardViews[i] = GetCardArt(m.cards[i])
		} else {
			// Show card back
			cardViews[i] = GetCardBack()
		}
	}

	gap := "   "
	cardRow := lipgloss.JoinHorizontal(lipgloss.Top, cardViews[0], gap, cardViews[1], gap, cardViews[2])

	return lipgloss.Place(m.width, 0, lipgloss.Center, lipgloss.Top, cardRow)
}

func (m model) renderReading() string {
	if m.readingErr != nil {
		wait := readingWaitStyle().Width(m.width).Render("✦ The oracle is silent ✦")
		errMsg := readingErrStyle().Width(m.width).Render(m.readingErr.Error())
		return wait + "\n" + errMsg
	}

	text := pageText(m.readingText)
	current := m.viewPage < 0
	if !current {
		end := m.pageStart
		if m.viewPage+1 < len(m.pageHistory) {
			end = m.pageHistory[m.viewPage+1]
		}
		text = pageText(m.readingBuffer[m.pageHistory[m.viewPage]:end])
	}
	if text == "" && m.readingBuffer == "" {
		return readingWaitStyle().Width(m.width).Render("✦ Consulting the oracle... ✦")
	}
	if text == "" {
		return ""
	}

	wrapped := wordWrap(text, m.readingWrapW())

	// Cursor while this page is still being typed out
	if current && m.revealing() {
		wrapped += readingCursorStyle().Render("█")
	}

	rendered := readingStyle().Render(wrapped)

	// Pages are sized to fit; after a resize, clip rather than scroll
	lines := strings.Split(rendered, "\n")
	if availH := m.readingAvailH(); len(lines) > availH {
		lines = lines[:availH]
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
