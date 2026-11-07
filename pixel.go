package main

import (
	"encoding/hex"
	"fmt"
	"strings"
)

type RGB3 = [3]uint8

const resetCode = "\033[0m"

func fgColor(r, g, b uint8) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
}

func bgColor(r, g, b uint8) string {
	return fmt.Sprintf("\033[48;2;%d;%d;%dm", r, g, b)
}

// xterm-256 colour cube levels; colours 16-231 are every combination of these.
var cubeLevels = [6]uint8{0, 95, 135, 175, 215, 255}

func cubeIndex(v uint8) int {
	for i, l := range cubeLevels {
		if l == v {
			return i
		}
	}
	return -1
}

// ansi256 returns the xterm-256 index that is exactly this colour, if there is one.
func ansi256(c RGB3) (int, bool) {
	r, g, b := cubeIndex(c[0]), cubeIndex(c[1]), cubeIndex(c[2])
	if r >= 0 && g >= 0 && b >= 0 {
		return 16 + 36*r + 6*g + b, true
	}
	if c[0] == c[1] && c[1] == c[2] && c[0] >= 8 && c[0] <= 238 && (c[0]-8)%10 == 0 {
		return 232 + int(c[0]-8)/10, true
	}
	return 0, false
}

// colourCode writes the SGR code for a foreground (38) or background (48) colour,
// using the short 256-colour form when the colour is exactly in that palette.
func colourCode(sb *strings.Builder, layer int, c RGB3) {
	if n, ok := ansi256(c); ok {
		fmt.Fprintf(sb, "\x1b[%d;5;%dm", layer, n)
		return
	}
	fmt.Fprintf(sb, "\x1b[%d;2;%d;%d;%dm", layer, c[0], c[1], c[2])
}

// BuildRGBCardFrame wraps an inner pixel grid in a frame with a 2px rounded border.
func BuildRGBCardFrame(inner [][]RGB3, borderColor RGB3) [][]RGB3 {
	innerH := len(inner)
	innerW := 0
	if innerH > 0 {
		innerW = len(inner[0])
	}
	frameH := innerH + 4
	frameW := innerW + 4
	bc := borderColor

	frame := make([][]RGB3, frameH)
	for y := range frame {
		frame[y] = make([]RGB3, frameW)
		for x := range frame[y] {
			frame[y][x] = bc
		}
	}

	// Clear corners for rounded effect
	frame[0][0] = RGB3{0, 0, 0}
	frame[0][1] = RGB3{0, 0, 0}
	frame[0][frameW-2] = RGB3{0, 0, 0}
	frame[0][frameW-1] = RGB3{0, 0, 0}
	frame[frameH-1][0] = RGB3{0, 0, 0}
	frame[frameH-1][1] = RGB3{0, 0, 0}
	frame[frameH-1][frameW-2] = RGB3{0, 0, 0}
	frame[frameH-1][frameW-1] = RGB3{0, 0, 0}

	// Place inner art at offset (2,2)
	for y := 0; y < innerH; y++ {
		for x := 0; x < innerW; x++ {
			frame[y+2][x+2] = inner[y][x]
		}
	}

	return frame
}

// renderHalfBlock writes a single half-block character for a top/bottom pixel pair.
func renderHalfBlock(sb *strings.Builder, top, bot RGB3) {
	topTransparent := top[0] == 0 && top[1] == 0 && top[2] == 0
	botTransparent := bot[0] == 0 && bot[1] == 0 && bot[2] == 0

	if topTransparent && botTransparent {
		sb.WriteString(" ")
	} else if topTransparent {
		colourCode(sb, 38, bot)
		sb.WriteString("▄")
		sb.WriteString(resetCode)
	} else if botTransparent {
		colourCode(sb, 38, top)
		sb.WriteString("▀")
		sb.WriteString(resetCode)
	} else {
		colourCode(sb, 38, top)
		colourCode(sb, 48, bot)
		sb.WriteString("▀")
		sb.WriteString(resetCode)
	}
}

// RenderRGBFrame renders a framed RGB grid as half-block terminal art.
func RenderRGBFrame(frame [][]RGB3) string {
	rows := len(frame)
	cols := 0
	if rows > 0 {
		cols = len(frame[0])
	}
	var sb strings.Builder
	for row := 0; row < rows-1; row += 2 {
		for col := 0; col < cols; col++ {
			renderHalfBlock(&sb, frame[row][col], frame[row+1][col])
		}
		if row+2 < rows {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// buildLabelRow renders a single terminal row with the card name centered in the bottom border.
func buildLabelRow(label string, bc RGB3, frameW int) string {
	innerW := frameW - 4
	if innerW < 0 {
		innerW = 0
	}
	if len(label) > innerW {
		label = label[:innerW]
	}
	pad := innerW - len(label)
	left := pad / 2
	right := pad - left
	centered := strings.Repeat(" ", left) + label + strings.Repeat(" ", right)

	var sb strings.Builder
	corner := fgColor(bc[0], bc[1], bc[2]) + "▀" + resetCode
	// Left 2 corner chars
	sb.WriteString(corner)
	sb.WriteString(corner)
	// Middle: bold text on border background
	lum := 0.299*float64(bc[0]) + 0.587*float64(bc[1]) + 0.114*float64(bc[2])
	var fr, fg, fb uint8
	if lum > 50 {
		fr, fg, fb = 0, 0, 0
	} else {
		fr, fg, fb = 224, 208, 240
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

// RenderRGBFrameWithLabel renders a framed RGB grid with the card name
// embedded in the bottom border.
func RenderRGBFrameWithLabel(frame [][]RGB3, label string) string {
	rows := len(frame)
	cols := 0
	if rows > 0 {
		cols = len(frame[0])
	}
	var sb strings.Builder
	// Render all but last 2 pixel rows as half-block art
	for row := 0; row < rows-2; row += 2 {
		for col := 0; col < cols; col++ {
			renderHalfBlock(&sb, frame[row][col], frame[row+1][col])
		}
		sb.WriteString("\n")
	}
	// Append label row (replaces last 2 pixel rows)
	bc := frame[rows-2][2] // border color from a non-corner pixel
	sb.WriteString(buildLabelRow(label, bc, cols))
	return sb.String()
}

// RenderRGBPixelBuffer renders a screen-sized RGB pixel buffer as half-block art.
// Pixels with RGB (0,0,0) are treated as transparent.
func RenderRGBPixelBuffer(buf [][]RGB3) string {
	if len(buf)%2 != 0 {
		buf = append(buf, make([]RGB3, 0))
	}
	var sb strings.Builder
	for row := 0; row < len(buf); row += 2 {
		renderHalfBlockRow(&sb, buf[row], buf[row+1])
		if row+2 < len(buf) {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// renderHalfBlockRow draws one terminal row of half blocks, sending a colour only
// when it changes and resetting once at the end of the row. The picture is the same
// as renderHalfBlock per cell, at about 1.4x fewer bytes for a shuffle frame, and
// colours in the xterm-256 palette (the card backs) go out in the short form.
func renderHalfBlockRow(sb *strings.Builder, topRow, botRow []RGB3) {
	n := len(topRow)
	if len(botRow) > n {
		n = len(botRow)
	}
	var fg, bg RGB3
	fgSet, bgSet := false, false
	setFg := func(c RGB3) {
		if !fgSet || fg != c {
			colourCode(sb, 38, c)
			fg, fgSet = c, true
		}
	}
	clearBg := func() {
		if bgSet {
			sb.WriteString("\x1b[49m")
			bgSet = false
		}
	}
	for col := 0; col < n; col++ {
		var top, bot RGB3
		if col < len(topRow) {
			top = topRow[col]
		}
		if col < len(botRow) {
			bot = botRow[col]
		}
		topT, botT := top == RGB3{}, bot == RGB3{}
		switch {
		case topT && botT:
			clearBg()
			sb.WriteByte(' ')
		case topT:
			clearBg()
			setFg(bot)
			sb.WriteString("▄")
		case botT:
			clearBg()
			setFg(top)
			sb.WriteString("▀")
		default:
			setFg(top)
			if !bgSet || bg != bot {
				colourCode(sb, 48, bot)
				bg, bgSet = bot, true
			}
			sb.WriteString("▀")
		}
	}
	if fgSet || bgSet {
		sb.WriteString(resetCode)
	}
}

// LoadHexCard reads a .hex file from the embedded FS and returns a variable-size RGB pixel grid.
func LoadHexCard(path string) ([][]RGB3, error) {
	data, err := decksFS.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("hex file is empty")
	}

	pixels := make([][]RGB3, len(lines))
	for y, line := range lines {
		cols := strings.Split(strings.TrimSpace(line), ",")
		pixels[y] = make([]RGB3, len(cols))
		for x, h := range cols {
			h = strings.TrimSpace(h)
			if len(h) != 6 {
				return nil, fmt.Errorf("row %d col %d: invalid hex %q", y, x, h)
			}
			b, err := hex.DecodeString(h)
			if err != nil {
				return nil, fmt.Errorf("row %d col %d: %w", y, x, err)
			}
			pixels[y][x] = RGB3{b[0], b[1], b[2]}
		}
	}
	return pixels, nil
}

// LoadHexFrame reads a .hex file from the embedded FS with a framed card (border included).
func LoadHexFrame(path string) ([][]RGB3, error) {
	return LoadHexCard(path) // same format, just expected to include the frame
}

// FrameWidth returns the terminal column width of a framed card grid.
func FrameWidth(frame [][]RGB3) int {
	if len(frame) == 0 {
		return 0
	}
	return len(frame[0])
}

// FrameHeight returns the terminal row height of a framed card grid.
func FrameHeight(frame [][]RGB3) int {
	return len(frame) / 2
}
