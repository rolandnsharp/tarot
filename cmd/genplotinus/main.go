package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
)

// ---------- hi-res canvas (240×400) ----------

const (
	hiW = 240
	hiH = 400
	loW = 24
	loH = 40
)

type HiResCanvas struct {
	img *image.RGBA
}

func newCanvas() *HiResCanvas {
	return &HiResCanvas{img: image.NewRGBA(image.Rect(0, 0, hiW, hiH))}
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func clampU8(v float64) uint8 { return uint8(clamp(v)) }

func lerpColor(a, b color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		clampU8(float64(a.R)*(1-t) + float64(b.R)*t),
		clampU8(float64(a.G)*(1-t) + float64(b.G)*t),
		clampU8(float64(a.B)*(1-t) + float64(b.B)*t),
		255,
	}
}

func (c *HiResCanvas) set(x, y int, col color.RGBA) {
	if x >= 0 && x < hiW && y >= 0 && y < hiH {
		c.img.SetRGBA(x, y, col)
	}
}

func (c *HiResCanvas) at(x, y int) color.RGBA {
	if x >= 0 && x < hiW && y >= 0 && y < hiH {
		return c.img.RGBAAt(x, y)
	}
	return color.RGBA{}
}

func (c *HiResCanvas) blend(x, y int, col color.RGBA, alpha float64) {
	if x < 0 || x >= hiW || y < 0 || y >= hiH || alpha <= 0 {
		return
	}
	bg := c.at(x, y)
	c.set(x, y, color.RGBA{
		clampU8(float64(bg.R)*(1-alpha) + float64(col.R)*alpha),
		clampU8(float64(bg.G)*(1-alpha) + float64(col.G)*alpha),
		clampU8(float64(bg.B)*(1-alpha) + float64(col.B)*alpha),
		255,
	})
}

// --- gradient fills ---

func (c *HiResCanvas) fillGradient(top, bot color.RGBA) {
	for y := 0; y < hiH; y++ {
		t := float64(y) / float64(hiH-1)
		col := lerpColor(top, bot, t)
		for x := 0; x < hiW; x++ {
			c.img.SetRGBA(x, y, col)
		}
	}
}

func (c *HiResCanvas) fillRadialGradient(cx, cy, radius float64, inner, outer color.RGBA) {
	r := int(radius) + 1
	for y := int(cy) - r; y <= int(cy)+r; y++ {
		for x := int(cx) - r; x <= int(cx)+r; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			d := math.Sqrt(dx*dx+dy*dy) / radius
			if d > 1 {
				continue
			}
			col := lerpColor(inner, outer, d)
			c.blend(x, y, col, 1.0-d*0.3)
		}
	}
}

// --- shape primitives ---

func (c *HiResCanvas) filledRect(x, y, w, h float64, col color.RGBA) {
	for py := int(y); py < int(y+h); py++ {
		for px := int(x); px < int(x+w); px++ {
			c.set(px, py, col)
		}
	}
}

func (c *HiResCanvas) filledCircle(cx, cy, r float64, col color.RGBA) {
	ir := int(r) + 2
	for y := int(cy) - ir; y <= int(cy)+ir; y++ {
		for x := int(cx) - ir; x <= int(cx)+ir; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			d := math.Sqrt(dx*dx + dy*dy)
			if d < r-1 {
				c.set(x, y, col)
			} else if d < r+1 {
				a := 1.0 - (d - (r - 1)) / 2.0
				if a > 0 {
					c.blend(x, y, col, a)
				}
			}
		}
	}
}

func (c *HiResCanvas) circle(cx, cy, r, thickness float64, col color.RGBA) {
	ir := int(r+thickness) + 2
	for y := int(cy) - ir; y <= int(cy)+ir; y++ {
		for x := int(cx) - ir; x <= int(cx)+ir; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			d := math.Sqrt(dx*dx + dy*dy)
			dist := math.Abs(d - r)
			if dist < thickness/2+1 {
				a := 1.0 - math.Max(0, dist-thickness/2)
				if a > 0 {
					c.blend(x, y, col, a)
				}
			}
		}
	}
}

func (c *HiResCanvas) line(x1, y1, x2, y2, thickness float64, col color.RGBA) {
	dx := x2 - x1
	dy := y2 - y1
	length := math.Sqrt(dx*dx + dy*dy)
	if length < 0.5 {
		c.filledCircle(x1, y1, thickness/2, col)
		return
	}
	steps := int(length * 2)
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		px := x1 + dx*t
		py := y1 + dy*t
		c.filledCircle(px, py, thickness/2, col)
	}
}

func (c *HiResCanvas) arc(cx, cy, r, startAngle, endAngle, thickness float64, col color.RGBA) {
	steps := int((endAngle - startAngle) * r * 2)
	if steps < 10 {
		steps = 10
	}
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		a := startAngle + (endAngle-startAngle)*t
		px := cx + r*math.Cos(a)
		py := cy + r*math.Sin(a)
		c.filledCircle(px, py, thickness/2, col)
	}
}

func (c *HiResCanvas) glow(cx, cy, radius float64, col color.RGBA) {
	ir := int(radius*2.5) + 1
	for y := int(cy) - ir; y <= int(cy)+ir; y++ {
		for x := int(cx) - ir; x <= int(cx)+ir; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			d2 := dx*dx + dy*dy
			a := math.Exp(-d2 / (2 * radius * radius))
			if a > 0.02 {
				c.blend(x, y, col, a)
			}
		}
	}
}

// --- bezier curves ---

func bezierPoint(t, x0, y0, x1, y1, x2, y2, x3, y3 float64) (float64, float64) {
	u := 1 - t
	x := u*u*u*x0 + 3*u*u*t*x1 + 3*u*t*t*x2 + t*t*t*x3
	y := u*u*u*y0 + 3*u*u*t*y1 + 3*u*t*t*y2 + t*t*t*y3
	return x, y
}

func (c *HiResCanvas) bezier(x0, y0, x1, y1, x2, y2, x3, y3, thickness float64, col color.RGBA) {
	steps := 200
	var prevX, prevY float64
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		px, py := bezierPoint(t, x0, y0, x1, y1, x2, y2, x3, y3)
		if i > 0 {
			c.line(prevX, prevY, px, py, thickness, col)
		}
		prevX, prevY = px, py
	}
}

// --- downscale & output ---

func (c *HiResCanvas) downscale() [loH][loW][3]uint8 {
	dst := image.NewRGBA(image.Rect(0, 0, loW, loH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), c.img, c.img.Bounds(), draw.Over, nil)
	var result [loH][loW][3]uint8
	for y := 0; y < loH; y++ {
		for x := 0; x < loW; x++ {
			r, g, b, _ := dst.At(x, y).RGBA()
			result[y][x] = [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
		}
	}
	return result
}

func writeHex(pixels [loH][loW][3]uint8, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var sb strings.Builder
	for y := 0; y < loH; y++ {
		for x := 0; x < loW; x++ {
			if x > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(fmt.Sprintf("%02x%02x%02x", pixels[y][x][0], pixels[y][x][1], pixels[y][x][2]))
		}
		sb.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

func (c *HiResCanvas) writeHexFile(path string) error {
	return writeHex(c.downscale(), path)
}

// ========== Plotinus / Neoplatonic color palette ==========

var (
	void     = color.RGBA{12, 8, 30, 255}    // deep indigo-black — the formless
	voidBot  = color.RGBA{5, 3, 18, 255}     // deeper void at bottom
	theOne   = color.RGBA{255, 248, 220, 255} // pure radiant white-gold
	gold     = color.RGBA{230, 200, 80, 255}  // emanation gold
	goldDim  = color.RGBA{140, 120, 45, 255}  // dimmer gold
	nous     = color.RGBA{140, 170, 220, 255} // intellect silver-blue
	nousDim  = color.RGBA{70, 85, 120, 255}   // dim intellect
	psyche   = color.RGBA{180, 120, 175, 255} // soul violet-rose
	psycheDim = color.RGBA{100, 60, 95, 255}  // dim soul
	dynamis  = color.RGBA{240, 190, 60, 255}  // power gold-amber
	dynamisDim = color.RGBA{150, 115, 35, 255} // dim power
	physis   = color.RGBA{160, 130, 80, 255}  // matter bronze-earth
	physisDim = color.RGBA{90, 72, 45, 255}   // dim matter
	indigo   = color.RGBA{40, 30, 90, 255}    // deep emanation blue
	indigoDim = color.RGBA{25, 18, 55, 255}   // darker indigo
)

// suit colors: swords=nous, cups=psyche, wands=dynamis, pentacles=physis
type suitColors struct {
	primary, dim color.RGBA
}

var suitPalette = []suitColors{
	{psyche, psycheDim},   // cups (index 0) → Psyche (Soul)
	{dynamis, dynamisDim}, // wands (index 1) → Dynamis (Power)
	{nous, nousDim},       // swords (index 2) → Nous (Intellect)
	{physis, physisDim},   // pentacles (index 3) → Physis (Nature)
}

var suitFileNames = []string{"cups", "wands", "swords", "pentacles"}

// ========== Plotinus visual elements ==========

// emanationRings draws concentric circles fading from bright center to dark edge
func (c *HiResCanvas) emanationRings(cx, cy, maxR float64) {
	rings := 8
	for i := 0; i < rings; i++ {
		t := float64(i) / float64(rings-1)
		r := maxR * (0.15 + t*0.85)
		col := lerpColor(theOne, indigo, t)
		alpha := 0.7 - t*0.4
		c.circle(cx, cy, r, 1.5, col)
		// Fill ring region with dim glow
		if i > 0 {
			prevR := maxR * (0.15 + float64(i-1)/float64(rings-1)*0.85)
			for py := int(cy - r - 1); py <= int(cy+r+1); py++ {
				for px := int(cx - r - 1); px <= int(cx+r+1); px++ {
					dx := float64(px) - cx
					dy := float64(py) - cy
					d := math.Sqrt(dx*dx + dy*dy)
					if d >= prevR && d < r {
						c.blend(px, py, col, alpha*0.15)
					}
				}
			}
		}
	}
	// Central glow
	c.glow(cx, cy, maxR*0.2, theOne)
	c.filledCircle(cx, cy, maxR*0.08, theOne)
}

// lightRays draws radial lines from a point
func (c *HiResCanvas) lightRays(cx, cy float64, count int, length float64) {
	for i := 0; i < count; i++ {
		a := float64(i) * 2 * math.Pi / float64(count)
		x2 := cx + length*math.Cos(a)
		y2 := cy + length*math.Sin(a)
		// Fade from bright to dim along ray
		steps := int(length)
		for s := 0; s < steps; s++ {
			t := float64(s) / float64(steps)
			px := cx + (x2-cx)*t
			py := cy + (y2-cy)*t
			alpha := 0.6 * (1 - t)
			col := lerpColor(theOne, gold, t)
			c.blend(int(px), int(py), col, alpha)
			// Slightly wider at base
			if t < 0.3 {
				c.blend(int(px)+1, int(py), col, alpha*0.5)
				c.blend(int(px)-1, int(py), col, alpha*0.5)
			}
		}
	}
}

// tetractys draws the 10-dot triangular arrangement (1+2+3+4)
func (c *HiResCanvas) tetractys(cx, cy, size float64) {
	rows := []int{1, 2, 3, 4}
	dotR := size * 0.08
	spacing := size * 0.3
	totalH := 3 * spacing
	startY := cy - totalH/2
	for row, count := range rows {
		y := startY + float64(row)*spacing
		startX := cx - float64(count-1)*spacing/2
		for i := 0; i < count; i++ {
			x := startX + float64(i)*spacing
			c.filledCircle(x, y, dotR, theOne)
			c.glow(x, y, dotR*2, gold)
		}
	}
}

// greekKey draws a meander/Greek key pattern along a horizontal line
func (c *HiResCanvas) greekKey(x, y, w float64, col color.RGBA) {
	unit := 5.0
	thick := 1.2
	px := x
	for px < x+w-unit*4 {
		// Each key unit: right, up, right, down, down, left, up
		c.line(px, y, px+unit*2, y, thick, col)
		c.line(px+unit*2, y, px+unit*2, y-unit, thick, col)
		c.line(px+unit*2, y-unit, px+unit, y-unit, thick, col)
		c.line(px+unit, y-unit, px+unit, y+unit, thick, col)
		c.line(px+unit, y+unit, px+unit*3, y+unit, thick, col)
		c.line(px+unit*3, y+unit, px+unit*3, y-unit*1.5, thick, col)
		c.line(px+unit*3, y-unit*1.5, px, y-unit*1.5, thick, col)
		c.line(px, y-unit*1.5, px, y, thick, col)
		px += unit * 4
	}
}

// plotinusBorder draws the double-line border with Greek key and corner glow points
func (c *HiResCanvas) plotinusBorder(col color.RGBA) {
	w := float64(hiW)
	h := float64(hiH)
	m := 8.0

	// Outer frame
	c.line(m, m, w-m, m, 2.0, col)
	c.line(w-m, m, w-m, h-m, 2.0, col)
	c.line(w-m, h-m, m, h-m, 2.0, col)
	c.line(m, h-m, m, m, 2.0, col)

	// Inner frame
	m2 := m + 6
	c.line(m2, m2, w-m2, m2, 1.2, col)
	c.line(w-m2, m2, w-m2, h-m2, 1.2, col)
	c.line(w-m2, h-m2, m2, h-m2, 1.2, col)
	c.line(m2, h-m2, m2, m2, 1.2, col)

	// Greek key along top and bottom (between outer and inner frame)
	keyY := m + 3
	c.greekKey(m+10, keyY, w-2*m-20, goldDim)
	keyY = h - m - 3
	c.greekKey(m+10, keyY, w-2*m-20, goldDim)

	// Corner glow points — The One emanating at corners
	corners := [][2]float64{{m, m}, {w - m, m}, {m, h - m}, {w - m, h - m}}
	for _, p := range corners {
		c.glow(p[0], p[1], 8, gold)
		c.filledCircle(p[0], p[1], 3, theOne)
	}
}

// ========== Plotinus background ==========

func plotinusBG() *HiResCanvas {
	cv := newCanvas()
	cv.fillGradient(void, voidBot)
	return cv
}

// ========== suit pip shapes (Neoplatonic symbols) ==========

// drawCup — Psyche: upward-opening arc with inner glow (soul receiving light)
func (c *HiResCanvas) drawCup(cx, cy, size float64, col color.RGBA) {
	// Upward-opening arc — the soul receiving light
	c.arc(cx, cy+size*0.1, size*0.4, math.Pi+0.3, 2*math.Pi-0.3, 2.5, col)
	// Inner glow — the light received
	c.glow(cx, cy, size*0.2, theOne)
	// Stem
	c.line(cx, cy+size*0.3, cx, cy+size*0.55, 2, col)
	// Base
	c.line(cx-size*0.2, cy+size*0.55, cx+size*0.2, cy+size*0.55, 2, col)
	// Soul-light descending into cup
	c.filledCircle(cx, cy-size*0.05, size*0.1, theOne)
}

// drawWand — Dynamis: vertical beam with expanding rays (emanation force)
func (c *HiResCanvas) drawWand(cx, cy, size float64, col color.RGBA) {
	// Central beam
	c.line(cx, cy-size*0.5, cx, cy+size*0.5, 2.5, col)
	// Expanding rays from top
	for i := 0; i < 5; i++ {
		a := -math.Pi/2 + float64(i-2)*0.35
		rx := cx + size*0.35*math.Cos(a)
		ry := (cy - size*0.45) + size*0.25*math.Sin(a)
		c.line(cx, cy-size*0.45, rx, ry, 1.5, col)
	}
	// Top glow — the overflow from The One
	c.glow(cx, cy-size*0.5, size*0.2, theOne)
	c.filledCircle(cx, cy-size*0.5, size*0.08, theOne)
}

// drawSword — Nous: vertical line of light with radiating horizontal bars (Form/Idea)
func (c *HiResCanvas) drawSword(cx, cy, size float64, col color.RGBA) {
	// Vertical line of light
	c.line(cx, cy-size*0.55, cx, cy+size*0.4, 2.5, col)
	// Horizontal bars — radiating Forms
	bars := 3
	for i := 0; i < bars; i++ {
		t := float64(i) / float64(bars)
		y := cy - size*0.35 + t*size*0.5
		hw := size * 0.25 * (1 - t*0.3)
		c.line(cx-hw, y, cx+hw, y, 1.8, col)
	}
	// Top glow — intellectual light
	c.glow(cx, cy-size*0.55, size*0.15, theOne)
	c.filledCircle(cx, cy-size*0.55, size*0.06, theOne)
}

// drawPentacle — Physis: circle with inscribed triangle (matter containing form)
func (c *HiResCanvas) drawPentacle(cx, cy, size float64, col color.RGBA) {
	// Outer circle — matter
	c.circle(cx, cy, size*0.4, 2.0, col)
	// Inscribed triangle — form within matter
	r := size * 0.32
	for i := 0; i < 3; i++ {
		a1 := float64(i)*2*math.Pi/3 - math.Pi/2
		a2 := float64(i+1)*2*math.Pi/3 - math.Pi/2
		x1 := cx + r*math.Cos(a1)
		y1 := cy + r*math.Sin(a1)
		x2 := cx + r*math.Cos(a2)
		y2 := cy + r*math.Sin(a2)
		c.line(x1, y1, x2, y2, 1.5, col)
	}
	// Center dot — the spark of form
	c.filledCircle(cx, cy, size*0.06, theOne)
}

func (c *HiResCanvas) drawSuitPip(si int, cx, cy, size float64) {
	col := suitPalette[si].primary
	switch si {
	case 0:
		c.drawCup(cx, cy, size, col)
	case 1:
		c.drawWand(cx, cy, size, col)
	case 2:
		c.drawSword(cx, cy, size, col)
	case 3:
		c.drawPentacle(cx, cy, size, col)
	}
}

// ========== number glyphs (hi-res) ==========

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

func (c *HiResCanvas) drawDigit(sx, sy float64, d int, col color.RGBA, scale float64) {
	bmp, ok := digits[d]
	if !ok {
		return
	}
	ps := scale
	for i, ch := range bmp {
		if ch == 'X' {
			x := sx + float64(i%3)*ps
			y := sy + float64(i/3)*ps
			c.filledRect(x, y, ps, ps, col)
		}
	}
}

func (c *HiResCanvas) drawNum(sx, sy float64, n int, col color.RGBA, scale float64) {
	if n >= 10 {
		c.drawDigit(sx, sy, n/10, col, scale)
		c.drawDigit(sx+4*scale, sy, n%10, col, scale)
	} else {
		c.drawDigit(sx, sy, n, col, scale)
	}
}

// ========== pip positions (scaled to 240×400) ==========

var pipPositions = map[int][][2]float64{
	1:  {{120, 200}},
	2:  {{120, 100}, {120, 300}},
	3:  {{120, 80}, {120, 200}, {120, 320}},
	4:  {{70, 80}, {170, 80}, {70, 320}, {170, 320}},
	5:  {{70, 80}, {170, 80}, {120, 200}, {70, 320}, {170, 320}},
	6:  {{70, 80}, {170, 80}, {70, 200}, {170, 200}, {70, 320}, {170, 320}},
	7:  {{70, 80}, {170, 80}, {120, 140}, {70, 200}, {170, 200}, {70, 320}, {170, 320}},
	8:  {{70, 80}, {170, 80}, {120, 140}, {70, 200}, {170, 200}, {120, 260}, {70, 320}, {170, 320}},
	9:  {{70, 70}, {170, 70}, {70, 150}, {170, 150}, {120, 200}, {70, 250}, {170, 250}, {70, 330}, {170, 330}},
	10: {{70, 60}, {170, 60}, {120, 110}, {70, 160}, {170, 160}, {70, 240}, {170, 240}, {120, 290}, {70, 340}, {170, 340}},
}

// ========== card generators ==========

func genBack() *HiResCanvas {
	cv := newCanvas()
	cv.fillGradient(void, voidBot)

	// Concentric emanation rings from golden center — unity to multiplicity
	cv.emanationRings(120, 200, 100)

	// Light rays from center
	cv.lightRays(120, 200, 12, 90)

	// Plotinus border with Greek key
	cv.plotinusBorder(goldDim)

	// Corner tetractys (small)
	cv.tetractys(35, 35, 35)
	cv.tetractys(205, 35, 35)
	cv.tetractys(35, 365, 35)
	cv.tetractys(205, 365, 35)

	// Central glow reinforcement
	cv.glow(120, 200, 30, theOne)

	return cv
}

func genPip(si, num int) *HiResCanvas {
	cv := plotinusBG()
	sc := suitPalette[si]

	// Subtle emanation rings in background
	cv.emanationRings(120, 200, 80)

	// Plotinus border
	cv.plotinusBorder(sc.dim)

	// Corner numbers
	cv.drawNum(16, 18, num, sc.primary, 5)
	cv.drawNum(186, 352, num, sc.primary, 5)

	// Small glow at top center
	cv.glow(120, 30, 12, sc.primary)

	// Draw pips
	if pos, ok := pipPositions[num]; ok {
		pipSize := 30.0
		if num >= 8 {
			pipSize = 25.0
		}
		for _, p := range pos {
			cv.drawSuitPip(si, p[0], p[1], pipSize)
		}
	}

	return cv
}

// suitGlassColor returns a dim background tint for court card panels
func suitGlassColor(si int) color.RGBA {
	switch si {
	case 0:
		return color.RGBA{35, 18, 32, 255} // dim psyche
	case 1:
		return color.RGBA{40, 32, 12, 255} // dim dynamis
	case 2:
		return color.RGBA{18, 22, 40, 255} // dim nous
	case 3:
		return color.RGBA{28, 24, 15, 255} // dim physis
	}
	return void
}

func genCourt(si, rank int) *HiResCanvas {
	cv := plotinusBG()
	sc := suitPalette[si]
	p := sc.primary

	// Background emanation
	cv.emanationRings(120, 200, 90)

	// Plotinus border
	cv.plotinusBorder(sc.dim)

	// Glass panel background
	bg := suitGlassColor(si)
	cv.filledRect(28, 48, 184, 304, bg)
	// Panel border
	c2 := sc.dim
	cv.line(28, 48, 212, 48, 1.5, c2)
	cv.line(28, 352, 212, 352, 1.5, c2)
	cv.line(28, 48, 28, 352, 1.5, c2)
	cv.line(212, 48, 212, 352, 1.5, c2)

	switch rank {
	case 11: // Page — young light-form emerging from emanation
		// Figure as light silhouette
		cv.glow(120, 130, 25, p)
		cv.filledCircle(120, 115, 20, lerpColor(p, theOne, 0.3))
		cv.glow(120, 115, 12, theOne)
		// Body — light form
		for y := 140.0; y < 260; y++ {
			t := (y - 140) / 120
			w := 15 + t*10
			col := lerpColor(p, sc.dim, t*0.6)
			cv.filledRect(120-w, y, w*2, 1, col)
		}
		// Suit symbol held forward
		cv.drawSuitPip(si, 120, 195, 28)
		// Emanation rings around figure
		cv.circle(120, 180, 55, 1.0, sc.dim)
		cv.circle(120, 180, 70, 0.8, indigoDim)
		// Small light rays from head
		cv.lightRays(120, 115, 6, 25)

	case 12: // Knight — active emanation figure, radiating outward
		// Head with strong glow
		cv.glow(110, 105, 28, p)
		cv.filledCircle(110, 105, 22, lerpColor(p, theOne, 0.4))
		cv.glow(110, 105, 14, theOne)
		// Body — dynamic pose
		cv.filledRect(92, 130, 36, 90, p)
		// Extended arm with suit symbol
		cv.line(128, 160, 190, 140, 3, p)
		cv.drawSuitPip(si, 190, 135, 26)
		cv.glow(190, 135, 15, p)
		// Legs in motion
		cv.line(100, 220, 85, 290, 2.5, sc.dim)
		cv.line(118, 220, 135, 290, 2.5, sc.dim)
		// Dynamic emanation rays
		cv.lightRays(110, 105, 8, 35)
		// Emanation wake
		cv.circle(110, 170, 50, 1.0, sc.dim)

	case 13: // Queen — seated in emanation, receiving light from above
		// Crown of light
		for i := 0; i < 5; i++ {
			x := 100 + float64(i)*10
			cv.glow(x, 62, 6, gold)
			cv.filledCircle(x, 60, 3, theOne)
		}
		cv.line(98, 68, 142, 68, 2, gold)
		// Head
		cv.filledCircle(120, 85, 20, lerpColor(p, theOne, 0.3))
		cv.glow(120, 85, 12, theOne)
		// Flowing robes of light
		for y := 110.0; y < 300; y++ {
			t := (y - 110) / 190
			w := 25 + t*55
			col := lerpColor(p, sc.dim, t*0.5)
			alpha := 0.9 - t*0.3
			for px := int(120 - w); px < int(120+w); px++ {
				cv.blend(px, int(y), col, alpha)
			}
		}
		// Suit symbol on chest
		cv.drawSuitPip(si, 120, 180, 28)
		// Descending light beam from above
		cv.line(120, 48, 120, 72, 2, theOne)
		cv.glow(120, 55, 10, theOne)
		// Concentric emanation behind figure
		cv.circle(120, 180, 60, 1.0, sc.dim)
		cv.circle(120, 180, 80, 0.8, indigoDim)

	case 14: // King — enthroned in full emanation, maximal light
		// Crown — more elaborate, with rays
		for i := 0; i < 7; i++ {
			x := 92 + float64(i)*8
			h := 12.0
			if i%2 == 0 {
				h = 18
			}
			cv.line(x, 55-h, x, 58, 2.5, gold)
			cv.filledCircle(x, 55-h-3, 4, theOne)
			cv.glow(x, 55-h-3, 5, gold)
		}
		cv.filledRect(88, 58, 64, 6, gold)
		// Head
		cv.filledCircle(120, 78, 22, lerpColor(p, theOne, 0.4))
		cv.glow(120, 78, 15, theOne)
		// Broad robes of light
		cv.filledRect(80, 108, 80, 130, p)
		cv.line(80, 108, 160, 108, 2, gold)
		// Scepter — beam of emanation
		cv.line(175, 95, 175, 280, 3, gold)
		cv.glow(175, 95, 10, theOne)
		cv.filledCircle(175, 92, 5, theOne)
		// Legs
		cv.filledRect(90, 238, 20, 55, sc.dim)
		cv.filledRect(130, 238, 20, 55, sc.dim)
		// Suit symbol on chest
		cv.drawSuitPip(si, 120, 165, 30)
		// Throne pillars — pillars of emanation
		cv.line(55, 60, 55, 295, 2.5, indigo)
		cv.line(185, 60, 185, 295, 2.5, indigo)
		cv.glow(55, 60, 8, gold)
		cv.glow(185, 60, 8, gold)
		// Full emanation rings
		cv.circle(120, 170, 65, 1.0, sc.dim)
		cv.circle(120, 170, 85, 0.8, indigoDim)
	}

	// Corner rank letter
	rankLetters := map[int]string{
		11: ".X." + "X.X" + "XXX" + "X.." + "X..", // P
		12: "X.X" + "X.X" + "XX." + "X.X" + "X.X", // N (Knight)
		13: "XX." + "X.X" + "X.X" + "X.X" + "XX.", // Q
		14: "X.X" + "X.X" + "XX." + "X.X" + "X.X", // K
	}
	if bmp, ok := rankLetters[rank]; ok {
		for i, ch := range bmp {
			if ch == 'X' {
				x := 15 + float64(i%3)*5
				y := 15 + float64(i/3)*5
				cv.filledRect(x, y, 5, 5, sc.primary)
			}
		}
	}

	return cv
}

// ========== major arcana ==========

var majorSlugs = []string{
	"the-fool", "the-magician", "the-high-priestess", "the-empress",
	"the-emperor", "the-hierophant", "the-lovers", "the-chariot",
	"strength", "the-hermit", "wheel-of-fortune", "justice",
	"the-hanged-man", "death", "temperance", "the-devil",
	"the-tower", "the-star", "the-moon", "the-sun",
	"judgement", "the-world",
}

type majorScheme struct {
	primary, secondary color.RGBA
}

var majorSchemes = []majorScheme{
	{theOne, gold},         // 0  Fool — innocent soul before descent
	{gold, nous},           // 1  Magician — active intellect
	{nous, psyche},         // 2  High Priestess — Sophia, divine wisdom
	{psyche, gold},         // 3  Empress — World Soul as mother
	{dynamis, gold},        // 4  Emperor — Demiurgic power
	{gold, theOne},         // 5  Hierophant — hieratic teacher
	{psyche, theOne},       // 6  Lovers — soul's choice
	{dynamis, nous},        // 7  Chariot — will in motion
	{gold, dynamis},        // 8  Strength — courage of the soul
	{theOne, indigo},       // 9  Hermit — contemplative withdrawal
	{gold, indigo},         // 10 Wheel — cycles of emanation/return
	{nous, gold},           // 11 Justice — cosmic order / Dike
	{psyche, indigo},       // 12 Hanged Man — surrender, katharsis
	{theOne, physis},       // 13 Death — dissolution of form
	{nous, psyche},         // 14 Temperance — harmonizing soul
	{physis, indigo},       // 15 Devil — soul trapped in matter
	{dynamis, physis},      // 16 Tower — shattering of illusion
	{theOne, nous},         // 17 Star — glimpse of The One
	{psyche, indigo},       // 18 Moon — soul in confusion
	{theOne, gold},         // 19 Sun — intellectual illumination
	{gold, theOne},         // 20 Judgement — call to ascent
	{theOne, gold},         // 21 World — henosis, return to The One
}

func genMajor(num int) *HiResCanvas {
	cv := plotinusBG()
	sc := majorSchemes[num]
	p, s := sc.primary, sc.secondary

	// Background emanation rings
	cv.emanationRings(120, 200, 85)

	// Number in corner
	cv.drawNum(15, 15, num, gold, 5)

	// Plotinus border
	cv.plotinusBorder(goldDim)

	switch num {
	case 0: // Fool — innocent soul at the edge of descent into matter
		// Cliff edge — the boundary between the intelligible and material
		for y := 280; y < 370; y++ {
			t := float64(y-280) / 90
			w := 120 - t*40
			col := lerpColor(physis, voidBot, t*0.5)
			cv.filledRect(25, float64(y), w, 1, col)
		}
		// Figure — luminous soul
		cv.glow(130, 175, 30, p)
		cv.filledCircle(130, 175, 20, lerpColor(p, theOne, 0.5))
		cv.glow(130, 175, 14, theOne)
		// Light-body
		for y := 200.0; y < 280; y++ {
			t := (y - 200) / 80
			w := 15 + t*12
			col := lerpColor(p, gold, t)
			cv.filledRect(130-w, y, w*2, 1, col)
		}
		// Star above — remembrance of The One
		cv.glow(180, 65, 18, theOne)
		cv.filledCircle(180, 65, 6, theOne)
		cv.lightRays(180, 65, 8, 20)
		// Looking toward the void below
		cv.glow(80, 340, 25, indigo)

	case 1: // Magician — active intellect channeling all hypostases
		// The One above — source of power
		cv.glow(120, 55, 25, theOne)
		cv.filledCircle(120, 55, 10, theOne)
		cv.lightRays(120, 55, 8, 30)
		// Descending beam to figure
		cv.line(120, 75, 120, 120, 2.5, gold)
		// Figure
		cv.filledCircle(120, 135, 22, lerpColor(p, theOne, 0.3))
		cv.glow(120, 135, 14, theOne)
		// Body
		cv.filledRect(105, 160, 30, 80, s)
		// Arms extended — channeling
		cv.line(105, 180, 55, 165, 2.5, nous)
		cv.line(135, 180, 185, 165, 2.5, dynamis)
		cv.glow(55, 165, 10, nous)
		cv.glow(185, 165, 10, dynamis)
		// Table of elements — four hypostases
		cv.filledRect(40, 260, 160, 6, goldDim)
		cv.drawSuitPip(0, 60, 295, 22)
		cv.drawSuitPip(1, 100, 295, 22)
		cv.drawSuitPip(2, 140, 295, 22)
		cv.drawSuitPip(3, 180, 295, 22)

	case 2: // High Priestess — Sophia, divine wisdom between the pillars
		// Two pillars: light and dark
		cv.filledRect(30, 80, 18, 270, color.RGBA{50, 45, 70, 255})
		cv.filledRect(192, 80, 18, 270, color.RGBA{18, 15, 35, 255})
		cv.filledRect(28, 75, 22, 8, gold)
		cv.filledRect(190, 75, 22, 8, gold)
		// Veil of nous — translucent
		for y := 90; y < 340; y++ {
			a := 0.1 + 0.06*math.Sin(float64(y)*0.04)
			for x := 52; x < 188; x++ {
				cv.blend(x, y, indigo, a)
			}
		}
		// Figure
		cv.filledCircle(120, 140, 20, lerpColor(nous, theOne, 0.3))
		cv.glow(120, 140, 12, theOne)
		// Robes — deep blue wisdom
		for y := 165.0; y < 340; y++ {
			t := (y - 165) / 175
			w := 20 + t*38
			cv.filledRect(120-w, y, w*2, 1, lerpColor(nous, nousDim, t))
		}
		// Crescent at crown
		cv.filledCircle(120, 118, 12, nous)
		cv.filledCircle(126, 115, 12, void)
		// Scroll of forms
		cv.filledRect(105, 305, 30, 18, theOne)
		cv.line(105, 305, 135, 305, 1.5, gold)
		cv.line(105, 323, 135, 323, 1.5, gold)

	case 3: // Empress — World Soul, the generative principle
		// Crown with living jewels
		for i := 0; i < 7; i++ {
			x := 92 + float64(i)*8
			cv.line(x, 52, x, 64, 2, gold)
			cv.filledCircle(x, 49, 3, psyche)
			cv.glow(x, 49, 4, psyche)
		}
		cv.filledRect(88, 64, 64, 5, gold)
		// Head
		cv.filledCircle(120, 80, 22, lerpColor(psyche, theOne, 0.3))
		cv.glow(120, 80, 14, theOne)
		// Flowing robes — the generative overflow
		for y := 105.0; y < 295; y++ {
			t := (y - 105) / 190
			w := 25 + t*70
			col := lerpColor(p, s, t*0.4)
			cv.filledRect(120-w, y, w*2, 1, col)
		}
		// Garden of manifestation below
		for x := 35.0; x < 210; x += 28 {
			cv.glow(x, 330, 8, psyche)
			cv.filledCircle(x, 330, 3, theOne)
			cv.line(x, 335, x, 355, 1.5, physisDim)
		}
		// Stars of emanation
		cv.glow(50, 55, 8, theOne)
		cv.glow(190, 55, 8, theOne)

	case 4: // Emperor — Demiurgic power, cosmic order
		// Crown with authority
		for i := 0; i < 5; i++ {
			x := 100 + float64(i)*10
			h := 18.0
			if i%2 == 1 {
				h = 13
			}
			cv.line(x, 52-h, x, 56, 3, gold)
			cv.filledCircle(x, 52-h-3, 4, dynamis)
			cv.glow(x, 52-h-3, 5, dynamis)
		}
		cv.filledRect(96, 56, 48, 7, gold)
		// Head
		cv.filledCircle(120, 78, 22, lerpColor(p, theOne, 0.3))
		cv.glow(120, 78, 12, theOne)
		// Throne of cosmic structure
		cv.filledRect(35, 55, 14, 280, color.RGBA{30, 22, 55, 255})
		cv.filledRect(191, 55, 14, 280, color.RGBA{30, 22, 55, 255})
		cv.glow(42, 55, 6, gold)
		cv.glow(198, 55, 6, gold)
		// Body
		cv.filledRect(82, 105, 76, 130, p)
		cv.line(82, 105, 158, 105, 2, gold)
		// Orb of emanation
		cv.filledCircle(172, 175, 14, gold)
		cv.glow(172, 175, 12, theOne)
		// Scepter of dynamis
		cv.line(68, 128, 68, 260, 3, gold)
		cv.filledCircle(68, 122, 6, dynamis)
		cv.glow(68, 122, 8, dynamis)
		// Legs
		cv.filledRect(92, 235, 18, 55, color.RGBA{p.R / 2, p.G / 2, p.B / 2, 255})
		cv.filledRect(130, 235, 18, 55, color.RGBA{p.R / 2, p.G / 2, p.B / 2, 255})

	case 5: // Hierophant — the teacher of return, transmitter of truth
		// Columns of ascending wisdom
		cv.filledRect(28, 80, 22, 270, color.RGBA{28, 22, 50, 255})
		cv.filledRect(190, 80, 22, 270, color.RGBA{28, 22, 50, 255})
		cv.filledRect(26, 75, 26, 8, gold)
		cv.filledRect(188, 75, 26, 8, gold)
		// The One above — triple rays
		cv.glow(120, 48, 18, theOne)
		cv.filledCircle(120, 48, 6, theOne)
		cv.line(120, 58, 120, 95, 2, gold)
		cv.line(105, 62, 120, 48, 1.5, gold)
		cv.line(135, 62, 120, 48, 1.5, gold)
		// Figure
		cv.filledCircle(120, 165, 22, lerpColor(p, theOne, 0.3))
		cv.glow(120, 165, 14, theOne)
		// Tiara
		cv.filledRect(105, 140, 30, 10, gold)
		cv.filledRect(110, 132, 20, 10, gold)
		cv.filledCircle(120, 130, 5, theOne)
		// Robes
		cv.filledRect(90, 190, 60, 100, goldDim)
		cv.line(90, 190, 150, 190, 2, gold)
		// Blessing hand — transmitting light
		cv.line(140, 200, 170, 180, 3, theOne)
		cv.glow(170, 178, 8, theOne)
		// Two seekers
		cv.glow(70, 320, 12, psyche)
		cv.filledCircle(70, 320, 10, psycheDim)
		cv.glow(170, 320, 12, psyche)
		cv.filledCircle(170, 320, 10, psycheDim)

	case 6: // Lovers — the soul's choice between ascent and descent
		// The One above — source of love
		cv.glow(120, 60, 22, theOne)
		cv.filledCircle(120, 60, 8, theOne)
		cv.lightRays(120, 60, 6, 25)
		// Two figures in separate light
		// Left — ascent toward nous
		cv.glow(70, 195, 20, nous)
		cv.filledCircle(70, 185, 16, lerpColor(nous, theOne, 0.3))
		cv.filledRect(58, 205, 24, 70, nous)
		// Right — descent toward matter
		cv.glow(170, 195, 20, physis)
		cv.filledCircle(170, 185, 16, lerpColor(physis, theOne, 0.2))
		cv.filledRect(158, 205, 24, 70, physis)
		// Central soul figure
		cv.glow(120, 140, 18, psyche)
		cv.filledCircle(120, 135, 14, lerpColor(psyche, theOne, 0.4))
		// Hands reaching
		cv.line(82, 220, 108, 155, 2, nous)
		cv.line(158, 220, 132, 155, 2, physis)
		// Garden below
		for x := 40.0; x < 210; x += 25 {
			cv.glow(x, 340, 6, psyche)
		}

	case 7: // Chariot — will in motion, the soul's directed journey
		// Stars of the intelligible realm
		for i := 0; i < 6; i++ {
			x := 45 + float64(i)*30
			y := 55 + float64(i%3)*10
			cv.glow(x, y, 7, theOne)
			cv.filledCircle(x, y, 2.5, theOne)
		}
		// Figure — directing emanation
		cv.filledCircle(120, 115, 20, lerpColor(p, theOne, 0.3))
		cv.glow(120, 115, 12, theOne)
		cv.filledRect(104, 138, 32, 50, nous)
		// Chariot body — vessel of the soul
		cv.filledRect(42, 195, 156, 70, color.RGBA{25, 20, 45, 255})
		cv.line(42, 195, 198, 195, 2, gold)
		cv.line(42, 265, 198, 265, 2, gold)
		// Emanation emblem on chariot
		cv.circle(120, 230, 22, 2, gold)
		cv.filledCircle(120, 230, 8, theOne)
		cv.glow(120, 230, 12, theOne)
		// Wheels of becoming
		cv.circle(55, 298, 20, 2.5, gold)
		cv.filledCircle(55, 298, 6, goldDim)
		cv.circle(185, 298, 20, 2.5, gold)
		cv.filledCircle(185, 298, 6, goldDim)
		for i := 0; i < 6; i++ {
			a := float64(i) * math.Pi / 3
			cv.line(55+20*math.Cos(a), 298+20*math.Sin(a), 55, 298, 1.2, goldDim)
			cv.line(185+20*math.Cos(a), 298+20*math.Sin(a), 185, 298, 1.2, goldDim)
		}

	case 8: // Strength — the soul's courage and mastery over the body
		// Emanation from above
		cv.glow(120, 50, 20, theOne)
		cv.lightRays(120, 50, 6, 22)
		// Figure — gentle mastery
		cv.filledCircle(100, 140, 20, lerpColor(gold, theOne, 0.3))
		cv.glow(100, 140, 12, theOne)
		// Robes
		for y := 165.0; y < 280; y++ {
			t := (y - 165) / 115
			w := 15 + t*25
			cv.filledRect(100-w, y, w*2, 1, lerpColor(gold, goldDim, t))
		}
		// Lion — the passionate soul (thumos)
		cv.filledCircle(155, 225, 28, dynamis)
		cv.glow(155, 225, 16, dynamis)
		cv.circle(155, 225, 32, 2, dynamisDim)
		cv.filledRect(135, 255, 42, 30, dynamisDim)
		cv.filledRect(138, 285, 12, 22, dynamisDim)
		cv.filledRect(160, 285, 12, 22, dynamisDim)
		// Hand taming the lion — nous over thumos
		cv.line(100, 162, 140, 218, 2.5, theOne)
		cv.glow(130, 200, 8, theOne)

	case 9: // Hermit — contemplative withdrawal toward The One
		// Mountain of ascent
		for y := 195; y < 360; y++ {
			t := float64(y-195) / 165
			w := t * 95
			col := lerpColor(color.RGBA{35, 28, 55, 255}, voidBot, t*0.5)
			cv.filledRect(120-w, float64(y), w*2, 1, col)
		}
		// Path upward
		cv.bezier(120, 260, 105, 300, 138, 340, 120, 375, 2, goldDim)
		// Figure at summit
		cv.filledCircle(120, 158, 18, lerpColor(theOne, gold, 0.2))
		cv.glow(120, 158, 12, theOne)
		// Hooded cloak — renunciation
		cv.arc(120, 148, 22, math.Pi, 2*math.Pi, 3, color.RGBA{40, 32, 60, 255})
		cv.filledRect(100, 178, 40, 70, color.RGBA{40, 32, 60, 255})
		// Staff
		cv.line(90, 160, 90, 290, 2.5, goldDim)
		// Lantern — the light of nous
		cv.filledRect(152, 142, 14, 18, gold)
		cv.glow(159, 151, 22, theOne)
		cv.glow(159, 151, 12, theOne)
		// Star of The One above
		cv.glow(120, 52, 18, theOne)
		cv.filledCircle(120, 52, 6, theOne)
		cv.lightRays(120, 52, 6, 15)

	case 10: // Wheel of Fortune — cycles of emanation and return
		// Great wheel of emanation
		cv.circle(120, 200, 75, 3, gold)
		cv.circle(120, 200, 65, 2, goldDim)
		cv.circle(120, 200, 55, 1.5, indigo)
		cv.circle(120, 200, 45, 1.5, indigoDim)
		// Central point — The One
		cv.glow(120, 200, 20, theOne)
		cv.filledCircle(120, 200, 8, theOne)
		// Spokes — paths of emanation
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			cv.line(120+15*math.Cos(a), 200+15*math.Sin(a),
				120+72*math.Cos(a), 200+72*math.Sin(a), 1.5, goldDim)
		}
		// Four figures at cardinal points — ascending/descending souls
		positions := [][2]float64{{120, 120}, {200, 200}, {120, 280}, {40, 200}}
		for i, pos := range positions {
			col := []color.RGBA{theOne, nous, physis, psyche}[i]
			cv.glow(pos[0], pos[1], 10, col)
			cv.filledCircle(pos[0], pos[1], 5, col)
		}
		// Corner symbols
		cv.tetractys(38, 55, 30)
		cv.tetractys(195, 55, 30)

	case 11: // Justice — Dike, cosmic order and proportion
		// Columns of law
		cv.filledRect(30, 80, 18, 270, color.RGBA{25, 20, 48, 255})
		cv.filledRect(192, 80, 18, 270, color.RGBA{25, 20, 48, 255})
		cv.filledRect(28, 75, 22, 8, gold)
		cv.filledRect(190, 75, 22, 8, gold)
		// Sword of intellect — vertical
		cv.line(120, 50, 120, 230, 2.5, nous)
		cv.line(106, 88, 134, 88, 2.5, nous)
		cv.glow(120, 55, 10, theOne)
		// Figure
		cv.filledCircle(120, 148, 20, lerpColor(nous, theOne, 0.3))
		cv.glow(120, 148, 12, theOne)
		// Crown of law
		cv.filledRect(105, 125, 30, 5, gold)
		// Robes
		for y := 172.0; y < 300; y++ {
			t := (y - 172) / 128
			w := 20 + t*32
			cv.filledRect(120-w, y, w*2, 1, lerpColor(nous, nousDim, t))
		}
		// Scales — cosmic balance
		cv.line(55, 200, 185, 200, 2.5, gold)
		cv.arc(70, 228, 18, 0, math.Pi, 2, gold)
		cv.arc(170, 228, 18, 0, math.Pi, 2, gold)
		cv.line(70, 200, 70, 228, 1.5, gold)
		cv.line(170, 200, 170, 228, 1.5, gold)

	case 12: // Hanged Man — katharsis, voluntary suspension of ego
		// Tree/beam — the cosmic structure
		cv.filledRect(110, 48, 20, 95, color.RGBA{40, 30, 58, 255})
		cv.line(55, 52, 185, 52, 3, color.RGBA{40, 30, 58, 255})
		// Rope of attachment
		cv.line(120, 140, 120, 180, 1.5, goldDim)
		// Inverted figure — surrender
		cv.line(120, 155, 108, 200, 2.5, psyche)
		cv.line(120, 155, 132, 200, 2.5, psyche)
		cv.line(108, 200, 132, 200, 2, psyche)
		cv.filledRect(110, 200, 20, 50, psycheDim)
		// Arms hanging
		cv.line(110, 225, 92, 255, 2, psycheDim)
		cv.line(130, 225, 148, 255, 2, psycheDim)
		// Head with golden halo — illumination through surrender
		cv.filledCircle(120, 278, 18, lerpColor(psyche, theOne, 0.3))
		cv.circle(120, 278, 28, 2.5, gold)
		cv.glow(120, 278, 32, gold)
		cv.glow(120, 278, 18, theOne)

	case 13: // Death — dissolution of form, return to source
		// Skull — dissolution
		cv.filledCircle(120, 108, 32, theOne)
		cv.filledCircle(108, 103, 9, void)
		cv.filledCircle(132, 103, 9, void)
		cv.filledCircle(120, 115, 5, void)
		cv.filledRect(108, 125, 24, 4, void)
		for x := 108.0; x < 132; x += 6 {
			cv.line(x, 125, x, 129, 1, theOne)
		}
		cv.glow(120, 108, 20, theOne)
		// Scythe — cutting ties to matter
		cv.line(175, 88, 78, 340, 2.5, goldDim)
		cv.arc(140, 98, 42, math.Pi+0.3, 2*math.Pi-0.3, 2.5, nous)
		cv.glow(140, 68, 10, theOne)
		// Skeletal form
		cv.line(120, 140, 120, 245, 2, theOne)
		for i := 0; i < 4; i++ {
			y := 168 + float64(i)*18
			cv.arc(120, y, 16, 0, math.Pi, 1.5, theOne)
		}
		cv.line(120, 245, 102, 308, 2, theOne)
		cv.line(120, 245, 138, 308, 2, theOne)
		// Rose of rebirth — the seed of return
		cv.glow(120, 348, 15, psyche)
		cv.filledCircle(120, 348, 6, psyche)
		cv.circle(120, 348, 10, 1.5, gold)

	case 14: // Temperance — harmonizing the soul's faculties
		// Wings — angelic messenger between hypostases
		cv.bezier(90, 108, 38, 68, 18, 58, 28, 98, 3, gold)
		cv.bezier(150, 108, 202, 68, 222, 58, 212, 98, 3, gold)
		for i := 0; i < 4; i++ {
			t := float64(i) * 0.15
			cv.bezier(90, 108, 38+t*30, 68+t*15, 18+t*20, 58+t*18, 28+t*15, 98, 1.2, goldDim)
			cv.bezier(150, 108, 202-t*30, 68+t*15, 222-t*20, 58+t*18, 212-t*15, 98, 1.2, goldDim)
		}
		// Halo
		cv.circle(120, 75, 16, 2, gold)
		cv.glow(120, 75, 10, theOne)
		// Head
		cv.filledCircle(120, 98, 20, lerpColor(nous, theOne, 0.3))
		cv.glow(120, 98, 12, theOne)
		// Robes
		for y := 122.0; y < 300; y++ {
			t := (y - 122) / 178
			w := 20 + t*42
			cv.filledRect(120-w, y, w*2, 1, lerpColor(nous, nousDim, t*0.4))
		}
		// Two vessels — mixing nous and psyche
		cv.filledRect(58, 185, 22, 32, nous)
		cv.line(58, 185, 80, 185, 1.5, gold)
		cv.filledRect(160, 218, 22, 32, psyche)
		cv.line(160, 218, 182, 218, 1.5, gold)
		// Flowing stream between — the harmonized soul
		cv.bezier(78, 200, 105, 195, 140, 215, 162, 225, 2.5, lerpColor(nous, psyche, 0.5))
		cv.glow(120, 210, 12, theOne)

	case 15: // Devil — soul trapped in matter, forgetting The One
		// Horns — inverted emanation
		cv.bezier(102, 88, 88, 48, 72, 38, 62, 52, 3, physis)
		cv.bezier(138, 88, 152, 48, 168, 38, 178, 52, 3, physis)
		// Dark head
		cv.filledCircle(120, 102, 26, color.RGBA{physis.R / 2, physis.G / 2, physis.B / 2, 255})
		cv.glow(120, 102, 15, physis)
		// Glowing eyes — false light
		cv.glow(112, 98, 6, dynamis)
		cv.glow(128, 98, 6, dynamis)
		cv.filledCircle(112, 98, 2.5, theOne)
		cv.filledCircle(128, 98, 2.5, theOne)
		// Body in darkness
		cv.filledRect(92, 130, 56, 95, color.RGBA{physis.R / 3, physis.G / 3, physis.B / 3, 255})
		// Inverted triangle — matter over form
		for i := 0; i < 3; i++ {
			a1 := float64(i)*2*math.Pi/3 + math.Pi/6
			a2 := float64(i+1)*2*math.Pi/3 + math.Pi/6
			cv.line(120+25*math.Cos(a1), 175+25*math.Sin(a1),
				120+25*math.Cos(a2), 175+25*math.Sin(a2), 2, physis)
		}
		// Chains — attachment to matter
		for y := 235.0; y < 310; y += 7 {
			cv.filledCircle(82, y, 2.5, goldDim)
			cv.filledCircle(158, y, 2.5, goldDim)
		}
		// Captive souls
		cv.glow(62, 328, 10, psycheDim)
		cv.filledCircle(62, 328, 8, psycheDim)
		cv.glow(178, 328, 10, psycheDim)
		cv.filledCircle(178, 328, 8, psycheDim)

	case 16: // Tower — shattering of material illusion
		// Tower of matter
		cv.filledRect(88, 88, 64, 248, color.RGBA{40, 32, 58, 255})
		cv.line(88, 88, 152, 88, 2, physis)
		cv.line(88, 88, 88, 336, 2, physis)
		cv.line(152, 88, 152, 336, 2, physis)
		// Crown shattering
		cv.filledRect(62, 58, 14, 10, physisDim)
		cv.filledRect(168, 68, 10, 8, physisDim)
		// Windows
		for y := 128.0; y < 300; y += 50 {
			cv.glow(112, y+8, 5, goldDim)
			cv.glow(132, y+22, 5, goldDim)
		}
		// Lightning of illumination — The One breaking through
		cv.line(148, 48, 135, 58, 4, theOne)
		cv.line(135, 58, 152, 72, 4, theOne)
		cv.line(152, 72, 125, 88, 5, theOne)
		cv.glow(140, 62, 18, theOne)
		cv.glow(132, 82, 22, theOne)
		// Explosion of light at top
		cv.glow(120, 88, 28, gold)
		// Falling figures — souls freed from matter
		cv.glow(52, 208, 10, psyche)
		cv.filledCircle(52, 208, 8, psyche)
		cv.glow(192, 238, 10, psyche)
		cv.filledCircle(192, 238, 8, psyche)

	case 17: // Star — glimpse of The One, hope of return
		// Great star — The One glimpsed
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			for r := 0.0; r < 42; r++ {
				px := 120 + r*math.Cos(a)
				py := 98 + r*math.Sin(a)
				cv.blend(int(px), int(py), theOne, 0.7-r/65)
			}
		}
		cv.glow(120, 98, 28, theOne)
		cv.filledCircle(120, 98, 10, theOne)
		// Seven smaller stars — the seven hypostases/spheres
		starPos := [][2]float64{{52, 58}, {188, 52}, {38, 128}, {202, 112}, {62, 178}, {182, 168}, {48, 68}}
		for _, sp := range starPos {
			cv.glow(sp[0], sp[1], 7, nous)
			cv.filledCircle(sp[0], sp[1], 2.5, theOne)
		}
		// Water bearer — pouring light back to the world
		cv.filledCircle(120, 238, 16, lerpColor(psyche, theOne, 0.3))
		cv.glow(120, 238, 10, theOne)
		cv.filledRect(108, 258, 24, 48, lerpColor(psyche, theOne, 0.15))
		// Streams of light descending
		cv.bezier(98, 272, 72, 298, 62, 332, 52, 358, 2.5, nous)
		cv.bezier(142, 272, 168, 298, 178, 332, 188, 358, 2.5, psyche)
		cv.glow(68, 342, 10, nous)
		cv.glow(172, 342, 10, psyche)

	case 18: // Moon — soul in confusion, the realm of phantasia
		// Crescent moon — reflected light, not The One directly
		cv.filledCircle(120, 88, 38, psyche)
		cv.filledCircle(136, 80, 38, void)
		cv.glow(108, 92, 32, psyche)
		// Dim rays — confused light
		for i := 0; i < 8; i++ {
			x := 72 + float64(i)*18
			cv.line(x, 138, x+3, 155, 1.2, psycheDim)
		}
		// Two towers of confusion
		cv.filledRect(30, 188, 28, 142, color.RGBA{28, 22, 48, 255})
		cv.filledRect(182, 188, 28, 142, color.RGBA{28, 22, 48, 255})
		// Dim window glows
		cv.glow(44, 228, 6, psycheDim)
		cv.glow(196, 228, 6, psycheDim)
		// Path of illusion — winding
		cv.bezier(120, 358, 102, 332, 138, 302, 120, 272, 2.5, goldDim)
		cv.bezier(120, 272, 102, 252, 112, 228, 120, 192, 2, goldDim)
		// Wolves of unreason
		cv.filledCircle(70, 338, 10, color.RGBA{55, 42, 65, 255})
		cv.filledRect(60, 345, 20, 10, color.RGBA{48, 38, 58, 255})
		cv.filledCircle(170, 338, 10, color.RGBA{55, 42, 65, 255})
		cv.filledRect(160, 345, 20, 10, color.RGBA{48, 38, 58, 255})

	case 19: // Sun — intellectual illumination, Nous fully radiant
		// Great sun — Nous illuminated by The One
		cv.filledCircle(120, 108, 45, gold)
		cv.glow(120, 108, 40, gold)
		cv.glow(120, 108, 22, theOne)
		// Sun face
		cv.filledCircle(110, 103, 4, color.RGBA{goldDim.R / 2, goldDim.G / 2, goldDim.B / 2, 255})
		cv.filledCircle(130, 103, 4, color.RGBA{goldDim.R / 2, goldDim.G / 2, goldDim.B / 2, 255})
		cv.arc(120, 115, 14, 0.2, math.Pi-0.2, 2, color.RGBA{goldDim.R / 2, goldDim.G / 2, goldDim.B / 2, 255})
		// Rays of emanation
		for i := 0; i < 16; i++ {
			a := float64(i) * math.Pi / 8
			r1 := 50.0
			r2 := 75.0
			if i%2 == 0 {
				r2 = 85
			}
			cv.line(120+r1*math.Cos(a), 108+r1*math.Sin(a),
				120+r2*math.Cos(a), 108+r2*math.Sin(a), 2.5, gold)
		}
		// Child soul — innocent and illuminated
		cv.filledCircle(120, 268, 14, lerpColor(theOne, gold, 0.2))
		cv.glow(120, 268, 8, theOne)
		cv.filledRect(112, 285, 16, 32, theOne)
		// Arms raised toward sun
		cv.line(112, 292, 96, 272, 2, theOne)
		cv.line(128, 292, 144, 272, 2, theOne)
		// Garden of the intelligible
		for x := 38.0; x < 210; x += 42 {
			cv.glow(x, 350, 6, gold)
			cv.filledCircle(x, 350, 2, theOne)
		}

	case 20: // Judgement — the call to ascend, epistrophe
		// Angel/herald of The One
		cv.filledCircle(120, 68, 18, lerpColor(gold, theOne, 0.4))
		cv.glow(120, 68, 12, theOne)
		cv.circle(120, 52, 20, 2, gold)
		cv.glow(120, 52, 14, gold)
		// Wings
		cv.bezier(102, 78, 52, 42, 25, 52, 30, 88, 2.5, gold)
		cv.bezier(138, 78, 188, 42, 215, 52, 210, 88, 2.5, gold)
		for i := 0; i < 4; i++ {
			t := float64(i) * 0.15
			cv.bezier(102, 78, 52+t*38, 42+t*18, 25+t*28, 52+t*18, 30+t*18, 88, 1.2, goldDim)
			cv.bezier(138, 78, 188-t*38, 42+t*18, 215-t*28, 52+t*18, 210-t*18, 88, 1.2, goldDim)
		}
		// Trumpet — the call
		cv.line(120, 92, 182, 112, 2.5, gold)
		cv.filledCircle(188, 114, 7, gold)
		cv.glow(188, 114, 10, theOne)
		// Ascending light beam
		cv.line(120, 105, 120, 140, 3, theOne)
		cv.glow(120, 122, 8, theOne)
		// Rising souls — three figures ascending
		riseFigs := []float64{62, 120, 178}
		for _, fx := range riseFigs {
			cv.glow(fx, 262, 12, psyche)
			cv.filledCircle(fx, 258, 12, lerpColor(psyche, theOne, 0.3))
			cv.filledRect(fx-8, 275, 16, 30, psycheDim)
			// Arms reaching up
			cv.line(fx-8, 282, fx-14, 268, 2, theOne)
			cv.line(fx+8, 282, fx+14, 268, 2, theOne)
		}
		// Glow of ascent
		cv.glow(120, 290, 38, gold)

	case 21: // World — henosis, the return to The One
		// Great circle — the completed journey
		cv.circle(120, 200, 82, 3.5, gold)
		cv.circle(120, 200, 76, 2, goldDim)
		// Fill ring with emanation gradients
		for i := 0; i < 12; i++ {
			a1 := float64(i) * 2 * math.Pi / 12
			a2 := float64(i+1) * 2 * math.Pi / 12
			cols := []color.RGBA{gold, nous, psyche, dynamis, physis, gold,
				nous, psyche, dynamis, physis, gold, nous}
			col := cols[i]
			for r := 66.0; r < 76; r++ {
				for a := a1 + 0.02; a < a2-0.02; a += 0.02 {
					px := 120 + r*math.Cos(a)
					py := 200 + r*math.Sin(a)
					bright := 0.4 + 0.3*math.Sin((r-66)/10*math.Pi)
					gc := color.RGBA{
						clampU8(float64(col.R) * bright),
						clampU8(float64(col.G) * bright),
						clampU8(float64(col.B) * bright),
						255,
					}
					cv.blend(int(px), int(py), gc, 0.7)
				}
			}
		}
		// Dancing figure inside — the reunited soul
		cv.filledCircle(120, 170, 16, lerpColor(theOne, gold, 0.2))
		cv.glow(120, 170, 10, theOne)
		cv.bezier(108, 185, 98, 218, 102, 252, 112, 268, 2, gold)
		cv.bezier(132, 185, 142, 218, 138, 252, 128, 268, 2, gold)
		cv.filledRect(110, 190, 20, 40, lerpColor(gold, theOne, 0.3))
		// Arms
		cv.line(108, 200, 82, 182, 2, theOne)
		cv.line(132, 200, 158, 182, 2, theOne)
		// Legs
		cv.line(115, 232, 100, 262, 2, theOne)
		cv.line(125, 232, 140, 262, 2, theOne)
		// The One at center — henosis achieved
		cv.glow(120, 200, 30, theOne)
		// Four corner emanation symbols
		corners := [][2]float64{{32, 52}, {208, 52}, {32, 355}, {208, 355}}
		cornerCols := []color.RGBA{nous, dynamis, psyche, physis}
		for i, pos := range corners {
			cv.glow(pos[0], pos[1], 8, cornerCols[i])
			cv.filledCircle(pos[0], pos[1], 4, theOne)
		}
	}

	// Decorative: small tetractys below the number
	if num <= 9 {
		cv.tetractys(190, 52, 20)
	}

	return cv
}

// ========== main ==========

func main() {
	base := "decks/plotinus"

	// Card back
	fmt.Print("Generating card back...")
	if err := genBack().writeHexFile(filepath.Join(base, "back.hex")); err != nil {
		fmt.Fprintf(os.Stderr, " error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(" done")

	// Major Arcana
	for i := 0; i <= 21; i++ {
		slug := majorSlugs[i]
		path := filepath.Join(base, "major", fmt.Sprintf("%02d-%s.hex", i, slug))
		fmt.Printf("Major %02d %-25s", i, slug)
		if err := genMajor(i).writeHexFile(path); err != nil {
			fmt.Fprintf(os.Stderr, " error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(" done")
	}

	// Minor Arcana
	for si := 0; si < 4; si++ {
		suitName := suitFileNames[si]
		// Pip cards 1-10
		for num := 1; num <= 10; num++ {
			path := filepath.Join(base, "minor", fmt.Sprintf("%s-%02d.hex", suitName, num))
			fmt.Printf("Minor %-10s %02d", suitName, num)
			if err := genPip(si, num).writeHexFile(path); err != nil {
				fmt.Fprintf(os.Stderr, " error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(" done")
		}
		// Court cards 11-14
		for rank := 11; rank <= 14; rank++ {
			path := filepath.Join(base, "minor", fmt.Sprintf("%s-%02d.hex", suitName, rank))
			rankName := []string{"Page", "Knight", "Queen", "King"}[rank-11]
			fmt.Printf("Minor %-10s %s", suitName, rankName)
			if err := genCourt(si, rank).writeHexFile(path); err != nil {
				fmt.Fprintf(os.Stderr, " error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(" done")
		}
	}

	fmt.Printf("\nGenerated 79 cards + back in %s/\n", base)
}
