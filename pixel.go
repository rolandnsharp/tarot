package main

import (
	"fmt"
	"strings"
)

// Retro-game color palette: single hex char → RGB
var Palette = map[byte][3]uint8{
	'.': {0, 0, 0},       // transparent (use terminal default)
	'1': {26, 26, 46},    // dark outline
	'2': {245, 230, 211},  // cream/card bg
	'3': {255, 51, 51},   // red
	'4': {34, 34, 68},    // dark blue
	'5': {255, 215, 0},   // gold
	'6': {245, 198, 160}, // skin
	'7': {139, 47, 201},  // purple
	'8': {46, 204, 113},  // green
	'9': {74, 158, 255},  // blue
	'a': {184, 150, 12},  // dark gold
	'b': {61, 30, 109},   // dark purple
	'c': {255, 255, 255}, // white
	'd': {136, 136, 136}, // gray
	'e': {255, 136, 0},   // orange
	'f': {102, 51, 51},   // brown
}

const resetCode = "\033[0m"

func fgColor(r, g, b uint8) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
}

func bgColor(r, g, b uint8) string {
	return fmt.Sprintf("\033[48;2;%d;%d;%dm", r, g, b)
}

// RenderPixelArt takes a grid of palette chars (rows separated by newlines)
// and renders using half-block characters. Each pair of rows becomes one
// terminal row: top pixel = foreground ▀, bottom pixel = background.
// '.' means transparent (terminal default).
func RenderPixelArt(art string) string {
	lines := strings.Split(art, "\n")
	// Remove leading empty line if present
	if len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	// Pad to even number of rows
	if len(lines)%2 != 0 {
		lines = append(lines, "")
	}

	var sb strings.Builder
	for row := 0; row < len(lines); row += 2 {
		top := lines[row]
		bot := lines[row+1]

		// Pad shorter line
		maxLen := len(top)
		if len(bot) > maxLen {
			maxLen = len(bot)
		}
		for len(top) < maxLen {
			top += "."
		}
		for len(bot) < maxLen {
			bot += "."
		}

		for col := 0; col < maxLen; col++ {
			tc := top[col]
			bc := bot[col]

			topRGB, topOK := Palette[tc]
			botRGB, botOK := Palette[bc]

			if tc == '.' && bc == '.' {
				sb.WriteString(" ")
			} else if tc == '.' {
				// Only bottom pixel: use ▄ with fg=bottom
				sb.WriteString(fgColor(botRGB[0], botRGB[1], botRGB[2]))
				sb.WriteString("▄")
				sb.WriteString(resetCode)
			} else if bc == '.' {
				// Only top pixel: use ▀ with fg=top
				sb.WriteString(fgColor(topRGB[0], topRGB[1], topRGB[2]))
				sb.WriteString("▀")
				sb.WriteString(resetCode)
			} else if topOK && botOK {
				// Both pixels: ▀ with fg=top, bg=bottom
				sb.WriteString(fgColor(topRGB[0], topRGB[1], topRGB[2]))
				sb.WriteString(bgColor(botRGB[0], botRGB[1], botRGB[2]))
				sb.WriteString("▀")
				sb.WriteString(resetCode)
			} else {
				sb.WriteString(" ")
			}
		}
		if row+2 < len(lines) {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// BuildCardFrame wraps 24×40 inner art in a 28×44 frame with a 2px rounded-pixel border.
func BuildCardFrame(inner string, borderColor, bgColor byte) string {
	innerLines := strings.Split(inner, "\n")
	if len(innerLines) > 0 && innerLines[0] == "" {
		innerLines = innerLines[1:]
	}

	// Pad/trim inner to exactly 24 wide × 40 tall
	for i := range innerLines {
		for len(innerLines[i]) < 24 {
			innerLines[i] += string(bgColor)
		}
		if len(innerLines[i]) > 24 {
			innerLines[i] = innerLines[i][:24]
		}
	}
	for len(innerLines) < 40 {
		innerLines = append(innerLines, strings.Repeat(string(bgColor), 24))
	}

	bc := string(borderColor)
	bg := string(bgColor)

	var rows []string
	// Top border: 2 rows with rounded corners
	// Row 0: ..BBBBBBBBBBBBBBBBBBBBBBBBBB.. (corners transparent)
	// Row 1: BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB (full width)
	topRow0 := ".." + strings.Repeat(bc, 24) + ".."
	topRow1 := strings.Repeat(bc, 28)
	rows = append(rows, topRow0, topRow1)

	// Inner rows: B B <24 inner> B B
	for _, line := range innerLines {
		row := bc + bg + line + bg + bc
		rows = append(rows, row)
	}

	// Bottom border: 2 rows with rounded corners
	botRow0 := strings.Repeat(bc, 28)
	botRow1 := ".." + strings.Repeat(bc, 24) + ".."
	rows = append(rows, botRow0, botRow1)

	return strings.Join(rows, "\n")
}
