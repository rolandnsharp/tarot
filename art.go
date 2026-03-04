package main

import (
	"strings"
)

// Card back: 28×44 pixel art (Brogue-inspired deep purple with diamond accents)
var cardBackPixels = strings.Join([]string{
	"..777777777777777777777777..", // 28
	"7777777777777777777777777777", // 28
	"77b7b7b7b7b7b7b7b7b7b7b7b777", // 28
	"777b7b7b7b7b7b7b7b7b7b7b7b77", // 28
	"77bbbbbbbbbbbbbbbbbbbbbbbb77", // 28
	"77bbbbbbbbbbbbbbbbbbbbbbbb77", // 28
	"77bbbbbbbbbbbbbbbbbbbbbbbb77", // 28
	"77bbbbbbb7bbbbbbbb7bbbbbbb77", // 28
	"77bbbbbb7b7bbbbbb7b7bbbbbb77", // 28
	"77bbbbb7b7b7bbbb7b7b7bbbbb77", // 28
	"77bbbb7b7b7b7bb7b7b7b7bbbb77", // 28
	"77bbbbb7b7b7b77b7b7b7bbbbb77", // 28
	"77bbbbbb7b7b7bb7b7b7bbbbbb77", // 28
	"77bbbbbbb7b7bbbb7b7bbbbbbb77", // 28
	"77bbbbbbbb7bbbbbb7bbbbbbbb77", // 28
	"77bbbbbbbbbbbbbbbbbbbbbbbb77", // 28
	"77bbbbbbbbbbbbbbbbbbbbbbbb77", // 28
	"77b7bb7bb7bb7bb7bb7bb7bb7b77", // 28
	"77bbbbbbbbbbbbbbbbbbbbbbbb77", // 28
	"77bbbbbbbbbbbb77bbbbbbbbbb77", // 28
	"77bbbbbbbbbbb7bb7bbbbbbbbb77", // 28
	"77bbbbbbbbbb7bbbb7bbbbbbbb77", // 28
	"77bbbbbbbbb7bbbbbb7bbbbbbb77", // 28
	"77bbbbbbbb7bbbbbbbb7bbbbbb77", // 28
	"77bbbbbbbbb7bbbbbb7bbbbbbb77", // 28
	"77bbbbbbbbbb7bbbb7bbbbbbbb77", // 28
	"77bbbbbbbbbbb7bb7bbbbbbbbb77", // 28
	"77bbbbbbbbbbbb77bbbbbbbbbb77", // 28
	"77bbbbbbbbbbbbbbbbbbbbbbbb77", // 28
	"77b7bb7bb7bb7bb7bb7bb7bb7b77", // 28
	"77bbbbbbbbbbbbbbbbbbbbbbbb77", // 28
	"77bbbbbbbbbbbbbbbbbbbbbbbb77", // 28
	"77bbbbbbbb7bbbbbb7bbbbbbbb77", // 28
	"77bbbbbbb7b7bbbb7b7bbbbbbb77", // 28
	"77bbbbbb7b7b7bb7b7b7bbbbbb77", // 28
	"77bbbbb7b7b7b77b7b7b7bbbbb77", // 28
	"77bbbb7b7b7b7bb7b7b7b7bbbb77", // 28
	"77bbbbb7b7b7bbbb7b7b7bbbbb77", // 28
	"77bbbbbb7b7bbbbbb7b7bbbbbb77", // 28
	"77bbbbbbb7bbbbbbbb7bbbbbbb77", // 28
	"77bbbbbbbbbbbbbbbbbbbbbbbb77", // 28
	"77b7b7b7b7b7b7b7b7b7b7b7b777", // 28
	"777b7b7b7b7b7b7b7b7b7b7b7b77", // 28
	"..777777777777777777777777..", // 28
}, "\n")

// Suit pip sprites: 5×7 pixels each
var pipSprites = map[Suit]string{
	Cups: // Heart ♥
		"..X.." +
			".XXX." +
			"XXXXX" +
			"XXXXX" +
			"XXXXX" +
			".XXX." +
			"..X..",
	Wands: // Club ♣
		"..X.." +
			".XXX." +
			"..X.." +
			".XXX." +
			"XXXXX" +
			".XXX." +
			"..X..",
	Swords: // Spade ♠
		"..X.." +
			".XXX." +
			"XXXXX" +
			"XXXXX" +
			"..X.." +
			".XXX." +
			"..X..",
	Pentacles: // Diamond ♦
		"..X.." +
			".XXX." +
			"XXXXX" +
			"XXXXX" +
			"XXXXX" +
			".XXX." +
			"..X..",
}

// 3×5 pixel number glyphs (for corner numbers on pip cards)
var numberGlyphs = map[int]string{
	1:  "X.." + "X.." + "X.." + "X.." + "X..",
	2:  "XXX" + "..X" + "XXX" + "X.." + "XXX",
	3:  "XXX" + "..X" + "XXX" + "..X" + "XXX",
	4:  "X.X" + "X.X" + "XXX" + "..X" + "..X",
	5:  "XXX" + "X.." + "XXX" + "..X" + "XXX",
	6:  "XXX" + "X.." + "XXX" + "X.X" + "XXX",
	7:  "XXX" + "..X" + "..X" + "..X" + "..X",
	8:  "XXX" + "X.X" + "XXX" + "X.X" + "XXX",
	9:  "XXX" + "X.X" + "XXX" + "..X" + "XXX",
	10: "XX." + "XX." + "XX." + "XX." + "XX.", // simplified "10"
}

// Pip positions on a 24×40 inner grid (x,y center of each pip)
// Standard playing card layouts for 1-10
type pipPos struct{ x, y int }

var pipLayouts = map[int][]pipPos{
	1:  {{12, 20}},
	2:  {{12, 8}, {12, 32}},
	3:  {{12, 8}, {12, 20}, {12, 32}},
	4:  {{6, 8}, {18, 8}, {6, 32}, {18, 32}},
	5:  {{6, 8}, {18, 8}, {12, 20}, {6, 32}, {18, 32}},
	6:  {{6, 8}, {18, 8}, {6, 20}, {18, 20}, {6, 32}, {18, 32}},
	7:  {{6, 8}, {18, 8}, {12, 14}, {6, 20}, {18, 20}, {6, 32}, {18, 32}},
	8:  {{6, 8}, {18, 8}, {12, 14}, {6, 20}, {18, 20}, {12, 26}, {6, 32}, {18, 32}},
	9:  {{6, 7}, {18, 7}, {6, 15}, {18, 15}, {12, 20}, {6, 25}, {18, 25}, {6, 33}, {18, 33}},
	10: {{6, 6}, {18, 6}, {12, 11}, {6, 16}, {18, 16}, {6, 24}, {18, 24}, {12, 29}, {6, 34}, {18, 34}},
}

// stampSprite places a 5×7 pip sprite onto a 24×40 grid centered at (cx, cy)
func stampSprite(grid [][]byte, sprite string, cx, cy int, color byte) {
	w, h := 5, 7
	sx := cx - w/2
	sy := cy - h/2
	for py := 0; py < h; py++ {
		for px := 0; px < w; px++ {
			idx := py*w + px
			if idx < len(sprite) && sprite[idx] == 'X' {
				gx := sx + px
				gy := sy + py
				if gx >= 0 && gx < 24 && gy >= 0 && gy < 40 {
					grid[gy][gx] = color
				}
			}
		}
	}
}

// stampGlyph places a 3×5 number glyph at position (sx, sy) on the grid
func stampGlyph(grid [][]byte, glyph string, sx, sy int, color byte) {
	w, h := 3, 5
	for py := 0; py < h; py++ {
		for px := 0; px < w; px++ {
			idx := py*w + px
			if idx < len(glyph) && glyph[idx] == 'X' {
				gx := sx + px
				gy := sy + py
				if gx >= 0 && gx < 24 && gy >= 0 && gy < 40 {
					grid[gy][gx] = color
				}
			}
		}
	}
}

// GeneratePipCard builds the 24×40 inner pixel art for a numbered (1-10) pip card.
func GeneratePipCard(number int, suit Suit) string {
	bg := byte('2') // cream background
	pipColor := suit.PrimaryColor()

	// Create 24×40 grid filled with background
	grid := make([][]byte, 40)
	for y := range grid {
		grid[y] = make([]byte, 24)
		for x := range grid[y] {
			grid[y][x] = bg
		}
	}

	// Stamp corner number (top-left)
	if glyph, ok := numberGlyphs[number]; ok {
		stampGlyph(grid, glyph, 1, 1, pipColor)
	}
	// Stamp corner number (bottom-right, same orientation for simplicity)
	if glyph, ok := numberGlyphs[number]; ok {
		stampGlyph(grid, glyph, 20, 34, pipColor)
	}

	// Stamp pips
	sprite := pipSprites[suit]
	if positions, ok := pipLayouts[number]; ok {
		for _, p := range positions {
			stampSprite(grid, sprite, p.x, p.y, pipColor)
		}
	}

	// Convert grid to string
	var lines []string
	for _, row := range grid {
		lines = append(lines, string(row))
	}
	return strings.Join(lines, "\n")
}

// GetCardArt returns the fully rendered pixel art string for a card face.
func GetCardArt(c Card) string {
	var inner string
	if c.IsMajor() {
		inner = GetMajorArt(c.Number)
	} else if c.Number >= 11 && c.Number <= 14 {
		inner = GetCourtArt(c.Number, c.Suit)
	} else {
		inner = GeneratePipCard(c.Number, c.Suit)
	}
	framed := BuildCardFrame(inner, '5', '2') // gold border, cream bg
	return RenderPixelArt(framed)
}

// GetCardBack returns the fully rendered pixel art string for the card back.
func GetCardBack() string {
	return RenderPixelArt(cardBackPixels)
}
