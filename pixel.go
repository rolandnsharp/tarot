package main

import (
	"encoding/hex"
	"fmt"
	"os"
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

// RenderRGBCard takes a 40×24 RGB pixel grid and renders it as half-block terminal art.
// Each pair of rows becomes one terminal row: top pixel = foreground ▀, bottom pixel = background.
func RenderRGBCard(pixels [40][24][3]uint8) string {
	var sb strings.Builder
	for row := 0; row < 40; row += 2 {
		for col := 0; col < 24; col++ {
			tr, tg, tb := pixels[row][col][0], pixels[row][col][1], pixels[row][col][2]
			br, bg, bb := pixels[row+1][col][0], pixels[row+1][col][1], pixels[row+1][col][2]
			sb.WriteString(fgColor(tr, tg, tb))
			sb.WriteString(bgColor(br, bg, bb))
			sb.WriteString("▀")
			sb.WriteString(resetCode)
		}
		if row+2 < 40 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// BuildRGBCardFrame wraps a 24×40 RGB pixel grid in a 28×44 frame with a 2px rounded border.
func BuildRGBCardFrame(inner [40][24][3]uint8, borderColor [3]uint8) [44][28][3]uint8 {
	var frame [44][28][3]uint8
	bc := borderColor

	// Fill entire frame with border color first
	for y := 0; y < 44; y++ {
		for x := 0; x < 28; x++ {
			frame[y][x] = bc
		}
	}

	// Clear corners for rounded effect (row 0 and row 43)
	frame[0][0] = [3]uint8{0, 0, 0}
	frame[0][1] = [3]uint8{0, 0, 0}
	frame[0][26] = [3]uint8{0, 0, 0}
	frame[0][27] = [3]uint8{0, 0, 0}
	frame[43][0] = [3]uint8{0, 0, 0}
	frame[43][1] = [3]uint8{0, 0, 0}
	frame[43][26] = [3]uint8{0, 0, 0}
	frame[43][27] = [3]uint8{0, 0, 0}

	// Place inner art at offset (2,2) with 1px padding
	for y := 0; y < 40; y++ {
		for x := 0; x < 24; x++ {
			frame[y+2][x+2] = inner[y][x]
		}
	}

	return frame
}

// RenderRGBFrame renders a 44×28 framed RGB grid as half-block terminal art.
// Pixels with RGB (0,0,0) are treated as transparent (terminal default).
func RenderRGBFrame(frame [44][28][3]uint8) string {
	var sb strings.Builder
	for row := 0; row < 44; row += 2 {
		for col := 0; col < 28; col++ {
			top := frame[row][col]
			bot := frame[row+1][col]
			topTransparent := top[0] == 0 && top[1] == 0 && top[2] == 0
			botTransparent := bot[0] == 0 && bot[1] == 0 && bot[2] == 0

			if topTransparent && botTransparent {
				sb.WriteString(" ")
			} else if topTransparent {
				sb.WriteString(fgColor(bot[0], bot[1], bot[2]))
				sb.WriteString("▄")
				sb.WriteString(resetCode)
			} else if botTransparent {
				sb.WriteString(fgColor(top[0], top[1], top[2]))
				sb.WriteString("▀")
				sb.WriteString(resetCode)
			} else {
				sb.WriteString(fgColor(top[0], top[1], top[2]))
				sb.WriteString(bgColor(bot[0], bot[1], bot[2]))
				sb.WriteString("▀")
				sb.WriteString(resetCode)
			}
		}
		if row+2 < 44 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// ParsePixelGrid converts a newline-separated palette string into a 2D byte grid.
func ParsePixelGrid(art string) [][]byte {
	lines := strings.Split(art, "\n")
	grid := make([][]byte, len(lines))
	for i, line := range lines {
		grid[i] = []byte(line)
	}
	return grid
}

// RenderPixelBuffer renders a 2D byte grid (pixel buffer) as half-block art.
func RenderPixelBuffer(buf [][]byte) string {
	// Pad to even number of rows
	rows := len(buf)
	if rows%2 != 0 {
		buf = append(buf, make([]byte, 0))
		rows++
	}

	var sb strings.Builder
	for row := 0; row < rows; row += 2 {
		top := buf[row]
		bot := buf[row+1]

		maxLen := len(top)
		if len(bot) > maxLen {
			maxLen = len(bot)
		}

		for col := 0; col < maxLen; col++ {
			var tc, bc byte = '.', '.'
			if col < len(top) {
				tc = top[col]
			}
			if col < len(bot) {
				bc = bot[col]
			}

			topRGB, topOK := Palette[tc]
			botRGB, botOK := Palette[bc]

			if tc == '.' && bc == '.' {
				sb.WriteString(" ")
			} else if tc == '.' {
				sb.WriteString(fgColor(botRGB[0], botRGB[1], botRGB[2]))
				sb.WriteString("▄")
				sb.WriteString(resetCode)
			} else if bc == '.' {
				sb.WriteString(fgColor(topRGB[0], topRGB[1], topRGB[2]))
				sb.WriteString("▀")
				sb.WriteString(resetCode)
			} else if topOK && botOK {
				sb.WriteString(fgColor(topRGB[0], topRGB[1], topRGB[2]))
				sb.WriteString(bgColor(botRGB[0], botRGB[1], botRGB[2]))
				sb.WriteString("▀")
				sb.WriteString(resetCode)
			} else {
				sb.WriteString(" ")
			}
		}
		if row+2 < rows {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// LoadHexCard reads a .hex file and returns a 40×24 RGB pixel grid.
// Format: 40 lines, each with 24 comma-separated 6-char hex RGB values.
func LoadHexCard(path string) ([40][24][3]uint8, error) {
	var pixels [40][24][3]uint8
	data, err := os.ReadFile(path)
	if err != nil {
		return pixels, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 40 {
		return pixels, fmt.Errorf("hex file has %d rows, need 40", len(lines))
	}
	for y := 0; y < 40; y++ {
		cols := strings.Split(strings.TrimSpace(lines[y]), ",")
		if len(cols) < 24 {
			return pixels, fmt.Errorf("row %d has %d pixels, need 24", y, len(cols))
		}
		for x := 0; x < 24; x++ {
			h := strings.TrimSpace(cols[x])
			if len(h) != 6 {
				return pixels, fmt.Errorf("row %d col %d: invalid hex %q", y, x, h)
			}
			b, err := hex.DecodeString(h)
			if err != nil {
				return pixels, fmt.Errorf("row %d col %d: %w", y, x, err)
			}
			pixels[y][x] = [3]uint8{b[0], b[1], b[2]}
		}
	}
	return pixels, nil
}

// LoadHexFrame reads a .hex file with a 44×28 framed card (border included).
func LoadHexFrame(path string) ([44][28][3]uint8, error) {
	var frame [44][28][3]uint8
	data, err := os.ReadFile(path)
	if err != nil {
		return frame, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 44 {
		return frame, fmt.Errorf("hex frame file has %d rows, need 44", len(lines))
	}
	for y := 0; y < 44; y++ {
		cols := strings.Split(strings.TrimSpace(lines[y]), ",")
		if len(cols) < 28 {
			return frame, fmt.Errorf("row %d has %d pixels, need 28", y, len(cols))
		}
		for x := 0; x < 28; x++ {
			h := strings.TrimSpace(cols[x])
			if len(h) != 6 {
				return frame, fmt.Errorf("row %d col %d: invalid hex %q", y, x, h)
			}
			b, err := hex.DecodeString(h)
			if err != nil {
				return frame, fmt.Errorf("row %d col %d: %w", y, x, err)
			}
			frame[y][x] = [3]uint8{b[0], b[1], b[2]}
		}
	}
	return frame, nil
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
