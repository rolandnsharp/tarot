package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// Canvas is 40 rows × 24 cols of RGB pixels — matches the card inner art size.
type Canvas [40][24][3]uint8

func (c *Canvas) set(x, y int, r, g, b uint8) {
	if x >= 0 && x < 24 && y >= 0 && y < 40 {
		c[y][x] = [3]uint8{r, g, b}
	}
}

func (c *Canvas) get(x, y int) (uint8, uint8, uint8) {
	if x >= 0 && x < 24 && y >= 0 && y < 40 {
		return c[y][x][0], c[y][x][1], c[y][x][2]
	}
	return 0, 0, 0
}

// blend mixes color onto pixel with alpha 0.0–1.0 (additive-ish for glow).
func (c *Canvas) blend(x, y int, r, g, b uint8, a float64) {
	if x < 0 || x >= 24 || y < 0 || y >= 40 || a <= 0 {
		return
	}
	or, og, ob := c.get(x, y)
	nr := clamp(float64(or)*(1-a) + float64(r)*a)
	ng := clamp(float64(og)*(1-a) + float64(g)*a)
	nb := clamp(float64(ob)*(1-a) + float64(b)*a)
	c.set(x, y, nr, ng, nb)
}

// screen blend mode — brightens, good for neon glow stacking.
func (c *Canvas) screen(x, y int, r, g, b uint8, a float64) {
	if x < 0 || x >= 24 || y < 0 || y >= 40 || a <= 0 {
		return
	}
	or, og, ob := c.get(x, y)
	// screen: 1-(1-a)*(1-b)
	sr := 255 - uint8(float64(255-or)*(1-float64(r)*a/255))
	sg := 255 - uint8(float64(255-og)*(1-float64(g)*a/255))
	sb := 255 - uint8(float64(255-ob)*(1-float64(b)*a/255))
	c.set(x, y, sr, sg, sb)
}

func clamp(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func (c *Canvas) fillGrad(top, bot [3]uint8) {
	for y := 0; y < 40; y++ {
		t := float64(y) / 39
		for x := 0; x < 24; x++ {
			c.set(x, y,
				clamp(float64(top[0])*(1-t)+float64(bot[0])*t),
				clamp(float64(top[1])*(1-t)+float64(bot[1])*t),
				clamp(float64(top[2])*(1-t)+float64(bot[2])*t))
		}
	}
}

func (c *Canvas) rect(x, y, w, h int, r, g, b uint8) {
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			c.set(x+dx, y+dy, r, g, b)
		}
	}
}

func (c *Canvas) hline(x1, x2, y int, r, g, b uint8) {
	for x := x1; x <= x2; x++ {
		c.set(x, y, r, g, b)
	}
}

func (c *Canvas) vline(x, y1, y2 int, r, g, b uint8) {
	for y := y1; y <= y2; y++ {
		c.set(x, y, r, g, b)
	}
}

// glow draws a Gaussian glow centered at (cx,cy) with given radius.
func (c *Canvas) glow(cx, cy, rad int, r, g, b uint8) {
	rr := float64(rad)
	for y := cy - rad*2; y <= cy+rad*2; y++ {
		for x := cx - rad*2; x <= cx+rad*2; x++ {
			dx := float64(x) - float64(cx)
			dy := float64(y) - float64(cy)
			d2 := dx*dx + dy*dy
			a := math.Exp(-d2 / (2 * rr * rr))
			if a > 0.02 {
				c.screen(x, y, r, g, b, a)
			}
		}
	}
}

// softCircle draws a filled circle with soft edges.
func (c *Canvas) softCircle(cx, cy int, rad float64, r, g, b uint8) {
	ir := int(rad) + 2
	for y := cy - ir; y <= cy+ir; y++ {
		for x := cx - ir; x <= cx+ir; x++ {
			dx := float64(x) - float64(cx)
			dy := float64(y) - float64(cy)
			d := math.Sqrt(dx*dx+dy*dy) - rad
			if d < -1 {
				c.set(x, y, r, g, b)
			} else if d < 0.5 {
				a := 1 - (d+1)/1.5
				c.blend(x, y, r, g, b, a)
			}
		}
	}
}

// stamp draws a bitmap string onto the canvas. 'X' pixels get the color.
func (c *Canvas) stamp(sx, sy, w int, bmp string, r, g, b uint8) {
	for i, ch := range bmp {
		if ch == 'X' {
			x := sx + i%w
			y := sy + i/w
			c.set(x, y, r, g, b)
		}
	}
}

// stampGlow draws a bitmap with glow around each set pixel.
func (c *Canvas) stampGlow(sx, sy, w int, bmp string, r, g, b uint8, grad int) {
	for i, ch := range bmp {
		if ch == 'X' {
			x := sx + i%w
			y := sy + i/w
			c.glow(x, y, grad, r, g, b)
		}
	}
	// Overdraw solid pixels on top
	c.stamp(sx, sy, w, bmp, r, g, b)
}

// scanlines adds subtle horizontal scanline effect.
func (c *Canvas) scanlines() {
	for y := 0; y < 40; y += 2 {
		for x := 0; x < 24; x++ {
			r, g, b := c.get(x, y)
			c.set(x, y, uint8(float64(r)*0.85), uint8(float64(g)*0.85), uint8(float64(b)*0.85))
		}
	}
}

// circuits draws subtle circuit-trace patterns.
func (c *Canvas) circuits(color [3]uint8, alpha float64) {
	cr, cg, cb := color[0], color[1], color[2]
	// Horizontal traces
	for _, y := range []int{4, 12, 20, 28, 36} {
		for x := 2; x < 22; x += 4 {
			c.blend(x, y, cr, cg, cb, alpha*0.3)
			c.blend(x+1, y, cr, cg, cb, alpha*0.5)
			c.blend(x+2, y, cr, cg, cb, alpha*0.3)
		}
	}
	// Vertical traces
	for _, x := range []int{4, 12, 19} {
		for y := 2; y < 38; y += 4 {
			c.blend(x, y, cr, cg, cb, alpha*0.3)
			c.blend(x, y+1, cr, cg, cb, alpha*0.5)
			c.blend(x, y+2, cr, cg, cb, alpha*0.3)
		}
	}
	// Nodes at intersections
	for _, y := range []int{4, 12, 20, 28, 36} {
		for _, x := range []int{4, 12, 19} {
			c.blend(x, y, cr, cg, cb, alpha*0.8)
		}
	}
}

func (c Canvas) writeHex(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var sb strings.Builder
	for y := 0; y < 40; y++ {
		for x := 0; x < 24; x++ {
			if x > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(fmt.Sprintf("%02x%02x%02x", c[y][x][0], c[y][x][1], c[y][x][2]))
		}
		sb.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// --- Suit symbol bitmaps (5×5) ---

var heartBmp = "" +
	".X.X." +
	"XXXXX" +
	"XXXXX" +
	".XXX." +
	"..X.."

var spadeBmp = "" +
	"..X.." +
	".XXX." +
	"XXXXX" +
	".XXX." +
	"..X.."

var diamondBmp = "" +
	"..X.." +
	".XXX." +
	"XXXXX" +
	".XXX." +
	"..X.."

var clubBmp = "" +
	"..X.." +
	".XXX." +
	"X.X.X" +
	".XXX." +
	"..X.."

// --- Small 3×5 digit bitmaps ---

var digits = map[int]string{
	0: "XXX" + "X.X" + "X.X" + "X.X" + "XXX",
	1: ".X." + "XX." + ".X." + ".X." + "XXX",
	2: "XXX" + "..X" + "XXX" + "X.." + "XXX",
	3: "XXX" + "..X" + "XXX" + "..X" + "XXX",
	4: "X.X" + "X.X" + "XXX" + "..X" + "..X",
	5: "XXX" + "X.." + "XXX" + "..X" + "XXX",
	6: "XXX" + "X.." + "XXX" + "X.X" + "XXX",
	7: "XXX" + "..X" + "..X" + "..X" + "..X",
	8: "XXX" + "X.X" + "XXX" + "X.X" + "XXX",
	9: "XXX" + "X.X" + "XXX" + "..X" + "XXX",
}

func (c *Canvas) drawNum(x, y, n int, r, g, b uint8) {
	if n >= 10 {
		c.stamp(x, y, 3, digits[n/10], r, g, b)
		c.stamp(x+4, y, 3, digits[n%10], r, g, b)
	} else {
		c.stamp(x, y, 3, digits[n], r, g, b)
	}
}

// --- Suit colors (cyberpunk neon) ---

type suitInfo struct {
	name    string
	bmp     string
	primary [3]uint8
	dim     [3]uint8
}

var suits = []suitInfo{
	{"hearts", heartBmp, [3]uint8{255, 20, 147}, [3]uint8{120, 10, 70}},    // Cups
	{"clubs", clubBmp, [3]uint8{0, 255, 65}, [3]uint8{0, 100, 30}},         // Wands
	{"spades", spadeBmp, [3]uint8{0, 212, 255}, [3]uint8{0, 80, 100}},       // Swords
	{"diamonds", diamondBmp, [3]uint8{255, 215, 0}, [3]uint8{100, 85, 0}},   // Pentacles
}

// suitFileNames maps our index to the actual file suit name used by art.go
var suitFileNames = []string{"cups", "wands", "swords", "pentacles"}

func (c *Canvas) drawSuit(cx, cy int, si int) {
	s := suits[si]
	c.stampGlow(cx-2, cy-2, 5, s.bmp, s.primary[0], s.primary[1], s.primary[2], 2)
}

// --- Background presets ---

var bgDark = [3]uint8{8, 4, 16}
var bgMid = [3]uint8{14, 8, 28}

func cyberBG() Canvas {
	var cv Canvas
	cv.fillGrad(bgMid, bgDark)
	return cv
}

// --- Pip card standard layouts (positions for N pips on 24×40 grid) ---

var pipPositions = map[int][][2]int{
	1:  {{12, 20}},
	2:  {{12, 10}, {12, 30}},
	3:  {{12, 8}, {12, 20}, {12, 32}},
	4:  {{7, 8}, {17, 8}, {7, 32}, {17, 32}},
	5:  {{7, 8}, {17, 8}, {12, 20}, {7, 32}, {17, 32}},
	6:  {{7, 8}, {17, 8}, {7, 20}, {17, 20}, {7, 32}, {17, 32}},
	7:  {{7, 8}, {17, 8}, {12, 14}, {7, 20}, {17, 20}, {7, 32}, {17, 32}},
	8:  {{7, 8}, {17, 8}, {12, 14}, {7, 20}, {17, 20}, {12, 26}, {7, 32}, {17, 32}},
	9:  {{7, 7}, {17, 7}, {7, 15}, {17, 15}, {12, 20}, {7, 25}, {17, 25}, {7, 33}, {17, 33}},
	10: {{7, 6}, {17, 6}, {12, 11}, {7, 16}, {17, 16}, {7, 24}, {17, 24}, {12, 29}, {7, 34}, {17, 34}},
}

func genPip(si, num int) Canvas {
	cv := cyberBG()
	s := suits[si]
	cv.circuits(s.dim, 0.15)

	// Corner numbers
	cv.drawNum(1, 1, num, s.primary[0], s.primary[1], s.primary[2])
	cv.drawNum(18, 34, num, s.primary[0], s.primary[1], s.primary[2])

	// Pips
	if pos, ok := pipPositions[num]; ok {
		for _, p := range pos {
			cv.drawSuit(p[0], p[1], si)
		}
	}

	cv.scanlines()
	return cv
}

// --- Court card generator ---

// Silhouette shapes for Page, Knight, Queen, King
func genCourt(si, rank int) Canvas {
	cv := cyberBG()
	s := suits[si]
	cv.circuits(s.dim, 0.12)

	p := s.primary
	d := s.dim

	// Draw figure silhouette based on rank
	switch rank {
	case 11: // Page — young figure, shorter
		// Head
		cv.softCircle(12, 10, 3, d[0], d[1], d[2])
		cv.glow(12, 10, 2, p[0], p[1], p[2])
		// Body
		cv.rect(10, 14, 5, 10, d[0], d[1], d[2])
		// Legs
		cv.rect(10, 24, 2, 6, d[0], d[1], d[2])
		cv.rect(13, 24, 2, 6, d[0], d[1], d[2])
		// Suit symbol accent
		cv.drawSuit(12, 18, si)

	case 12: // Knight — figure with extended arm (weapon)
		// Head
		cv.softCircle(10, 9, 3, d[0], d[1], d[2])
		cv.glow(10, 9, 2, p[0], p[1], p[2])
		// Body
		cv.rect(8, 13, 5, 10, d[0], d[1], d[2])
		// Extended arm with weapon
		cv.hline(13, 20, 16, p[0], p[1], p[2])
		cv.hline(13, 20, 17, p[0], p[1], p[2])
		cv.vline(20, 12, 17, p[0], p[1], p[2])
		// Legs
		cv.rect(8, 23, 2, 7, d[0], d[1], d[2])
		cv.rect(11, 23, 2, 7, d[0], d[1], d[2])
		// Suit symbol
		cv.drawSuit(18, 26, si)

	case 13: // Queen — figure with crown, flowing robes
		// Crown
		cv.stamp(9, 4, 7, "..X.X.."+".XXXXX."+"XXXXXXX", p[0], p[1], p[2])
		// Head
		cv.softCircle(12, 10, 3, d[0], d[1], d[2])
		cv.glow(12, 10, 2, p[0], p[1], p[2])
		// Body — wider robes
		for y := 14; y < 32; y++ {
			w := 3 + (y-14)/3
			cv.hline(12-w, 12+w, y, d[0], d[1], d[2])
		}
		// Neon trim on robes
		for y := 14; y < 32; y += 3 {
			w := 3 + (y-14)/3
			cv.set(12-w, y, p[0], p[1], p[2])
			cv.set(12+w, y, p[0], p[1], p[2])
		}
		// Suit symbol
		cv.drawSuit(12, 22, si)

	case 14: // King — broad figure with crown, scepter
		// Crown
		cv.stamp(9, 3, 7, ".X.X.X."+".XXXXX."+"XXXXXXX", p[0], p[1], p[2])
		// Head
		cv.softCircle(12, 9, 3, d[0], d[1], d[2])
		cv.glow(12, 9, 2, p[0], p[1], p[2])
		// Broad body
		cv.rect(7, 13, 11, 14, d[0], d[1], d[2])
		// Scepter
		cv.vline(19, 10, 28, p[0], p[1], p[2])
		cv.glow(19, 10, 2, p[0], p[1], p[2])
		// Legs
		cv.rect(8, 27, 3, 5, d[0], d[1], d[2])
		cv.rect(14, 27, 3, 5, d[0], d[1], d[2])
		// Suit symbol
		cv.drawSuit(12, 20, si)
	}

	// Corner rank letter
	letters := map[int]string{
		11: ".X." + "X.X" + "XXX" + "X.." + "X..", // P
		12: "X.X" + "X.X" + "XX." + "X.X" + "X.X", // K(night) → N
		13: "XX." + "X.X" + "X.X" + "X.X" + "XX.", // Q
		14: "X.X" + "X.X" + "XX." + "X.X" + "X.X", // K
	}
	if l, ok := letters[rank]; ok {
		cv.stamp(1, 1, 3, l, p[0], p[1], p[2])
	}

	cv.scanlines()
	return cv
}

// --- Major Arcana generator ---

var majorSlugs = []string{
	"the-fool", "the-magician", "the-high-priestess", "the-empress",
	"the-emperor", "the-hierophant", "the-lovers", "the-chariot",
	"strength", "the-hermit", "wheel-of-fortune", "justice",
	"the-hanged-man", "death", "temperance", "the-devil",
	"the-tower", "the-star", "the-moon", "the-sun",
	"judgement", "the-world",
}

type rgb = [3]uint8

func genMajor(num int) Canvas {
	cv := cyberBG()

	// Per-card color scheme
	type scheme struct{ p, s rgb }
	schemes := []scheme{
		{rgb{0, 212, 255}, rgb{255, 215, 0}},   // 0  Fool: cyan + gold
		{rgb{180, 0, 255}, rgb{255, 215, 0}},    // 1  Magician: purple + gold
		{rgb{100, 140, 255}, rgb{200, 200, 220}}, // 2  High Priestess: blue + silver
		{rgb{0, 220, 100}, rgb{255, 215, 0}},    // 3  Empress: green + gold
		{rgb{255, 50, 50}, rgb{255, 215, 0}},    // 4  Emperor: red + gold
		{rgb{255, 215, 0}, rgb{255, 255, 255}},  // 5  Hierophant: gold + white
		{rgb{255, 20, 147}, rgb{255, 100, 100}}, // 6  Lovers: pink + red
		{rgb{100, 140, 255}, rgb{200, 200, 220}}, // 7  Chariot: blue + silver
		{rgb{255, 160, 0}, rgb{255, 215, 0}},    // 8  Strength: orange + gold
		{rgb{255, 255, 150}, rgb{100, 100, 80}}, // 9  Hermit: warm yellow + dim
		{rgb{255, 215, 0}, rgb{180, 0, 255}},    // 10 Wheel: gold + purple
		{rgb{255, 215, 0}, rgb{100, 140, 255}},  // 11 Justice: gold + blue
		{rgb{0, 200, 200}, rgb{180, 0, 255}},    // 12 Hanged Man: teal + purple
		{rgb{255, 255, 255}, rgb{255, 50, 50}},  // 13 Death: white + red
		{rgb{100, 140, 255}, rgb{255, 215, 0}},  // 14 Temperance: blue + gold
		{rgb{255, 50, 50}, rgb{255, 140, 0}},    // 15 Devil: red + orange
		{rgb{255, 160, 0}, rgb{255, 50, 50}},    // 16 Tower: orange + red
		{rgb{255, 255, 100}, rgb{100, 140, 255}}, // 17 Star: yellow + blue
		{rgb{200, 200, 240}, rgb{180, 0, 255}},  // 18 Moon: silver + purple
		{rgb{255, 215, 0}, rgb{255, 160, 0}},    // 19 Sun: gold + orange
		{rgb{255, 255, 255}, rgb{255, 215, 0}},  // 20 Judgement: white + gold
		{rgb{0, 220, 100}, rgb{255, 215, 0}},    // 21 World: green + gold
	}

	sc := schemes[num]
	p, s := sc.p, sc.s

	// Dim versions for background elements
	pd := rgb{p[0] / 4, p[1] / 4, p[2] / 4}

	cv.circuits(pd, 0.2)

	// Roman numeral in top-left corner
	cv.drawNum(1, 1, num, p[0], p[1], p[2])

	switch num {
	case 0: // Fool — figure at cliff edge, swirling void below
		// Figure
		cv.softCircle(12, 10, 3, p[0]/2, p[1]/2, p[2]/2)
		cv.glow(12, 10, 2, p[0], p[1], p[2])
		cv.rect(10, 14, 5, 8, pd[0], pd[1], pd[2])
		cv.rect(10, 22, 2, 5, pd[0], pd[1], pd[2])
		cv.rect(13, 22, 2, 5, pd[0], pd[1], pd[2])
		// Cliff edge
		cv.hline(0, 15, 28, p[0]/3, p[1]/3, p[2]/3)
		cv.hline(0, 14, 29, p[0]/4, p[1]/4, p[2]/4)
		// Void below with swirling glow
		cv.glow(8, 34, 5, s[0], s[1], s[2])
		cv.glow(16, 36, 4, p[0], p[1], p[2])
		// Star above
		cv.glow(18, 4, 2, s[0], s[1], s[2])

	case 1: // Magician — infinity symbol, altar with 4 suit symbols
		// Infinity symbol above head
		cv.stamp(7, 4, 10,
			"..XX..XX.."+
				".X..XX..X."+
				"..XX..XX..", p[0], p[1], p[2])
		cv.glow(12, 5, 3, p[0], p[1], p[2])
		// Figure
		cv.softCircle(12, 11, 3, 80, 60, 90)
		cv.glow(12, 11, 2, p[0], p[1], p[2])
		cv.rect(10, 15, 5, 8, pd[0], pd[1], pd[2])
		// Arms extended
		cv.hline(4, 8, 17, p[0], p[1], p[2])
		cv.hline(16, 20, 17, p[0], p[1], p[2])
		// Altar/table
		cv.rect(3, 28, 18, 2, s[0]/2, s[1]/2, s[2]/2)
		cv.hline(3, 20, 27, s[0], s[1], s[2])
		// Four suit symbols on altar
		for i := 0; i < 4; i++ {
			cx := 5 + i*5
			cv.drawSuit(cx, 32, i)
		}

	case 2: // High Priestess — two pillars, crescent moon, veil
		// Moon
		cv.softCircle(12, 7, 3, p[0], p[1], p[2])
		cv.softCircle(14, 7, 3, bgMid[0], bgMid[1], bgMid[2]) // crescent cutout
		cv.glow(11, 7, 4, p[0], p[1], p[2])
		// Pillars
		cv.rect(3, 12, 3, 22, 40, 40, 60)
		cv.rect(18, 12, 3, 22, 40, 40, 60)
		cv.glow(4, 12, 1, p[0], p[1], p[2])
		cv.glow(19, 12, 1, s[0], s[1], s[2])
		// Veil between pillars
		for y := 14; y < 32; y++ {
			a := 0.15 + 0.1*math.Sin(float64(y)*0.8)
			for x := 7; x < 17; x++ {
				cv.blend(x, y, p[0], p[1], p[2], a)
			}
		}
		// Figure silhouette
		cv.softCircle(12, 16, 2, 60, 50, 80)
		cv.rect(10, 19, 5, 10, 40, 30, 60)
		// Scroll
		cv.rect(10, 32, 4, 3, s[0]/2, s[1]/2, s[2]/2)

	case 3: // Empress — crown, nature, abundance
		// Crown with gems
		cv.stamp(9, 3, 7, "X.X.X.X"+".XXXXX."+"XXXXXXX", s[0], s[1], s[2])
		cv.glow(12, 4, 2, s[0], s[1], s[2])
		// Head
		cv.softCircle(12, 9, 3, 80, 60, 70)
		cv.glow(12, 9, 2, p[0], p[1], p[2])
		// Flowing robes
		for y := 13; y < 30; y++ {
			w := 3 + (y-13)/2
			for x := 12 - w; x <= 12+w; x++ {
				cv.blend(x, y, p[0]/3, p[1]/3, p[2]/3, 0.7)
			}
		}
		// Nature — garden below
		for x := 0; x < 24; x++ {
			h := 3 + int(2*math.Sin(float64(x)*0.7))
			for y := 37 - h; y < 40; y++ {
				cv.blend(x, y, p[0]/2, p[1]/2, p[2]/2, 0.6)
			}
		}
		// Star accents
		cv.glow(5, 6, 1, s[0], s[1], s[2])
		cv.glow(19, 6, 1, s[0], s[1], s[2])

	case 4: // Emperor — throne, angular, authoritative
		// Crown
		cv.stamp(9, 3, 7, ".XXXXX."+"X.X.X.X"+"XXXXXXX", s[0], s[1], s[2])
		cv.glow(12, 4, 2, s[0], s[1], s[2])
		// Head
		cv.softCircle(12, 9, 3, 80, 60, 70)
		cv.glow(12, 9, 2, p[0], p[1], p[2])
		// Throne — angular shapes
		cv.rect(3, 6, 3, 28, 60, 30, 30)
		cv.rect(18, 6, 3, 28, 60, 30, 30)
		cv.rect(3, 6, 18, 2, p[0]/3, p[1]/3, p[2]/3)
		// Body
		cv.rect(8, 13, 8, 14, 50, 25, 25)
		// Orb in hand
		cv.softCircle(17, 18, 2, s[0], s[1], s[2])
		cv.glow(17, 18, 2, s[0], s[1], s[2])
		// Legs
		cv.rect(9, 27, 3, 5, 50, 25, 25)
		cv.rect(13, 27, 3, 5, 50, 25, 25)

	case 5: // Hierophant — triple cross, columns, spiritual
		// Triple cross
		cv.vline(12, 3, 15, s[0], s[1], s[2])
		cv.hline(10, 14, 5, s[0], s[1], s[2])
		cv.hline(9, 15, 9, s[0], s[1], s[2])
		cv.hline(10, 14, 13, s[0], s[1], s[2])
		cv.glow(12, 9, 3, s[0], s[1], s[2])
		// Columns
		cv.rect(2, 16, 2, 20, 50, 40, 30)
		cv.rect(20, 16, 2, 20, 50, 40, 30)
		// Figure
		cv.softCircle(12, 20, 3, 80, 70, 50)
		cv.rect(9, 24, 7, 8, pd[0], pd[1], pd[2])
		// Two supplicants
		cv.softCircle(6, 33, 2, 60, 50, 40)
		cv.softCircle(18, 33, 2, 60, 50, 40)
		cv.rect(5, 35, 3, 4, 40, 30, 25)
		cv.rect(17, 35, 3, 4, 40, 30, 25)

	case 6: // Lovers — two figures, heart glow above
		// Heart glow above
		cv.stampGlow(10, 3, 5, heartBmp, p[0], p[1], p[2], 3)
		// Two figures
		// Left figure
		cv.softCircle(7, 14, 2, 80, 60, 70)
		cv.glow(7, 14, 2, s[0], s[1], s[2])
		cv.rect(5, 17, 5, 10, 60, 30, 40)
		cv.rect(5, 27, 2, 5, 60, 30, 40)
		cv.rect(8, 27, 2, 5, 60, 30, 40)
		// Right figure
		cv.softCircle(17, 14, 2, 80, 60, 70)
		cv.glow(17, 14, 2, s[0], s[1], s[2])
		cv.rect(15, 17, 5, 10, 60, 30, 40)
		cv.rect(15, 27, 2, 5, 60, 30, 40)
		cv.rect(18, 27, 2, 5, 60, 30, 40)
		// Connection beam between them
		for x := 9; x <= 15; x++ {
			cv.glow(x, 20, 1, p[0], p[1], p[2])
		}

	case 7: // Chariot — vehicle, two wheels, determination
		// Figure on top
		cv.softCircle(12, 6, 3, 80, 60, 70)
		cv.glow(12, 6, 2, p[0], p[1], p[2])
		cv.rect(10, 10, 5, 5, pd[0], pd[1], pd[2])
		// Chariot body
		cv.rect(4, 16, 16, 10, 40, 40, 60)
		cv.hline(4, 19, 16, s[0], s[1], s[2])
		cv.hline(4, 19, 25, s[0], s[1], s[2])
		cv.vline(4, 16, 25, s[0], s[1], s[2])
		cv.vline(19, 16, 25, s[0], s[1], s[2])
		// Star emblem on chariot
		cv.glow(12, 21, 2, s[0], s[1], s[2])
		// Wheels
		cv.softCircle(5, 30, 3, p[0]/2, p[1]/2, p[2]/2)
		cv.glow(5, 30, 2, p[0], p[1], p[2])
		cv.softCircle(19, 30, 3, p[0]/2, p[1]/2, p[2]/2)
		cv.glow(19, 30, 2, p[0], p[1], p[2])
		// Ground line
		cv.hline(0, 23, 35, p[0]/3, p[1]/3, p[2]/3)

	case 8: // Strength — figure taming a beast
		// Figure
		cv.softCircle(8, 8, 3, 80, 60, 70)
		cv.glow(8, 8, 2, p[0], p[1], p[2])
		cv.rect(6, 12, 5, 8, pd[0], pd[1], pd[2])
		// Infinity symbol above (inner strength)
		cv.stamp(5, 3, 10,
			"..XX..XX.."+
				".X..XX..X."+
				"..XX..XX..", s[0], s[1], s[2])
		// Beast (lion shape)
		cv.softCircle(16, 20, 4, p[0]/2, p[1]/2, p[2]/2)
		cv.glow(16, 20, 3, p[0], p[1], p[2])
		cv.rect(12, 22, 10, 6, p[0]/3, p[1]/3, p[2]/3)
		// Legs
		cv.rect(12, 28, 2, 4, p[0]/3, p[1]/3, p[2]/3)
		cv.rect(15, 28, 2, 4, p[0]/3, p[1]/3, p[2]/3)
		cv.rect(18, 28, 2, 4, p[0]/3, p[1]/3, p[2]/3)
		// Tail
		cv.set(22, 22, p[0], p[1], p[2])
		cv.set(23, 21, p[0], p[1], p[2])
		// Hand reaching to beast
		cv.hline(10, 13, 16, s[0], s[1], s[2])

	case 9: // Hermit — solitary figure, lantern, mountain
		// Mountain
		for y := 20; y < 40; y++ {
			w := (y - 20) * 12 / 20
			for x := 12 - w; x <= 12+w; x++ {
				cv.blend(x, y, 30, 20, 40, 0.5)
			}
		}
		// Figure on mountain
		cv.softCircle(12, 14, 2, 60, 50, 40)
		cv.rect(10, 17, 5, 8, 40, 30, 30)
		cv.rect(10, 25, 2, 4, 40, 30, 30)
		cv.rect(13, 25, 2, 4, 40, 30, 30)
		// Lantern
		cv.glow(16, 12, 4, p[0], p[1], p[2])
		cv.set(16, 12, p[0], p[1], p[2])
		// Staff
		cv.vline(7, 14, 28, s[0]/2, s[1]/2, s[2]/2)
		// Stars
		cv.glow(4, 4, 1, p[0], p[1], p[2])
		cv.glow(20, 6, 1, p[0], p[1], p[2])

	case 10: // Wheel of Fortune — spinning wheel with symbols
		// Large circle
		for a := 0.0; a < 2*math.Pi; a += 0.15 {
			x := 12 + int(9*math.Cos(a))
			y := 20 + int(9*math.Sin(a))
			cv.set(x, y, p[0], p[1], p[2])
		}
		// Inner circle
		for a := 0.0; a < 2*math.Pi; a += 0.2 {
			x := 12 + int(5*math.Cos(a))
			y := 20 + int(5*math.Sin(a))
			cv.set(x, y, s[0], s[1], s[2])
		}
		// Spokes
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			for r := 2.0; r < 9; r += 0.5 {
				x := 12 + int(r*math.Cos(a))
				y := 20 + int(r*math.Sin(a))
				cv.blend(x, y, p[0], p[1], p[2], 0.5)
			}
		}
		// Center glow
		cv.glow(12, 20, 3, s[0], s[1], s[2])
		// Four suit symbols at cardinal points
		cv.drawSuit(12, 10, 0)
		cv.drawSuit(21, 20, 1)
		cv.drawSuit(12, 30, 2)
		cv.drawSuit(3, 20, 3)

	case 11: // Justice — scales, sword
		// Sword (vertical)
		cv.vline(12, 4, 24, s[0], s[1], s[2])
		cv.hline(10, 14, 8, s[0], s[1], s[2])
		cv.glow(12, 6, 2, s[0], s[1], s[2])
		// Scales bar
		cv.hline(4, 20, 14, p[0], p[1], p[2])
		// Left scale
		for a := 0.0; a < math.Pi; a += 0.2 {
			x := 6 + int(3*math.Cos(a))
			y := 18 + int(2*math.Sin(a))
			cv.set(x, y, p[0], p[1], p[2])
		}
		// Right scale
		for a := 0.0; a < math.Pi; a += 0.2 {
			x := 18 + int(3*math.Cos(a))
			y := 18 + int(2*math.Sin(a))
			cv.set(x, y, p[0], p[1], p[2])
		}
		// Chains
		cv.vline(6, 14, 18, p[0]/2, p[1]/2, p[2]/2)
		cv.vline(18, 14, 18, p[0]/2, p[1]/2, p[2]/2)
		// Pedestal
		cv.rect(6, 30, 12, 3, 50, 40, 30)
		cv.hline(4, 19, 29, p[0]/2, p[1]/2, p[2]/2)
		cv.glow(12, 30, 3, p[0], p[1], p[2])

	case 12: // Hanged Man — inverted figure, triangle frame
		// Triangle frame
		for y := 2; y < 20; y++ {
			w := y
			if w > 23 {
				w = 23
			}
			cv.set(12-w/2, y, s[0], s[1], s[2])
			cv.set(12+w/2, y, s[0], s[1], s[2])
		}
		cv.hline(2, 21, 20, s[0], s[1], s[2])
		// Rope
		cv.vline(12, 2, 12, p[0]/2, p[1]/2, p[2]/2)
		// Inverted figure (feet up, head down)
		// Legs (up, forming inverted V)
		cv.vline(10, 10, 16, pd[0], pd[1], pd[2])
		cv.vline(14, 10, 16, pd[0], pd[1], pd[2])
		cv.set(11, 12, pd[0], pd[1], pd[2])
		cv.set(13, 12, pd[0], pd[1], pd[2])
		// Body
		cv.rect(10, 16, 5, 8, pd[0], pd[1], pd[2])
		// Head (at bottom with halo)
		cv.softCircle(12, 27, 3, 80, 60, 70)
		cv.glow(12, 27, 4, p[0], p[1], p[2])
		// Halo ring
		for a := 0.0; a < 2*math.Pi; a += 0.3 {
			x := 12 + int(4*math.Cos(a))
			y := 27 + int(4*math.Sin(a))
			cv.blend(x, y, s[0], s[1], s[2], 0.5)
		}

	case 13: // Death — skull, scythe
		// Skull
		cv.softCircle(12, 10, 5, p[0], p[1], p[2])
		// Eye sockets
		cv.set(10, 9, bgDark[0], bgDark[1], bgDark[2])
		cv.set(14, 9, bgDark[0], bgDark[1], bgDark[2])
		// Nose
		cv.set(12, 11, bgDark[0], bgDark[1], bgDark[2])
		// Mouth
		cv.hline(10, 14, 13, bgDark[0], bgDark[1], bgDark[2])
		cv.glow(12, 10, 4, p[0], p[1], p[2])
		// Scythe arc
		for a := 0.5; a < 2.5; a += 0.1 {
			x := 12 + int(10*math.Cos(a))
			y := 25 + int(8*math.Sin(a))
			cv.set(x, y, s[0], s[1], s[2])
		}
		cv.glow(6, 20, 2, s[0], s[1], s[2])
		// Scythe handle
		cv.vline(20, 18, 36, s[0]/2, s[1]/2, s[2]/2)
		// Rose at bottom (rebirth)
		cv.glow(8, 36, 2, 200, 50, 50)

	case 14: // Temperance — angel, two vessels, flowing water
		// Wings
		for i := 0; i < 6; i++ {
			cv.set(4+i, 10-i/2, p[0]/2, p[1]/2, p[2]/2)
			cv.set(20-i, 10-i/2, p[0]/2, p[1]/2, p[2]/2)
		}
		// Head
		cv.softCircle(12, 10, 3, 80, 70, 60)
		cv.glow(12, 10, 2, p[0], p[1], p[2])
		// Halo
		for a := 0.0; a < math.Pi; a += 0.3 {
			x := 12 + int(4*math.Cos(a))
			y := 7 - int(2*math.Sin(a))
			cv.blend(x, y, s[0], s[1], s[2], 0.6)
		}
		// Body
		cv.rect(10, 14, 5, 10, pd[0], pd[1], pd[2])
		// Two vessels
		cv.rect(4, 18, 4, 5, s[0]/2, s[1]/2, s[2]/2)
		cv.rect(16, 22, 4, 5, s[0]/2, s[1]/2, s[2]/2)
		// Water flowing between
		for i := 0; i < 10; i++ {
			x := 8 + i
			y := 20 + i/3
			cv.blend(x, y, p[0], p[1], p[2], 0.7)
		}
		cv.glow(12, 22, 3, p[0], p[1], p[2])
		// Ground/water
		for x := 0; x < 24; x++ {
			cv.blend(x, 35, p[0]/3, p[1]/3, p[2]/3, 0.5)
			cv.blend(x, 36, p[0]/3, p[1]/3, p[2]/3, 0.4)
		}

	case 15: // Devil — horned figure, chains, captives
		// Horns
		cv.set(8, 4, p[0], p[1], p[2])
		cv.set(7, 3, p[0], p[1], p[2])
		cv.set(16, 4, p[0], p[1], p[2])
		cv.set(17, 3, p[0], p[1], p[2])
		// Head
		cv.softCircle(12, 8, 4, p[0]/2, p[1]/2, p[2]/2)
		cv.glow(12, 8, 3, p[0], p[1], p[2])
		// Eyes (glowing)
		cv.glow(10, 7, 1, s[0], s[1], s[2])
		cv.glow(14, 7, 1, s[0], s[1], s[2])
		// Body
		cv.rect(8, 13, 8, 10, p[0]/4, p[1]/4, p[2]/4)
		// Pentagram on chest
		cv.glow(12, 17, 2, s[0], s[1], s[2])
		// Chains going down
		for y := 24; y < 32; y += 2 {
			cv.set(8, y, 100, 100, 100)
			cv.set(16, y, 100, 100, 100)
		}
		// Two captive figures
		cv.softCircle(5, 33, 2, 60, 50, 40)
		cv.softCircle(19, 33, 2, 60, 50, 40)
		cv.rect(4, 35, 3, 3, 40, 30, 25)
		cv.rect(18, 35, 3, 3, 40, 30, 25)

	case 16: // Tower — tall tower, lightning, falling figures
		// Tower
		cv.rect(8, 10, 8, 25, 60, 40, 30)
		cv.hline(8, 15, 10, 80, 50, 35)
		// Windows
		cv.set(10, 16, 20, 15, 10)
		cv.set(13, 16, 20, 15, 10)
		cv.set(10, 22, 20, 15, 10)
		cv.set(13, 22, 20, 15, 10)
		// Crown/top exploding
		cv.glow(12, 10, 3, p[0], p[1], p[2])
		cv.glow(12, 8, 2, s[0], s[1], s[2])
		// Lightning bolt
		cv.set(14, 2, p[0], p[1], p[2])
		cv.set(13, 3, p[0], p[1], p[2])
		cv.set(14, 4, p[0], p[1], p[2])
		cv.set(13, 5, p[0], p[1], p[2])
		cv.set(12, 6, p[0], p[1], p[2])
		cv.set(13, 7, p[0], p[1], p[2])
		cv.set(12, 8, p[0], p[1], p[2])
		cv.glow(13, 5, 2, p[0], p[1], p[2])
		// Falling figures
		cv.softCircle(4, 20, 2, 80, 60, 50)
		cv.softCircle(20, 24, 2, 80, 60, 50)
		// Flames at base
		for x := 7; x <= 16; x++ {
			h := 2 + int(math.Sin(float64(x)*1.5)*2)
			for y := 35 - h; y <= 35; y++ {
				cv.blend(x, y, s[0], s[1]/2, s[2]/3, 0.6)
			}
		}

	case 17: // Star — large star, water pouring, hope
		// Large 8-pointed star
		cv.glow(12, 10, 6, p[0], p[1], p[2])
		// Star points
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			for r := 0.0; r < 6; r += 0.5 {
				x := 12 + int(r*math.Cos(a))
				y := 10 + int(r*math.Sin(a))
				cv.blend(x, y, p[0], p[1], p[2], 0.8-r/10)
			}
		}
		cv.set(12, 10, 255, 255, 255) // bright center
		// Smaller stars
		cv.glow(4, 5, 1, p[0], p[1], p[2])
		cv.glow(20, 4, 1, p[0], p[1], p[2])
		cv.glow(6, 14, 1, p[0], p[1], p[2])
		cv.glow(19, 12, 1, p[0], p[1], p[2])
		cv.glow(3, 18, 1, p[0], p[1], p[2])
		cv.glow(21, 8, 1, p[0], p[1], p[2])
		cv.glow(18, 18, 1, p[0], p[1], p[2])
		// Figure pouring water
		cv.softCircle(12, 24, 2, 80, 60, 70)
		cv.rect(10, 27, 5, 5, 50, 40, 50)
		// Water streams
		for y := 30; y < 39; y++ {
			cv.blend(8, y, s[0], s[1], s[2], 0.5)
			cv.blend(16, y, s[0], s[1], s[2], 0.5)
		}
		// Pool at bottom
		for x := 2; x < 22; x++ {
			cv.blend(x, 38, s[0]/2, s[1]/2, s[2]/2, 0.4)
			cv.blend(x, 39, s[0]/2, s[1]/2, s[2]/2, 0.5)
		}

	case 18: // Moon — crescent, two towers, path, dogs
		// Moon (crescent)
		cv.softCircle(12, 8, 5, p[0], p[1], p[2])
		cv.softCircle(14, 7, 5, bgMid[0], bgMid[1], bgMid[2])
		cv.glow(10, 9, 5, p[0], p[1], p[2])
		// Two towers
		cv.rect(2, 18, 4, 14, 40, 35, 50)
		cv.rect(18, 18, 4, 14, 40, 35, 50)
		cv.glow(4, 18, 1, s[0], s[1], s[2])
		cv.glow(20, 18, 1, s[0], s[1], s[2])
		// Path between towers
		for y := 32; y < 40; y++ {
			w := 1 + (39-y)/3
			for x := 12 - w; x <= 12+w; x++ {
				cv.blend(x, y, 60, 55, 70, 0.4)
			}
		}
		// Dogs/wolves
		cv.rect(6, 30, 3, 2, 60, 50, 40)
		cv.set(6, 29, 60, 50, 40) // head
		cv.rect(15, 30, 3, 2, 60, 50, 40)
		cv.set(17, 29, 60, 50, 40)
		// Drops/rays from moon
		cv.blend(8, 16, p[0], p[1], p[2], 0.3)
		cv.blend(10, 17, p[0], p[1], p[2], 0.3)
		cv.blend(14, 16, p[0], p[1], p[2], 0.3)

	case 19: // Sun — large sun, rays, joyful figure
		// Sun circle
		cv.softCircle(12, 12, 6, p[0], p[1], p[2])
		cv.glow(12, 12, 6, p[0], p[1], p[2])
		// Face features
		cv.set(10, 11, s[0]/2, s[1]/2, s[2]/2)
		cv.set(14, 11, s[0]/2, s[1]/2, s[2]/2)
		cv.hline(10, 14, 14, s[0]/2, s[1]/2, s[2]/2)
		// Rays
		for i := 0; i < 12; i++ {
			a := float64(i) * math.Pi / 6
			for r := 7.0; r < 11; r += 0.5 {
				x := 12 + int(r*math.Cos(a))
				y := 12 + int(r*math.Sin(a))
				cv.blend(x, y, s[0], s[1], s[2], 0.4)
			}
		}
		// Child figure below
		cv.softCircle(12, 28, 2, 80, 65, 55)
		cv.rect(10, 31, 5, 4, 60, 50, 40)
		cv.rect(10, 35, 2, 3, 60, 50, 40)
		cv.rect(13, 35, 2, 3, 60, 50, 40)
		// Flowers/garden
		cv.glow(4, 36, 1, 0, 200, 80)
		cv.glow(8, 38, 1, 200, 50, 50)
		cv.glow(16, 37, 1, 0, 200, 80)
		cv.glow(20, 36, 1, 200, 200, 50)

	case 20: // Judgement — angel with trumpet, rising figures
		// Angel/trumpet at top
		cv.softCircle(12, 7, 3, p[0], p[1], p[2])
		cv.glow(12, 7, 3, p[0], p[1], p[2])
		// Wings
		for i := 0; i < 5; i++ {
			cv.set(6-i, 9+i/2, s[0]/2, s[1]/2, s[2]/2)
			cv.set(18+i, 9+i/2, s[0]/2, s[1]/2, s[2]/2)
		}
		// Trumpet
		cv.hline(12, 20, 12, s[0], s[1], s[2])
		cv.set(20, 11, s[0], s[1], s[2])
		cv.set(20, 13, s[0], s[1], s[2])
		cv.glow(20, 12, 2, s[0], s[1], s[2])
		// Sound waves
		for i := 0; i < 3; i++ {
			for a := -0.5; a < 0.5; a += 0.2 {
				x := 21 + i
				y := 12 + int(float64(i)*math.Sin(a*3))
				cv.blend(x, y, s[0], s[1], s[2], 0.4-float64(i)*0.1)
			}
		}
		// Rising figures from below
		cv.softCircle(6, 28, 2, 80, 70, 60)
		cv.rect(5, 31, 3, 5, 60, 50, 40)
		cv.softCircle(12, 26, 2, 80, 70, 60)
		cv.rect(11, 29, 3, 5, 60, 50, 40)
		cv.softCircle(18, 28, 2, 80, 70, 60)
		cv.rect(17, 31, 3, 5, 60, 50, 40)
		// Glow around rising figures
		cv.glow(12, 30, 5, p[0]/2, p[1]/2, p[2]/2)
		// Coffins/ground
		cv.hline(0, 23, 36, 50, 40, 30)

	case 21: // World — wreath/circle, dancing figure
		// Wreath (oval)
		for a := 0.0; a < 2*math.Pi; a += 0.12 {
			x := 12 + int(9*math.Cos(a))
			y := 20 + int(12*math.Sin(a))
			cv.set(x, y, p[0], p[1], p[2])
			cv.blend(x, y, p[0], p[1], p[2], 0.8)
		}
		cv.glow(12, 8, 2, p[0], p[1], p[2])
		cv.glow(12, 32, 2, p[0], p[1], p[2])
		cv.glow(3, 20, 2, p[0], p[1], p[2])
		cv.glow(21, 20, 2, p[0], p[1], p[2])
		// Dancing figure inside
		cv.softCircle(12, 16, 2, 80, 65, 55)
		cv.glow(12, 16, 2, s[0], s[1], s[2])
		cv.rect(10, 19, 5, 7, 50, 40, 30)
		// Arms out
		cv.set(8, 20, 80, 65, 55)
		cv.set(7, 19, 80, 65, 55)
		cv.set(16, 20, 80, 65, 55)
		cv.set(17, 19, 80, 65, 55)
		// Legs
		cv.set(10, 26, 50, 40, 30)
		cv.set(9, 27, 50, 40, 30)
		cv.set(14, 26, 50, 40, 30)
		cv.set(15, 27, 50, 40, 30)
		// Four corner symbols (the four fixed signs)
		cv.drawSuit(3, 5, 0)
		cv.drawSuit(21, 5, 1)
		cv.drawSuit(3, 35, 2)
		cv.drawSuit(21, 35, 3)
	}

	cv.scanlines()
	return cv
}

// --- Card back generator ---

func genBack() Canvas {
	var cv Canvas
	// Deep purple gradient
	cv.fillGrad(rgb{20, 10, 35}, rgb{8, 4, 16})

	// Diamond pattern
	for y := 0; y < 40; y += 8 {
		for x := 0; x < 24; x += 8 {
			cv.stampGlow(x+1, y+1, 5, diamondBmp, 139, 47, 201, 2)
		}
	}
	// Offset row
	for y := 4; y < 40; y += 8 {
		for x := 4; x < 24; x += 8 {
			cv.stampGlow(x+1, y+1, 5, diamondBmp, 100, 40, 160, 2)
		}
	}

	// Border accents
	for x := 0; x < 24; x++ {
		cv.blend(x, 0, 139, 47, 201, 0.3)
		cv.blend(x, 39, 139, 47, 201, 0.3)
	}
	for y := 0; y < 40; y++ {
		cv.blend(0, y, 139, 47, 201, 0.3)
		cv.blend(23, y, 139, 47, 201, 0.3)
	}

	// Central glow
	cv.glow(12, 20, 6, 139, 47, 201)

	cv.circuits(rgb{60, 25, 90}, 0.15)
	cv.scanlines()
	return cv
}

// --- Main ---

func main() {
	base := "decks/cyberpunk"

	// Card back
	fmt.Print("Generating card back...")
	genBack().writeHex(filepath.Join(base, "back.hex"))
	fmt.Println(" done")

	// Major Arcana
	for i := 0; i <= 21; i++ {
		slug := majorSlugs[i]
		path := filepath.Join(base, "major", fmt.Sprintf("%02d-%s.hex", i, slug))
		fmt.Printf("Major %02d %-25s", i, slug)
		genMajor(i).writeHex(path)
		fmt.Println(" done")
	}

	// Minor Arcana
	for si := 0; si < 4; si++ {
		suitName := suitFileNames[si]
		// Pip cards 1-10
		for num := 1; num <= 10; num++ {
			path := filepath.Join(base, "minor", fmt.Sprintf("%s-%02d.hex", suitName, num))
			fmt.Printf("Minor %-10s %02d", suitName, num)
			genPip(si, num).writeHex(path)
			fmt.Println(" done")
		}
		// Court cards 11-14
		for rank := 11; rank <= 14; rank++ {
			path := filepath.Join(base, "minor", fmt.Sprintf("%s-%02d.hex", suitName, rank))
			rankName := []string{"Page", "Knight", "Queen", "King"}[rank-11]
			fmt.Printf("Minor %-10s %s", suitName, rankName)
			genCourt(si, rank).writeHex(path)
			fmt.Println(" done")
		}
	}

	fmt.Printf("\nGenerated 79 cards in %s/\n", base)
}
