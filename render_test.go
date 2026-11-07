package main

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
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
				case strings.HasPrefix(p, "38;5;"):
					fg = xterm256(p[5:])
				case strings.HasPrefix(p, "48;5;"):
					bg = xterm256(p[5:])
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

// xterm256 turns a 256-colour index back into the "r;g;b" a 24-bit code would carry.
func xterm256(idx string) string {
	n, _ := strconv.Atoi(idx)
	if n >= 232 {
		v := 8 + 10*(n-232)
		return fmt.Sprintf("%d;%d;%d", v, v, v)
	}
	n -= 16
	return fmt.Sprintf("%d;%d;%d", cubeLevels[n/36], cubeLevels[n/6%6], cubeLevels[n%6])
}

func TestAnsi256(t *testing.T) {
	for _, tc := range []struct {
		c  RGB3
		n  int
		ok bool
	}{{RGB3{0, 255, 215}, 50, true}, {RGB3{18, 18, 18}, 233, true}, {RGB3{95, 95, 95}, 59, true}, {RGB3{0, 220, 200}, 0, false}} {
		if n, ok := ansi256(tc.c); n != tc.n || ok != tc.ok {
			t.Errorf("%v: got %d %v", tc.c, n, ok)
		}
	}
}

// The run-length encoder must draw exactly what the per-cell encoder draws.
func TestRowEncoderMatchesPerCell(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	palette := []RGB3{{}, {}, {200, 10, 10}, {10, 200, 10}, {1, 2, 3}, {0, 255, 215}, {18, 18, 18}}
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
