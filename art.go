package main

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

//go:embed decks
var decksFS embed.FS

// ActiveDeck holds the currently selected deck name.
var ActiveDeck string

// deckBorderColor returns the RGB border color appropriate for the active deck.
func deckBorderColor() [3]uint8 {
	switch ActiveDeck {
	case "nouveau":
		return [3]uint8{184, 140, 40} // warm gold
	case "gothic":
		return [3]uint8{40, 35, 50} // dark stone
	case "diffusion":
		return [3]uint8{140, 110, 40} // warm gold art nouveau
	case "neon":
		return [3]uint8{0, 220, 200} // neon cyan
	case "ember":
		return [3]uint8{200, 140, 30} // warm gold
	default:
		return [3]uint8{139, 47, 201} // cyberpunk purple
	}
}

// deckDir returns the embedded path prefix for the active deck.
func deckDir() string {
	if ActiveDeck == "" {
		return ""
	}
	return path.Join("decks", ActiveDeck)
}

// cardHexPath returns the .hex file path for a card in the active deck.
func cardHexPath(c Card) string {
	dir := deckDir()
	if dir == "" {
		return ""
	}
	if c.IsMajor() {
		return path.Join(dir, "major", fmt.Sprintf("%02d-%s.hex", c.Number, slugify(c.Name)))
	}
	return path.Join(dir, "minor", fmt.Sprintf("%s-%02d.hex", strings.ToLower(c.SuitName()), c.Number))
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

// hiddenDecks are kept on disk but excluded from random selection.
var hiddenDecks = map[string]bool{
	"ember": true,
}

// ListDecks returns names of available decks from the embedded filesystem.
func ListDecks() []string {
	var decks []string
	entries, err := fs.ReadDir(decksFS, "decks")
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() && !hiddenDecks[e.Name()] {
			decks = append(decks, e.Name())
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

// deckDescription returns a short flavor description of the active deck,
// used as the default interpreter personality when none is configured.
func deckDescription() string {
	switch ActiveDeck {
	case "cyberpunk":
		return "You are reading from a cyberpunk neon-lit deck — glowing circuits, scanlines, and electric violet. Your tone is streetwise and electric, mixing tech slang with mystical insight, like a back-alley oracle jacked into the mainframe of fate."
	case "nouveau":
		return "You are reading from an Art Nouveau deck — flowing vines, golden curves, and warm earthy tones. Your tone is lyrical and sensuous, weaving organic metaphors of growth, beauty, and the natural cycle of things, like a fin-de-siècle mystic in a candlelit salon."
	case "gothic":
		return "You are reading from a stained glass cathedral deck — jewel-tone ruby, sapphire, emerald, and amber panes divided by dark lead lines, with rose windows and pointed arches. Your tone is reverent and luminous, as if light itself is speaking through ancient glass, carrying the weight of centuries of devotion and mystery."
	case "diffusion":
		return "You are reading from an Art Nouveau dream deck — flowing organic lines, jewel tones, and golden botanical borders in the style of Alphonse Mucha. Your tone is that of a Parisian salon mystic at the turn of the century, weaving symbolism through images of lush gardens, starlit figures, and ornate scrollwork. You speak with poetic elegance, finding beauty and meaning in the dance of light and shadow."
	case "neon":
		return "You are reading from a neon arcade deck — glowing pixel blocks on pure black, electric cyan, magenta, and gold burning against the void. Your tone is that of a midnight oracle in a retro arcade, speaking in sharp neon flashes of insight. You mix digital mysticism with raw truth, each card a glowing screen in the dark."
	case "ember":
		return "You are reading from an ember deck — warm golden light glowing against deep darkness, like firelight illuminating painted cards. Your tone is that of a fireside storyteller, warm and intimate, speaking in rich earthy metaphors of flame, shadow, and the gold that survives the furnace."
	default:
		return ""
	}
}

// GetCardBack returns the fully rendered pixel art string for the card back.
func GetCardBack() string {
	dir := deckDir()
	if dir == "" {
		return ""
	}
	pixels, err := LoadHexCard(path.Join(dir, "back.hex"))
	if err != nil {
		return ""
	}
	frame := BuildRGBCardFrame(pixels, deckBorderColor())
	return RenderRGBFrame(frame)
}
