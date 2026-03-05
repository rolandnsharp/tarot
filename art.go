package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ActiveDeck holds the currently selected deck name.
var ActiveDeck string

// deckBorderColor returns the RGB border color appropriate for the active deck.
func deckBorderColor() [3]uint8 {
	switch ActiveDeck {
	case "nouveau":
		return [3]uint8{184, 140, 40} // warm gold
	case "ukiyoe":
		return [3]uint8{40, 50, 100} // indigo
	case "gothic":
		return [3]uint8{40, 35, 50} // dark stone
default:
		return [3]uint8{139, 47, 201} // cyberpunk purple
	}
}

// deckDir returns the base directory for a named deck.
func deckDir() string {
	if ActiveDeck == "" {
		return ""
	}
	// Look for decks/ relative to the executable, then current directory
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	candidates := []string{
		filepath.Join(exeDir, "decks", ActiveDeck),
		filepath.Join("decks", ActiveDeck),
	}
	for _, d := range candidates {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			return d
		}
	}
	return filepath.Join("decks", ActiveDeck) // fallback
}

// cardHexPath returns the .hex file path for a card in the active deck.
func cardHexPath(c Card) string {
	dir := deckDir()
	if dir == "" {
		return ""
	}
	if c.IsMajor() {
		return filepath.Join(dir, "major", fmt.Sprintf("%02d-%s.hex", c.Number, slugify(c.Name)))
	}
	return filepath.Join(dir, "minor", fmt.Sprintf("%s-%02d.hex", strings.ToLower(c.SuitName()), c.Number))
}

// slugify converts a card name to a filename-safe slug.
func slugify(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	// Remove non-alphanumeric except hyphens
	var b strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// ListDecks returns names of available decks in the decks/ directory.
func ListDecks() []string {
	var decks []string
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	searchDirs := []string{
		filepath.Join(exeDir, "decks"),
		"decks",
	}
	seen := make(map[string]bool)
	for _, base := range searchDirs {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() && !seen[e.Name()] {
				seen[e.Name()] = true
				decks = append(decks, e.Name())
			}
		}
	}
	return decks
}

// GetCardArt returns the fully rendered pixel art string for a card face,
// with the card name embedded in the bottom border.
func GetCardArt(c Card) string {
	path := cardHexPath(c)
	if path == "" {
		return ""
	}
	pixels, err := LoadHexCard(path)
	if err != nil {
		return ""
	}
	frame := BuildRGBCardFrame(pixels, deckBorderColor())
	return RenderRGBFrameWithLabel(frame, c.Name)
}

// GetCardBack returns the fully rendered pixel art string for the card back.
func GetCardBack() string {
	dir := deckDir()
	if dir == "" {
		return ""
	}
	pixels, err := LoadHexCard(filepath.Join(dir, "back.hex"))
	if err != nil {
		return ""
	}
	frame := BuildRGBCardFrame(pixels, deckBorderColor())
	return RenderRGBFrame(frame)
}
