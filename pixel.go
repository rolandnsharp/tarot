package main

import (
	"encoding/hex"
	"fmt"
	"strings"
)

const resetCode = "\033[0m"

func fgColor(r, g, b uint8) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
}

func bgColor(r, g, b uint8) string {
	return fmt.Sprintf("\033[48;2;%d;%d;%dm", r, g, b)
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

// buildLabelRow renders a single terminal row with the card name centered in the bottom border.
// The outer 2 chars on each side render as half-block corners (border over transparent),
// and the middle 24 chars render as black text on the border color background.
func buildLabelRow(label string, bc [3]uint8) string {
	// Truncate and center label in 24 chars
	if len(label) > 24 {
		label = label[:24]
	}
	pad := 24 - len(label)
	left := pad / 2
	right := pad - left
	centered := strings.Repeat(" ", left) + label + strings.Repeat(" ", right)

	var sb strings.Builder
	corner := fgColor(bc[0], bc[1], bc[2]) + "▀" + resetCode
	// Left 2 corner chars
	sb.WriteString(corner)
	sb.WriteString(corner)
	// Middle 24 chars: bold text on border background
	// Pick white or black text based on border luminance
	lum := 0.299*float64(bc[0]) + 0.587*float64(bc[1]) + 0.114*float64(bc[2])
	var fr, fg, fb uint8
	if lum > 50 {
		fr, fg, fb = 0, 0, 0
	} else {
		fr, fg, fb = 224, 208, 240 // lavender — matches reading text
	}
	sb.WriteString("\033[1m")
	sb.WriteString(fgColor(fr, fg, fb))
	sb.WriteString(bgColor(bc[0], bc[1], bc[2]))
	sb.WriteString(centered)
	sb.WriteString(resetCode)
	// Right 2 corner chars
	sb.WriteString(corner)
	sb.WriteString(corner)
	return sb.String()
}

// RenderRGBFrameWithLabel renders a 44×28 framed RGB grid with the card name
// embedded in the bottom border. Rows 0-41 render as half-block art (21 terminal rows),
// and the last terminal row shows the label as styled text replacing the bottom border.
func RenderRGBFrameWithLabel(frame [44][28][3]uint8, label string) string {
	var sb strings.Builder
	// Render first 42 pixel rows as half-block art (21 terminal rows)
	for row := 0; row < 42; row += 2 {
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
		sb.WriteString("\n")
	}
	// Append label row (replaces pixel rows 42-43)
	bc := frame[42][2] // border color from a non-corner pixel
	sb.WriteString(buildLabelRow(label, bc))
	return sb.String()
}

// RenderRGBPixelBuffer renders a screen-sized RGB pixel buffer as half-block art.
// Pixels with RGB (0,0,0) are treated as transparent.
func RenderRGBPixelBuffer(buf [][][3]uint8) string {
	rows := len(buf)
	if rows%2 != 0 {
		buf = append(buf, make([][3]uint8, 0))
		rows++
	}

	var sb strings.Builder
	for row := 0; row < rows; row += 2 {
		topRow := buf[row]
		botRow := buf[row+1]

		maxLen := len(topRow)
		if len(botRow) > maxLen {
			maxLen = len(botRow)
		}

		for col := 0; col < maxLen; col++ {
			var top, bot [3]uint8
			if col < len(topRow) {
				top = topRow[col]
			}
			if col < len(botRow) {
				bot = botRow[col]
			}
			topT := top[0] == 0 && top[1] == 0 && top[2] == 0
			botT := bot[0] == 0 && bot[1] == 0 && bot[2] == 0

			if topT && botT {
				sb.WriteString(" ")
			} else if topT {
				sb.WriteString(fgColor(bot[0], bot[1], bot[2]))
				sb.WriteString("▄")
				sb.WriteString(resetCode)
			} else if botT {
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
		if row+2 < rows {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// LoadHexCard reads a .hex file from the embedded FS and returns a 40×24 RGB pixel grid.
// Format: 40 lines, each with 24 comma-separated 6-char hex RGB values.
func LoadHexCard(path string) ([40][24][3]uint8, error) {
	var pixels [40][24][3]uint8
	data, err := decksFS.ReadFile(path)
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

// LoadHexFrame reads a .hex file from the embedded FS with a 44×28 framed card (border included).
func LoadHexFrame(path string) ([44][28][3]uint8, error) {
	var frame [44][28][3]uint8
	data, err := decksFS.ReadFile(path)
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
