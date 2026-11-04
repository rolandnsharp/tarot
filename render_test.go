package main

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"
)

type cell struct {
	ch     rune
	fg, bg string
}

var sgr = regexp.MustCompile(`^\x1b\[([0-9;]*)m`)

// screen interprets the few escape codes the half-block renderers emit.
func screen(s string) [][]cell {
	var rows [][]cell
	for _, line := range strings.Split(s, "\n") {
		var row []cell
		fg, bg := "", ""
		for len(line) > 0 {
			if m := sgr.FindStringSubmatch(line); m != nil {
				switch p := m[1]; {
				case p == "0":
					fg, bg = "", ""
				case p == "49":
					bg = ""
				case strings.HasPrefix(p, "38;2;"):
					fg = p[5:]
				case strings.HasPrefix(p, "48;2;"):
					bg = p[5:]
				}
				line = line[len(m[0]):]
				continue
			}
			r := []rune(line)[0]
			c := cell{ch: r, fg: fg, bg: bg}
			if r == ' ' {
				c.fg = "" // foreground colour is invisible on a space
			}
			row = append(row, c)
			line = line[len(string(r)):]
		}
		rows = append(rows, row)
	}
	return rows
}

// The run-length encoder must draw exactly what the per-cell encoder draws.
func TestRowEncoderMatchesPerCell(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	palette := []RGB3{{}, {}, {200, 10, 10}, {10, 200, 10}, {1, 2, 3}}
	for trial := 0; trial < 200; trial++ {
		h, w := 2*(1+rng.Intn(4)), 1+rng.Intn(30)
		buf := make([][]RGB3, h)
		for r := range buf {
			buf[r] = make([]RGB3, w)
			for c := range buf[r] {
				buf[r][c] = palette[rng.Intn(len(palette))]
			}
		}
		var old strings.Builder
		for r := 0; r < h; r += 2 {
			for c := 0; c < w; c++ {
				renderHalfBlock(&old, buf[r][c], buf[r+1][c])
			}
			if r+2 < h {
				old.WriteString("\n")
			}
		}
		a, b := screen(old.String()), screen(RenderRGBPixelBuffer(buf))
		if len(a) != len(b) {
			t.Fatalf("rows %d vs %d", len(a), len(b))
		}
		for r := range a {
			if len(a[r]) != len(b[r]) {
				t.Fatalf("row %d: %d vs %d cells", r, len(a[r]), len(b[r]))
			}
			for c := range a[r] {
				if a[r][c] != b[r][c] {
					t.Fatalf("trial %d row %d col %d: %+v vs %+v", trial, r, c, a[r][c], b[r][c])
				}
			}
		}
	}
}
