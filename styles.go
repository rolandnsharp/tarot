package main

import (
	"github.com/charmbracelet/lipgloss"
)

// DeckTheme holds all colors for a deck's visual identity.
type DeckTheme struct {
	Background  string // terminal background escape color
	Title       lipgloss.Color
	Subtitle    lipgloss.Color
	Text        lipgloss.Color // reading text, cursor
	Help        lipgloss.Color
	Hint        lipgloss.Color // config/question hints
	Error       lipgloss.Color
	InputText   lipgloss.Color
	InputDim    lipgloss.Color // placeholder
	InputBg     lipgloss.Color // input field background
	Placeholder string         // prompt placeholder text
}

func deckTheme() DeckTheme {
	switch ActiveDeck {
	case "nouveau":
		return DeckTheme{
			Background:  "#1a1008",
			Title:       "#ffd700",
			Subtitle:    "#c4883a",
			Text:        "#f5e6d3",
			Help:        "#a08850",
			Hint:        "#d4b88c",
			Error:       "#c05030",
			InputText:   "#f5e6d3",
			InputDim:    "#7a6840",
			InputBg:     "#251a0c",
			Placeholder: "",
		}
	case "gothic":
		return DeckTheme{
			Background:  "#080810",
			Title:       "#d4aa00",
			Subtitle:    "#8040a0",
			Text:        "#c8b8d8",
			Help:        "#605880",
			Hint:        "#a090b0",
			Error:       "#a03040",
			InputText:   "#c8b8d8",
			InputDim:    "#504868",
			InputBg:     "#101018",
			Placeholder: "",
		}
	case "diffusion":
		return DeckTheme{
			Background:  "#100a04",
			Title:       "#ffd700",
			Subtitle:    "#c07830",
			Text:        "#f0e0c8",
			Help:        "#a08850",
			Hint:        "#d0b880",
			Error:       "#c05030",
			InputText:   "#f0e0c8",
			InputDim:    "#7a6840",
			InputBg:     "#1a1208",
			Placeholder: "",
		}
	case "ember":
		return DeckTheme{
			Background:  "#0a0604",
			Title:       "#e8a020",
			Subtitle:    "#c06020",
			Text:        "#e8c888",
			Help:        "#906828",
			Hint:        "#c0a060",
			Error:       "#c04020",
			InputText:   "#e8c888",
			InputDim:    "#604820",
			InputBg:     "#140c06",
			Placeholder: "",
		}
	case "neon":
		return DeckTheme{
			Background:  "#020208",
			Title:       "#00dcc8",
			Subtitle:    "#ff00aa",
			Text:        "#00ffcc",
			Help:        "#0088aa",
			Hint:        "#00aa88",
			Error:       "#ff2060",
			InputText:   "#00ffcc",
			InputDim:    "#005544",
			InputBg:     "#080810",
			Placeholder: "",
		}
	default: // cyberpunk
		return DeckTheme{
			Background:  "#0a0510",
			Title:       "#ffd700",
			Subtitle:    "#8b2fc9",
			Text:        "#e0d0f0",
			Help:        "#8878a8",
			Hint:        "#d0b880",
			Error:       "#d05080",
			InputText:   "#e0d0f0",
			InputDim:    "#6b5b7b",
			InputBg:     "#110a1a",
			Placeholder: "",
		}
	}
}

func titleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(deckTheme().Title).
		Bold(true).
		Align(lipgloss.Center)
}

func subtitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(deckTheme().Subtitle).
		Italic(true).
		Align(lipgloss.Center)
}

func helpStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(deckTheme().Help).
		Align(lipgloss.Center)
}

func readingStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(deckTheme().Text).
		PaddingLeft(2).
		PaddingRight(2)
}

func readingCursorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(deckTheme().Text).
		Bold(true)
}

func readingWaitStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(deckTheme().Text).
		Italic(true).
		Align(lipgloss.Center)
}

func readingErrStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(deckTheme().Error).
		Italic(true).
		Align(lipgloss.Center)
}

func configHintStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(deckTheme().Hint).
		Italic(true).
		Align(lipgloss.Center)
}

func questionHintStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(deckTheme().Help).
		Italic(true).
		Align(lipgloss.Center)
}
