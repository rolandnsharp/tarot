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
	case "plotinus":
		return [3]uint8{15, 10, 40} // deep indigo void
	case "ukiyoe":
		return [3]uint8{20, 25, 60} // deep indigo aizuri blue
	case "aztec":
		return [3]uint8{45, 55, 30} // dark jungle/jade tone
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

// ListDecks returns names of available decks from the embedded filesystem.
func ListDecks() []string {
	var decks []string
	entries, err := fs.ReadDir(decksFS, "decks")
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() {
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
	case "plotinus":
		return "You are reading from the Monad of the Absolute deck — gold and white light emanating from deep indigo void, concentric rings of emanation, sacred geometry, and Greek key borders. Your tone is that of a Neoplatonic philosopher-mystic, speaking of The One, Nous, and Psyche — the soul's descent into matter and its longing to return. You weave metaphors of light overflowing from an infinite source, each card a stage in the great procession and return."
	case "ukiyoe":
		return "You are reading from a ukiyo-e floating world deck — indigo aizuri blue, vermillion red, pale cream, and bold black outlines in the style of Japanese woodblock prints. Your tone is that of an Edo-period diviner seated in a teahouse, weaving wisdom through images of waves, cherry blossoms, samurai, and shrine maidens. You speak with poetic restraint and seasonal awareness, finding meaning in the transient beauty of the floating world."
	case "aztec":
		return "You are reading from a Mesoamerican oracle deck — obsidian black, jade green, turquoise blue, sun gold, and blood red, carved in the angular geometric style of Aztec and Maya glyphs. Your tone is that of an ancient daykeeper reading the Tonalpohualli, speaking of the five suns, the feathered serpent Quetzalcoatl, and the sacred cenote. You weave prophecy through images of stepped pyramids, jaguar warriors, and the eternal calendar stone, finding cosmic order in sacrifice and renewal."
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
