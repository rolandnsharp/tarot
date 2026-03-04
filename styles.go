package main

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Balatro-inspired color palette
	colorGold      = lipgloss.Color("#ffd700")
	colorCream     = lipgloss.Color("#f5e6d3")
	colorMagenta   = lipgloss.Color("#8b2fc9")
	colorDimPurple = lipgloss.Color("#2d1150")

	// Card name
	cardNameStyle = lipgloss.NewStyle().
			Foreground(colorGold).
			Bold(true).
			Align(lipgloss.Center)

	// Card meaning
	meaningStyle = lipgloss.NewStyle().
			Foreground(colorCream).
			Italic(true).
			Align(lipgloss.Center)

	// Position labels (Past, Present, Future)
	positionStyle = lipgloss.NewStyle().
			Foreground(colorMagenta).
			Bold(true).
			Align(lipgloss.Center)

	// Title style
	titleStyle = lipgloss.NewStyle().
			Foreground(colorGold).
			Bold(true).
			Align(lipgloss.Center)

	// Subtitle
	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorMagenta).
			Italic(true).
			Align(lipgloss.Center)

	// Help text
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("141")).
			Align(lipgloss.Center)

	// Reading text
	readingStyle = lipgloss.NewStyle().
			Foreground(colorCream).
			PaddingLeft(2).
			PaddingRight(2)

	// Streaming cursor
	readingCursorStyle = lipgloss.NewStyle().
				Foreground(colorGold).
				Bold(true)

	// Waiting message
	readingWaitStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("223")).
				Italic(true).
				Align(lipgloss.Center)

	// Error message
	readingErrStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("168")).
			Italic(true).
			Align(lipgloss.Center)

	// Config hint (shown when no tarot.md)
	configHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("223")).
			Italic(true).
			Align(lipgloss.Center)

	// Question hint (press enter to begin)
	questionHintStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("141")).
				Italic(true).
				Align(lipgloss.Center)

	// Spread positions
	positions = []string{"PAST", "PRESENT", "FUTURE"}
)
