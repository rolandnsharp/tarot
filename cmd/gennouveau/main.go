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

func rgba(r, g, b uint8, a float64) color.RGBA {
	return color.RGBA{r, g, b, uint8(clamp(a * 255))}
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
		clampU8(float64(a.A)*(1-t) + float64(b.A)*t),
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
		col.A = 255
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

func (c *HiResCanvas) ellipse(cx, cy, rx, ry, thickness float64, col color.RGBA) {
	irx := int(rx+thickness) + 2
	iry := int(ry+thickness) + 2
	for y := int(cy) - iry; y <= int(cy)+iry; y++ {
		for x := int(cx) - irx; x <= int(cx)+irx; x++ {
			dx := (float64(x) - cx) / rx
			dy := (float64(y) - cy) / ry
			d := math.Sqrt(dx*dx+dy*dy) * math.Min(rx, ry)
			ref := math.Min(rx, ry)
			dist := math.Abs(d - ref)
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

// glow draws a soft Gaussian glow
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

// --- Art Nouveau decorative elements ---

func (c *HiResCanvas) vine(x1, y1, x2, y2 float64, col color.RGBA) {
	// Sinusoidal vine with leaves
	dx := x2 - x1
	dy := y2 - y1
	length := math.Sqrt(dx*dx + dy*dy)
	steps := int(length)
	nx := -dy / length
	ny := dx / length
	amplitude := length * 0.15
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		wave := math.Sin(t*math.Pi*4) * amplitude
		px := x1 + dx*t + nx*wave
		py := y1 + dy*t + ny*wave
		c.filledCircle(px, py, 1.5, col)
		// Small leaves at wave peaks
		if i%int(math.Max(float64(steps)/8, 1)) == 0 && i > 0 && i < steps {
			leafCol := color.RGBA{
				clampU8(float64(col.R) * 0.7),
				clampU8(float64(col.G) * 0.7),
				clampU8(float64(col.B) * 0.7),
				255,
			}
			c.filledCircle(px+nx*4, py+ny*4, 3, leafCol)
			c.filledCircle(px-nx*4, py-ny*4, 3, leafCol)
		}
	}
}

func (c *HiResCanvas) floral(cx, cy, size float64, col color.RGBA) {
	// 5-petal flower
	petals := 5
	for i := 0; i < petals; i++ {
		a := float64(i) * 2 * math.Pi / float64(petals)
		px := cx + size*0.6*math.Cos(a)
		py := cy + size*0.6*math.Sin(a)
		c.filledCircle(px, py, size*0.4, col)
	}
	// Center
	center := color.RGBA{
		clampU8(float64(col.R) * 1.3),
		clampU8(float64(col.G) * 1.2),
		clampU8(float64(col.B) * 0.8),
		255,
	}
	c.filledCircle(cx, cy, size*0.25, center)
}

func (c *HiResCanvas) ornamentBorder(margin, thickness float64, col color.RGBA) {
	w := float64(hiW)
	h := float64(hiH)
	m := margin
	// Outer rectangle with rounded-ish effect
	c.line(m, m, w-m, m, thickness, col)
	c.line(w-m, m, w-m, h-m, thickness, col)
	c.line(w-m, h-m, m, h-m, thickness, col)
	c.line(m, h-m, m, m, thickness, col)
	// Inner rectangle
	m2 := margin + 8
	c.line(m2, m2, w-m2, m2, thickness*0.6, col)
	c.line(w-m2, m2, w-m2, h-m2, thickness*0.6, col)
	c.line(w-m2, h-m2, m2, h-m2, thickness*0.6, col)
	c.line(m2, h-m2, m2, m2, thickness*0.6, col)
	// Corner flourishes
	cornerSize := 20.0
	// Top-left
	c.arc(m+cornerSize, m+cornerSize, cornerSize, math.Pi, 1.5*math.Pi, thickness*0.8, col)
	// Top-right
	c.arc(w-m-cornerSize, m+cornerSize, cornerSize, 1.5*math.Pi, 2*math.Pi, thickness*0.8, col)
	// Bottom-left
	c.arc(m+cornerSize, h-m-cornerSize, cornerSize, 0.5*math.Pi, math.Pi, thickness*0.8, col)
	// Bottom-right
	c.arc(w-m-cornerSize, h-m-cornerSize, cornerSize, 0, 0.5*math.Pi, thickness*0.8, col)
}

func (c *HiResCanvas) archeFrame(col color.RGBA) {
	// Art Nouveau arch frame around the card
	w := float64(hiW)
	h := float64(hiH)
	m := 15.0
	th := 3.0
	// Sides
	c.line(m, h*0.2, m, h-m, th, col)
	c.line(w-m, h*0.2, w-m, h-m, th, col)
	// Bottom
	c.line(m, h-m, w-m, h-m, th, col)
	// Arch top
	c.arc(w/2, h*0.2, w/2-m, math.Pi, 2*math.Pi, th, col)
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
	pixels := c.downscale()
	return writeHex(pixels, path)
}

// ========== color palette ==========

var (
	bgDark    = color.RGBA{25, 10, 8, 255}
	bgMid     = color.RGBA{45, 20, 15, 255}
	gold      = color.RGBA{218, 165, 32, 255}
	amber     = color.RGBA{255, 191, 0, 255}
	cream     = color.RGBA{245, 230, 200, 255}
	copper    = color.RGBA{184, 115, 51, 255}
	warmWhite = color.RGBA{255, 240, 220, 255}
	burgundy  = color.RGBA{80, 20, 20, 255}
	deepRose  = color.RGBA{160, 50, 60, 255}
	dimGold   = color.RGBA{109, 83, 16, 255}
	dimCopper = color.RGBA{92, 58, 26, 255}
)

// suit colors
var (
	cupsRose     = color.RGBA{200, 80, 90, 255}
	cupsRoseDim  = color.RGBA{120, 40, 50, 255}
	wandsAmber   = color.RGBA{230, 160, 40, 255}
	wandsAmberD  = color.RGBA{140, 90, 20, 255}
	swordsSilver = color.RGBA{160, 180, 210, 255}
	swordsSilvD  = color.RGBA{80, 100, 130, 255}
	pentGold     = color.RGBA{220, 180, 50, 255}
	pentGoldDim  = color.RGBA{120, 95, 25, 255}
)

type suitColors struct {
	primary, dim color.RGBA
}

var suitPalette = []suitColors{
	{cupsRose, cupsRoseDim},       // cups (index 0)
	{wandsAmber, wandsAmberD},     // wands (index 1)
	{swordsSilver, swordsSilvD},   // swords (index 2)
	{pentGold, pentGoldDim},       // pentacles (index 3)
}

var suitFileNames = []string{"cups", "wands", "swords", "pentacles"}

// ========== nouveau background ==========

func nouveauBG() *HiResCanvas {
	cv := newCanvas()
	cv.fillGradient(bgMid, bgDark)
	return cv
}

// subtle background vine pattern
func (c *HiResCanvas) bgVines(col color.RGBA) {
	dimCol := color.RGBA{col.R / 3, col.G / 3, col.B / 3, 255}
	// Vertical vine pairs
	c.bezier(30, 0, 20, 100, 40, 200, 25, 400, 1.0, dimCol)
	c.bezier(210, 0, 220, 100, 200, 200, 215, 400, 1.0, dimCol)
	// Cross vine
	c.bezier(0, 200, 80, 180, 160, 220, 240, 200, 0.8, dimCol)
}

// ========== suit pip bitmaps (hi-res: ~30px) ==========

func (c *HiResCanvas) drawHeart(cx, cy, size float64, col color.RGBA) {
	// Heart shape from two circles + triangle
	r := size * 0.35
	c.filledCircle(cx-r, cy-r*0.4, r, col)
	c.filledCircle(cx+r, cy-r*0.4, r, col)
	// Lower triangle
	for y := cy; y < cy+size*0.7; y++ {
		t := (y - cy) / (size * 0.7)
		w := size * 0.7 * (1 - t)
		for x := cx - w; x <= cx+w; x++ {
			c.set(int(x), int(y), col)
		}
	}
}

func (c *HiResCanvas) drawCup(cx, cy, size float64, col color.RGBA) {
	// Chalice / cup shape
	// Bowl (arc)
	c.arc(cx, cy-size*0.1, size*0.4, 0, math.Pi, 2.0, col)
	c.filledCircle(cx, cy-size*0.1, size*0.3, col)
	// Stem
	c.line(cx, cy+size*0.2, cx, cy+size*0.5, 2.0, col)
	// Base
	c.line(cx-size*0.25, cy+size*0.5, cx+size*0.25, cy+size*0.5, 2.0, col)
}

func (c *HiResCanvas) drawWand(cx, cy, size float64, col color.RGBA) {
	// Vertical rod with flame/bud top
	c.line(cx, cy-size*0.5, cx, cy+size*0.5, 2.0, col)
	// Flame on top
	c.filledCircle(cx, cy-size*0.55, size*0.15, amber)
	c.glow(cx, cy-size*0.55, size*0.2, amber)
	// Leaf accents
	c.filledCircle(cx-size*0.15, cy-size*0.1, size*0.08, col)
	c.filledCircle(cx+size*0.15, cy+size*0.1, size*0.08, col)
}

func (c *HiResCanvas) drawSword(cx, cy, size float64, col color.RGBA) {
	// Vertical blade
	c.line(cx, cy-size*0.55, cx, cy+size*0.4, 2.0, col)
	// Cross-guard
	c.line(cx-size*0.25, cy-size*0.1, cx+size*0.25, cy-size*0.1, 2.0, col)
	// Pommel
	c.filledCircle(cx, cy+size*0.45, size*0.08, col)
	// Blade tip glow
	c.glow(cx, cy-size*0.55, size*0.1, warmWhite)
}

func (c *HiResCanvas) drawPentacle(cx, cy, size float64, col color.RGBA) {
	// Circle with 5-pointed star
	c.circle(cx, cy, size*0.4, 2.0, col)
	// Star points
	for i := 0; i < 5; i++ {
		a := float64(i)*2*math.Pi/5 - math.Pi/2
		a2 := float64(i+2)*2*math.Pi/5 - math.Pi/2
		x1 := cx + size*0.38*math.Cos(a)
		y1 := cy + size*0.38*math.Sin(a)
		x2 := cx + size*0.38*math.Cos(a2)
		y2 := cy + size*0.38*math.Sin(a2)
		c.line(x1, y1, x2, y2, 1.5, col)
	}
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
	ps := scale // pixel size
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
	cv.fillGradient(burgundy, bgDark)

	// Ornamental border
	cv.ornamentBorder(8, 3.0, gold)

	// Central medallion
	cv.circle(120, 200, 60, 2.5, gold)
	cv.circle(120, 200, 45, 2.0, amber)
	cv.circle(120, 200, 30, 1.5, copper)
	cv.filledCircle(120, 200, 15, gold)
	cv.glow(120, 200, 25, gold)

	// Radiating curves from medallion
	for i := 0; i < 8; i++ {
		a := float64(i) * math.Pi / 4
		x1 := 120 + 60*math.Cos(a)
		y1 := 200 + 60*math.Sin(a)
		x2 := 120 + 100*math.Cos(a)
		y2 := 200 + 100*math.Sin(a)
		// Control points offset perpendicular
		cpx := 120 + 80*math.Cos(a+0.3)
		cpy := 200 + 80*math.Sin(a+0.3)
		cv.bezier(x1, y1, cpx, cpy, cpx, cpy, x2, y2, 2.0, dimGold)
	}

	// Floral motifs in corners
	cv.floral(40, 40, 15, dimGold)
	cv.floral(200, 40, 15, dimGold)
	cv.floral(40, 360, 15, dimGold)
	cv.floral(200, 360, 15, dimGold)

	// Vine patterns along borders
	cv.vine(15, 80, 15, 320, dimCopper)
	cv.vine(225, 80, 225, 320, dimCopper)
	cv.vine(40, 15, 200, 15, dimCopper)
	cv.vine(40, 385, 200, 385, dimCopper)

	// Repeating small floral along top and bottom
	for x := 40.0; x <= 200; x += 40 {
		cv.floral(x, 30, 8, dimGold)
		cv.floral(x, 370, 8, dimGold)
	}

	return cv
}

func genPip(si, num int) *HiResCanvas {
	cv := nouveauBG()
	sc := suitPalette[si]
	cv.bgVines(sc.dim)

	// Ornamental border (lighter)
	cv.ornamentBorder(6, 2.0, dimGold)

	// Corner numbers
	cv.drawNum(15, 15, num, sc.primary, 5)
	cv.drawNum(185, 350, num, sc.primary, 5)

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

func genCourt(si, rank int) *HiResCanvas {
	cv := nouveauBG()
	sc := suitPalette[si]
	cv.bgVines(sc.dim)

	// Ornate arch frame
	cv.archeFrame(gold)
	cv.ornamentBorder(5, 2.0, dimGold)

	p := sc.primary
	skin := color.RGBA{210, 175, 140, 255}

	switch rank {
	case 11: // Page — young figure
		// Head
		cv.filledCircle(120, 100, 25, skin)
		cv.glow(120, 100, 15, warmWhite)
		// Hair
		cv.filledCircle(120, 88, 20, color.RGBA{120, 70, 30, 255})
		// Body
		cv.filledRect(100, 130, 40, 100, p)
		// Cloak edges
		cv.line(100, 130, 90, 230, 3, sc.dim)
		cv.line(140, 130, 150, 230, 3, sc.dim)
		// Legs
		cv.filledRect(105, 230, 15, 60, sc.dim)
		cv.filledRect(125, 230, 15, 60, sc.dim)
		// Suit symbol in hand
		cv.drawSuitPip(si, 120, 180, 28)
		// Floral frame
		cv.floral(40, 100, 12, dimGold)
		cv.floral(200, 100, 12, dimGold)

	case 12: // Knight — figure with weapon
		// Head
		cv.filledCircle(100, 90, 25, skin)
		cv.glow(100, 90, 15, warmWhite)
		// Helmet
		cv.arc(100, 80, 25, math.Pi, 2*math.Pi, 3, p)
		// Body/armor
		cv.filledRect(80, 120, 40, 100, p)
		// Extended arm with weapon
		cv.line(120, 150, 200, 130, 3, sc.primary)
		cv.line(200, 100, 200, 160, 3, sc.primary)
		cv.glow(200, 110, 8, warmWhite)
		// Legs
		cv.filledRect(85, 220, 15, 70, sc.dim)
		cv.filledRect(105, 220, 15, 70, sc.dim)
		// Suit symbol
		cv.drawSuitPip(si, 180, 260, 28)
		// Shield shape
		cv.arc(60, 200, 25, 0.5*math.Pi, 1.5*math.Pi, 2, p)

	case 13: // Queen — crowned, flowing robes
		// Crown
		for i := 0; i < 5; i++ {
			x := 100 + float64(i)*10
			cv.line(x, 55, x, 65, 2, gold)
			cv.filledCircle(x, 52, 4, amber)
		}
		cv.filledRect(98, 65, 44, 6, gold)
		// Head
		cv.filledCircle(120, 82, 22, skin)
		cv.glow(120, 82, 12, warmWhite)
		// Flowing robes (widening)
		for y := 110.0; y < 320; y += 1 {
			t := (y - 110) / 210
			w := 30 + t*60
			col := lerpColor(p, sc.dim, t*0.5)
			cv.filledRect(120-w, y, w*2, 1, col)
		}
		// Robe trim
		cv.bezier(60, 320, 80, 280, 160, 280, 180, 320, 2, gold)
		// Suit symbol
		cv.drawSuitPip(si, 120, 200, 30)
		// Floral accents
		cv.floral(50, 180, 10, dimGold)
		cv.floral(190, 180, 10, dimGold)

	case 14: // King — broad, crowned, scepter
		// Crown (larger, more ornate)
		for i := 0; i < 7; i++ {
			x := 92 + float64(i)*8
			h := 12.0
			if i%2 == 0 {
				h = 18
			}
			cv.line(x, 60-h, x, 60, 2.5, gold)
			cv.filledCircle(x, 60-h-3, 4, amber)
		}
		cv.filledRect(88, 60, 64, 8, gold)
		// Head
		cv.filledCircle(120, 82, 24, skin)
		cv.glow(120, 82, 14, warmWhite)
		// Beard
		cv.filledCircle(120, 100, 15, color.RGBA{100, 60, 30, 255})
		// Broad body / robes
		cv.filledRect(80, 115, 80, 120, p)
		cv.filledRect(75, 115, 90, 8, gold) // collar
		// Scepter
		cv.line(175, 100, 175, 280, 3, gold)
		cv.filledCircle(175, 95, 8, amber)
		cv.glow(175, 95, 12, gold)
		// Legs
		cv.filledRect(90, 235, 20, 60, sc.dim)
		cv.filledRect(130, 235, 20, 60, sc.dim)
		// Suit symbol on chest
		cv.drawSuitPip(si, 120, 170, 30)
		// Throne hints
		cv.line(60, 110, 60, 280, 3, dimGold)
		cv.line(180, 110, 180, 280, 3, dimGold)
		cv.arc(120, 110, 60, math.Pi, 2*math.Pi, 2, dimGold)
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
	{warmWhite, gold},                                         // 0  Fool
	{gold, color.RGBA{180, 80, 200, 255}},                    // 1  Magician
	{color.RGBA{150, 170, 220, 255}, cream},                  // 2  High Priestess
	{color.RGBA{80, 180, 80, 255}, gold},                     // 3  Empress
	{deepRose, gold},                                          // 4  Emperor
	{gold, warmWhite},                                         // 5  Hierophant
	{color.RGBA{220, 100, 120, 255}, warmWhite},               // 6  Lovers
	{color.RGBA{120, 150, 220, 255}, gold},                    // 7  Chariot
	{amber, gold},                                             // 8  Strength
	{cream, dimGold},                                          // 9  Hermit
	{gold, color.RGBA{180, 80, 200, 255}},                    // 10 Wheel
	{gold, color.RGBA{120, 150, 220, 255}},                    // 11 Justice
	{color.RGBA{80, 180, 180, 255}, color.RGBA{180, 80, 200, 255}}, // 12 Hanged Man
	{warmWhite, deepRose},                                     // 13 Death
	{color.RGBA{120, 160, 220, 255}, gold},                    // 14 Temperance
	{deepRose, amber},                                         // 15 Devil
	{amber, deepRose},                                         // 16 Tower
	{cream, color.RGBA{120, 160, 220, 255}},                   // 17 Star
	{color.RGBA{200, 200, 230, 255}, color.RGBA{180, 80, 200, 255}}, // 18 Moon
	{gold, amber},                                             // 19 Sun
	{warmWhite, gold},                                         // 20 Judgement
	{color.RGBA{80, 180, 80, 255}, gold},                     // 21 World
}

func genMajor(num int) *HiResCanvas {
	cv := nouveauBG()
	sc := majorSchemes[num]
	p, s := sc.primary, sc.secondary

	// Dim primary for background
	pd := color.RGBA{p.R / 4, p.G / 4, p.B / 4, 255}

	cv.bgVines(pd)

	// Number in corner
	cv.drawNum(15, 15, num, gold, 5)

	// Arch frame
	cv.archeFrame(dimGold)

	skin := color.RGBA{210, 175, 140, 255}

	switch num {
	case 0: // Fool — figure at cliff edge, swirling path, roses
		// Cliff edge (organic curve)
		cv.bezier(0, 270, 60, 260, 120, 280, 160, 270, 3, copper)
		for y := 270; y < 400; y++ {
			t := float64(y-270) / 130
			col := lerpColor(copper, bgDark, t)
			for x := 0; x < 160; x++ {
				cv.blend(x, y, col, 0.6)
			}
		}
		// Figure
		cv.filledCircle(100, 180, 22, skin)
		cv.glow(100, 180, 12, warmWhite)
		cv.filledRect(85, 205, 30, 70, p)
		// Flowing cloak
		cv.bezier(85, 210, 60, 240, 50, 280, 70, 300, 2.5, s)
		cv.bezier(115, 210, 130, 240, 140, 260, 130, 280, 2.5, s)
		// Legs
		cv.filledRect(90, 275, 12, 40, color.RGBA{p.R / 2, p.G / 2, p.B / 2, 255})
		cv.filledRect(105, 275, 12, 40, color.RGBA{p.R / 2, p.G / 2, p.B / 2, 255})
		// Rose in hand
		cv.floral(130, 220, 10, deepRose)
		// Star above
		cv.glow(180, 60, 15, s)
		cv.filledCircle(180, 60, 5, warmWhite)
		// Swirling void below cliff
		cv.arc(200, 350, 40, 0, 2*math.Pi, 2, dimGold)
		cv.glow(200, 350, 20, dimGold)
		// Rose at feet
		cv.floral(80, 310, 8, deepRose)

	case 1: // Magician — infinity symbol, table with elements, ornate arch
		// Ornate arch above
		cv.arc(120, 100, 80, math.Pi, 2*math.Pi, 3, gold)
		cv.arc(120, 100, 70, math.Pi, 2*math.Pi, 2, dimGold)
		// Infinity symbol
		cv.arc(105, 60, 15, 0, 2*math.Pi, 2.5, p)
		cv.arc(135, 60, 15, 0, 2*math.Pi, 2.5, p)
		cv.glow(120, 60, 10, p)
		// Figure
		cv.filledCircle(120, 120, 22, skin)
		cv.glow(120, 120, 12, warmWhite)
		cv.filledRect(105, 145, 30, 80, s)
		// Arms outstretched
		cv.line(105, 170, 50, 155, 3, skin)
		cv.line(135, 170, 190, 155, 3, skin)
		// Right hand pointing up
		cv.glow(50, 155, 8, p)
		// Left hand pointing down
		cv.glow(190, 155, 8, s)
		// Table
		cv.filledRect(40, 260, 160, 8, copper)
		cv.line(40, 260, 200, 260, 2, gold)
		// Four elements on table
		cv.drawSuitPip(0, 60, 290, 25)
		cv.drawSuitPip(1, 100, 290, 25)
		cv.drawSuitPip(2, 140, 290, 25)
		cv.drawSuitPip(3, 180, 290, 25)
		// Roses below
		cv.floral(60, 350, 10, deepRose)
		cv.floral(180, 350, 10, deepRose)

	case 2: // High Priestess — crescent moon, pillars, veil
		// Moon crescent
		cv.filledCircle(120, 55, 25, p)
		cv.filledCircle(132, 50, 25, bgMid) // crescent cutout
		cv.glow(110, 55, 20, p)
		// Two pillars with vine motifs
		cv.filledRect(25, 90, 20, 280, color.RGBA{60, 50, 70, 255})
		cv.filledRect(195, 90, 20, 280, color.RGBA{60, 50, 70, 255})
		// Pillar capitals
		cv.filledRect(20, 85, 30, 10, gold)
		cv.filledRect(190, 85, 30, 10, gold)
		// Vine on pillars
		cv.vine(35, 100, 35, 360, dimGold)
		cv.vine(205, 100, 205, 360, dimGold)
		// Veil between pillars (translucent)
		for y := 100; y < 350; y++ {
			a := 0.15 + 0.08*math.Sin(float64(y)*0.05)
			for x := 50; x < 190; x++ {
				cv.blend(x, y, p, a)
			}
		}
		// Figure
		cv.filledCircle(120, 140, 20, skin)
		cv.glow(120, 140, 10, warmWhite)
		// Robes
		for y := 165.0; y < 340; y++ {
			t := (y - 165) / 175
			w := 20 + t*40
			cv.filledRect(120-w, y, w*2, 1, lerpColor(p, color.RGBA{p.R / 2, p.G / 2, p.B / 2, 255}, t))
		}
		// Scroll/Torah
		cv.filledRect(105, 300, 30, 20, cream)
		cv.line(105, 300, 135, 300, 2, gold)
		cv.line(105, 320, 135, 320, 2, gold)

	case 3: // Empress — crown, flowing robes, garden
		// Crown with gems
		for i := 0; i < 7; i++ {
			x := 92 + float64(i)*8
			cv.line(x, 45, x, 58, 2, gold)
			cv.filledCircle(x, 42, 4, amber)
		}
		cv.filledRect(88, 58, 64, 6, gold)
		// Head
		cv.filledCircle(120, 75, 22, skin)
		cv.glow(120, 75, 12, warmWhite)
		// Hair flowing
		cv.bezier(100, 65, 80, 80, 70, 120, 75, 150, 2, color.RGBA{160, 100, 40, 255})
		cv.bezier(140, 65, 160, 80, 170, 120, 165, 150, 2, color.RGBA{160, 100, 40, 255})
		// Flowing robes (green/gold, widening greatly)
		for y := 100.0; y < 300; y++ {
			t := (y - 100) / 200
			w := 25 + t*80
			col := lerpColor(p, s, t*0.3)
			cv.filledRect(120-w, y, w*2, 1, col)
		}
		// Garden below
		for x := 0.0; x < 240; x += 15 {
			h := 40 + 20*math.Sin(x*0.1)
			for y := 400 - h; y < 400; y++ {
				cv.blend(int(x), int(y), p, 0.3)
			}
			cv.floral(x, 400-h-5, 8, deepRose)
		}
		// Wheat/grain accents
		cv.vine(30, 300, 30, 380, dimGold)
		cv.vine(210, 300, 210, 380, dimGold)
		// Star accents
		cv.glow(50, 50, 8, s)
		cv.glow(190, 50, 8, s)

	case 4: // Emperor — throne, angular, commanding
		// Crown
		for i := 0; i < 5; i++ {
			x := 100 + float64(i)*10
			h := 20.0
			if i%2 == 1 {
				h = 15
			}
			cv.line(x, 50-h, x, 55, 3, gold)
			cv.filledCircle(x, 50-h-3, 5, deepRose)
		}
		cv.filledRect(96, 55, 48, 8, gold)
		// Head
		cv.filledCircle(120, 75, 24, skin)
		cv.glow(120, 75, 12, warmWhite)
		// Beard
		cv.filledCircle(120, 95, 14, color.RGBA{130, 80, 40, 255})
		// Throne — angular, ornate
		cv.filledRect(30, 50, 20, 280, color.RGBA{80, 40, 30, 255})
		cv.filledRect(190, 50, 20, 280, color.RGBA{80, 40, 30, 255})
		// Throne back
		cv.filledRect(30, 45, 180, 10, gold)
		// Ram heads on throne
		cv.circle(40, 70, 12, 2, gold)
		cv.circle(200, 70, 12, 2, gold)
		// Body — broad
		cv.filledRect(80, 105, 80, 130, p)
		cv.line(80, 105, 160, 105, 2, gold) // collar
		// Orb in left hand
		cv.filledCircle(175, 180, 15, s)
		cv.glow(175, 180, 12, s)
		// Ankh/scepter in right hand
		cv.line(65, 130, 65, 260, 3, gold)
		cv.circle(65, 125, 8, 2, gold)
		// Legs
		cv.filledRect(90, 235, 20, 60, color.RGBA{p.R / 2, p.G / 2, p.B / 2, 255})
		cv.filledRect(130, 235, 20, 60, color.RGBA{p.R / 2, p.G / 2, p.B / 2, 255})

	case 5: // Hierophant — triple cross, ornate columns
		// Ornate columns
		cv.filledRect(20, 80, 25, 300, color.RGBA{70, 55, 40, 255})
		cv.filledRect(195, 80, 25, 300, color.RGBA{70, 55, 40, 255})
		cv.filledRect(15, 75, 35, 10, gold)
		cv.filledRect(190, 75, 35, 10, gold)
		cv.vine(32, 100, 32, 370, dimGold)
		cv.vine(207, 100, 207, 370, dimGold)
		// Triple cross
		cv.line(120, 30, 120, 130, 3, s)
		cv.line(110, 50, 130, 50, 3, s)
		cv.line(105, 80, 135, 80, 3, s)
		cv.line(108, 110, 132, 110, 3, s)
		cv.glow(120, 80, 15, s)
		// Figure
		cv.filledCircle(120, 165, 22, skin)
		cv.glow(120, 165, 12, warmWhite)
		// Papal tiara
		cv.filledRect(105, 138, 30, 12, gold)
		cv.filledRect(110, 130, 20, 10, gold)
		cv.filledCircle(120, 128, 8, amber)
		// Robes
		cv.filledRect(90, 190, 60, 100, p)
		cv.line(90, 190, 150, 190, 2, gold)
		// Blessing hand
		cv.line(140, 200, 170, 180, 3, skin)
		cv.glow(170, 178, 8, warmWhite)
		// Two supplicants
		cv.filledCircle(70, 320, 15, skin)
		cv.filledRect(60, 340, 20, 40, color.RGBA{80, 60, 50, 255})
		cv.filledCircle(170, 320, 15, skin)
		cv.filledRect(160, 340, 20, 40, color.RGBA{80, 60, 50, 255})

	case 6: // Lovers — two figures, angel above, garden frame
		// Angel/cupid above
		cv.filledCircle(120, 60, 18, skin)
		cv.glow(120, 60, 15, warmWhite)
		// Wings
		cv.bezier(102, 65, 70, 40, 50, 50, 60, 80, 2, gold)
		cv.bezier(138, 65, 170, 40, 190, 50, 180, 80, 2, gold)
		// Heart between
		cv.drawHeart(120, 100, 25, p)
		cv.glow(120, 100, 15, p)
		// Two figures
		// Left figure
		cv.filledCircle(75, 170, 20, skin)
		cv.glow(75, 170, 10, warmWhite)
		cv.filledRect(60, 195, 30, 90, color.RGBA{200, 160, 120, 255})
		cv.filledRect(63, 285, 12, 45, color.RGBA{180, 140, 100, 255})
		cv.filledRect(83, 285, 12, 45, color.RGBA{180, 140, 100, 255})
		// Right figure
		cv.filledCircle(165, 170, 20, skin)
		cv.glow(165, 170, 10, warmWhite)
		cv.filledRect(150, 195, 30, 90, color.RGBA{180, 120, 140, 255})
		cv.filledRect(153, 285, 12, 45, color.RGBA{160, 100, 120, 255})
		cv.filledRect(173, 285, 12, 45, color.RGBA{160, 100, 120, 255})
		// Hands reaching
		cv.line(90, 220, 150, 220, 2.5, skin)
		cv.glow(120, 220, 10, warmWhite)
		// Garden frame (vines + flowers)
		cv.vine(10, 350, 230, 350, dimGold)
		for x := 20.0; x < 230; x += 30 {
			cv.floral(x, 360, 8, deepRose)
		}

	case 7: // Chariot — vehicle with sphinxes, stars
		// Starry canopy
		for i := 0; i < 6; i++ {
			x := 40 + float64(i)*32
			cv.glow(x, 40+float64(i%3)*10, 8, s)
			cv.filledCircle(x, 40+float64(i%3)*10, 3, warmWhite)
		}
		// Figure
		cv.filledCircle(120, 100, 22, skin)
		cv.glow(120, 100, 12, warmWhite)
		// Armor
		cv.filledRect(100, 125, 40, 60, p)
		cv.line(100, 125, 140, 125, 2, gold)
		// Chariot body
		cv.filledRect(40, 190, 160, 80, color.RGBA{80, 50, 35, 255})
		cv.line(40, 190, 200, 190, 3, gold)
		cv.line(40, 270, 200, 270, 3, gold)
		cv.line(40, 190, 40, 270, 2, gold)
		cv.line(200, 190, 200, 270, 2, gold)
		// Star emblem
		cv.drawPentacle(120, 230, 30, gold)
		cv.glow(120, 230, 12, gold)
		// Wheels
		cv.circle(50, 300, 25, 3, gold)
		cv.filledCircle(50, 300, 8, amber)
		cv.circle(190, 300, 25, 3, gold)
		cv.filledCircle(190, 300, 8, amber)
		// Spokes
		for i := 0; i < 6; i++ {
			a := float64(i) * math.Pi / 3
			cv.line(50+25*math.Cos(a), 300+25*math.Sin(a), 50, 300, 1.5, dimGold)
			cv.line(190+25*math.Cos(a), 300+25*math.Sin(a), 190, 300, 1.5, dimGold)
		}
		// Sphinxes
		cv.filledCircle(60, 350, 15, color.RGBA{60, 50, 40, 255})
		cv.filledRect(50, 360, 20, 15, color.RGBA{50, 40, 35, 255})
		cv.filledCircle(180, 350, 15, color.RGBA{80, 70, 60, 255})
		cv.filledRect(170, 360, 20, 15, color.RGBA{70, 60, 50, 255})

	case 8: // Strength — figure with lion, infinity, floral frame
		// Infinity symbol
		cv.arc(105, 50, 18, 0, 2*math.Pi, 2.5, s)
		cv.arc(135, 50, 18, 0, 2*math.Pi, 2.5, s)
		cv.glow(120, 50, 10, s)
		// Floral arch frame
		cv.arc(120, 120, 90, math.Pi, 2*math.Pi, 2, dimGold)
		for a := math.Pi; a < 2*math.Pi; a += 0.5 {
			fx := 120 + 90*math.Cos(a)
			fy := 120 + 90*math.Sin(a)
			cv.floral(fx, fy, 6, dimGold)
		}
		// Figure
		cv.filledCircle(100, 130, 20, skin)
		cv.glow(100, 130, 10, warmWhite)
		// Hair
		cv.bezier(85, 125, 70, 140, 65, 170, 75, 190, 2, color.RGBA{160, 100, 40, 255})
		// Robes
		for y := 155.0; y < 280; y++ {
			t := (y - 155) / 125
			w := 15 + t*30
			cv.filledRect(100-w, y, w*2, 1, lerpColor(warmWhite, cream, t))
		}
		// Lion
		cv.filledCircle(150, 220, 30, p)
		cv.glow(150, 220, 18, amber)
		// Mane
		cv.arc(150, 220, 35, 0, 2*math.Pi, 3, dimGold)
		// Lion body
		cv.filledRect(130, 250, 50, 40, color.RGBA{p.R / 2, p.G / 2, p.B / 2, 255})
		// Legs
		cv.filledRect(130, 290, 12, 30, color.RGBA{p.R / 2, p.G / 2, p.B / 2, 255})
		cv.filledRect(155, 290, 12, 30, color.RGBA{p.R / 2, p.G / 2, p.B / 2, 255})
		// Hand on lion
		cv.line(100, 155, 140, 210, 3, skin)

	case 9: // Hermit — lantern, mountain, star, solitary path
		// Mountain
		for y := 180; y < 400; y++ {
			t := float64(y-180) / 220
			w := t * 120
			col := lerpColor(color.RGBA{60, 45, 35, 255}, bgDark, t*0.5)
			cv.filledRect(120-w, float64(y), w*2, 1, col)
		}
		// Path winding down
		cv.bezier(120, 250, 100, 300, 140, 340, 120, 400, 2, dimGold)
		// Figure on mountain
		cv.filledCircle(120, 150, 18, skin)
		cv.glow(120, 150, 8, warmWhite)
		// Hooded cloak
		cv.arc(120, 140, 22, math.Pi, 2*math.Pi, 3, color.RGBA{80, 60, 50, 255})
		cv.filledRect(100, 170, 40, 80, color.RGBA{80, 60, 50, 255})
		// Staff
		cv.line(85, 150, 85, 300, 3, copper)
		// Lantern (glowing)
		cv.filledRect(155, 135, 15, 20, gold)
		cv.glow(162, 145, 25, amber)
		cv.glow(162, 145, 15, warmWhite)
		// Star above
		cv.glow(120, 40, 15, s)
		cv.filledCircle(120, 40, 5, warmWhite)
		// Smaller stars
		cv.glow(60, 60, 6, s)
		cv.glow(180, 50, 6, s)

	case 10: // Wheel of Fortune — ornate wheel, four corner creatures
		// Large ornate wheel
		cv.circle(120, 200, 80, 3, gold)
		cv.circle(120, 200, 70, 2, amber)
		cv.circle(120, 200, 50, 2, dimGold)
		cv.circle(120, 200, 20, 2, copper)
		// Spokes
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			cv.line(120+20*math.Cos(a), 200+20*math.Sin(a),
				120+80*math.Cos(a), 200+80*math.Sin(a), 2, dimGold)
		}
		// Center
		cv.filledCircle(120, 200, 12, gold)
		cv.glow(120, 200, 15, amber)
		// Symbols on wheel (T A R O)
		cv.drawDigit(110, 135, 7, warmWhite, 4) // T-ish
		cv.drawDigit(165, 192, 0, warmWhite, 4) // A-ish (using 0)
		cv.drawDigit(110, 250, 8, warmWhite, 4) // R-ish
		cv.drawDigit(63, 192, 0, warmWhite, 4)  // O
		// Four corner creatures
		cv.filledCircle(30, 40, 15, gold)   // eagle/angel
		cv.filledCircle(210, 40, 15, gold)  // bull
		cv.filledCircle(30, 360, 15, gold)  // lion
		cv.filledCircle(210, 360, 15, gold) // human
		cv.glow(30, 40, 10, amber)
		cv.glow(210, 40, 10, amber)
		cv.glow(30, 360, 10, amber)
		cv.glow(210, 360, 10, amber)

	case 11: // Justice — scales, sword, balanced columns
		// Two columns
		cv.filledRect(25, 80, 18, 280, color.RGBA{70, 55, 40, 255})
		cv.filledRect(197, 80, 18, 280, color.RGBA{70, 55, 40, 255})
		cv.filledRect(20, 75, 28, 8, gold)
		cv.filledRect(192, 75, 28, 8, gold)
		// Sword (vertical center)
		cv.line(120, 40, 120, 230, 3, s)
		cv.line(105, 80, 135, 80, 3, s)
		cv.glow(120, 50, 10, warmWhite)
		// Figure
		cv.filledCircle(120, 140, 22, skin)
		cv.glow(120, 140, 12, warmWhite)
		// Crown/blindfold
		cv.filledRect(102, 130, 36, 6, gold)
		// Robes
		for y := 165.0; y < 300; y++ {
			t := (y - 165) / 135
			w := 20 + t*35
			cv.filledRect(120-w, y, w*2, 1, lerpColor(p, color.RGBA{p.R / 2, p.G / 2, p.B / 2, 255}, t))
		}
		// Scales bar
		cv.line(50, 190, 190, 190, 3, gold)
		// Scale pans (arcs)
		cv.arc(65, 220, 20, 0, math.Pi, 2, gold)
		cv.arc(175, 220, 20, 0, math.Pi, 2, gold)
		// Chains
		cv.line(65, 190, 65, 220, 1.5, dimGold)
		cv.line(175, 190, 175, 220, 1.5, dimGold)
		// Pedestal
		cv.filledRect(80, 310, 80, 15, copper)
		cv.line(80, 310, 160, 310, 2, gold)

	case 12: // Hanged Man — inverted figure, tree, halo
		// Tree (organic, Art Nouveau)
		cv.filledRect(110, 30, 20, 100, color.RGBA{90, 60, 35, 255})
		// Branches
		cv.bezier(110, 40, 60, 30, 30, 50, 20, 40, 3, color.RGBA{90, 60, 35, 255})
		cv.bezier(130, 40, 180, 30, 210, 50, 220, 40, 3, color.RGBA{90, 60, 35, 255})
		cv.bezier(110, 60, 70, 55, 40, 75, 30, 65, 2, color.RGBA{80, 55, 30, 255})
		cv.bezier(130, 60, 170, 55, 200, 75, 210, 65, 2, color.RGBA{80, 55, 30, 255})
		// Leaves
		for x := 20.0; x < 230; x += 25 {
			cv.floral(x, 35+10*math.Sin(x*0.05), 6, color.RGBA{80, 120, 50, 255})
		}
		// Rope
		cv.line(120, 130, 120, 170, 2, copper)
		// Inverted figure
		// Feet at top (tied)
		cv.filledCircle(120, 140, 6, skin)
		// Legs forming triangle
		cv.line(120, 145, 105, 195, 3, p)
		cv.line(120, 145, 135, 195, 3, p)
		cv.line(105, 195, 135, 195, 2, p)
		// Body hanging
		cv.filledRect(108, 195, 24, 60, s)
		// Arms behind
		cv.line(108, 220, 85, 250, 2, skin)
		cv.line(132, 220, 155, 250, 2, skin)
		// Head at bottom with golden halo
		cv.filledCircle(120, 280, 20, skin)
		cv.circle(120, 280, 30, 3, gold)
		cv.glow(120, 280, 35, dimGold)
		cv.glow(120, 280, 20, warmWhite)
		// Serene expression
		cv.filledCircle(114, 278, 2, color.RGBA{80, 50, 30, 255})
		cv.filledCircle(126, 278, 2, color.RGBA{80, 50, 30, 255})

	case 13: // Death — skeleton, scythe, rose of rebirth
		// Skull
		cv.filledCircle(120, 100, 35, warmWhite)
		// Eye sockets
		cv.filledCircle(108, 95, 10, bgDark)
		cv.filledCircle(132, 95, 10, bgDark)
		// Nose
		cv.filledCircle(120, 108, 5, bgDark)
		// Mouth
		cv.filledRect(108, 118, 24, 4, bgDark)
		for x := 108.0; x < 132; x += 6 {
			cv.line(x, 118, x, 122, 1, warmWhite)
		}
		cv.glow(120, 100, 25, warmWhite)
		// Scythe handle
		cv.line(180, 80, 80, 350, 3, copper)
		// Scythe blade (arc)
		cv.arc(140, 90, 50, math.Pi+0.3, 2*math.Pi-0.3, 3, swordsSilver)
		cv.glow(140, 60, 15, warmWhite)
		// Skeletal body
		cv.line(120, 135, 120, 240, 3, warmWhite)
		// Ribs
		for i := 0; i < 4; i++ {
			y := 160 + float64(i)*18
			cv.arc(120, y, 20, 0, math.Pi, 1.5, warmWhite)
		}
		// Skeletal legs
		cv.line(120, 240, 100, 310, 2, warmWhite)
		cv.line(120, 240, 140, 310, 2, warmWhite)
		// Rose of rebirth at bottom
		cv.floral(120, 355, 18, deepRose)
		cv.glow(120, 355, 15, deepRose)
		// Small roses
		cv.floral(50, 370, 8, deepRose)
		cv.floral(190, 370, 8, deepRose)

	case 14: // Temperance — angel, two vessels, flowing water, wings
		// Wings
		cv.bezier(90, 100, 30, 60, 10, 50, 20, 90, 3, gold)
		cv.bezier(150, 100, 210, 60, 230, 50, 220, 90, 3, gold)
		// Wing feather lines
		for i := 0; i < 5; i++ {
			t := float64(i) / 5
			cv.bezier(90, 100, 30+t*30, 60+t*15, 10+t*20, 50+t*20, 20+t*15, 90, 1.5, dimGold)
			cv.bezier(150, 100, 210-t*30, 60+t*15, 230-t*20, 50+t*20, 220-t*15, 90, 1.5, dimGold)
		}
		// Halo
		cv.circle(120, 65, 18, 2, gold)
		cv.glow(120, 65, 12, gold)
		// Head
		cv.filledCircle(120, 90, 22, skin)
		cv.glow(120, 90, 12, warmWhite)
		// Robes
		for y := 115.0; y < 300; y++ {
			t := (y - 115) / 185
			w := 20 + t*45
			cv.filledRect(120-w, y, w*2, 1, lerpColor(p, cream, t*0.3))
		}
		// Two vessels
		cv.filledRect(55, 175, 25, 35, gold)
		cv.line(55, 175, 80, 175, 2, amber)
		cv.filledRect(160, 210, 25, 35, gold)
		cv.line(160, 210, 185, 210, 2, amber)
		// Flowing water between
		cv.bezier(75, 190, 100, 185, 140, 205, 165, 215, 3, p)
		cv.glow(120, 200, 15, p)
		// Ground/pool
		for x := 20; x < 220; x++ {
			for y := 340; y < 380; y++ {
				t := float64(y-340) / 40
				wave := 0.1 * math.Sin(float64(x)*0.1+float64(y)*0.05)
				cv.blend(x, y, p, 0.2+t*0.15+wave)
			}
		}

	case 15: // Devil — horned figure, chains, captives
		// Horns
		cv.bezier(100, 80, 85, 40, 70, 30, 60, 45, 3, p)
		cv.bezier(140, 80, 155, 40, 170, 30, 180, 45, 3, p)
		// Head
		cv.filledCircle(120, 95, 28, color.RGBA{p.R / 2, p.G / 2, p.B / 2, 255})
		cv.glow(120, 95, 18, p)
		// Glowing eyes
		cv.glow(110, 90, 8, amber)
		cv.glow(130, 90, 8, amber)
		cv.filledCircle(110, 90, 3, warmWhite)
		cv.filledCircle(130, 90, 3, warmWhite)
		// Body
		cv.filledRect(90, 125, 60, 100, color.RGBA{p.R / 3, p.G / 3, p.B / 3, 255})
		// Inverted pentagram on chest
		cv.drawPentacle(120, 170, 30, s)
		cv.glow(120, 170, 12, s)
		// Wings (bat-like)
		cv.bezier(90, 130, 50, 110, 30, 130, 35, 160, 2, color.RGBA{p.R / 3, p.G / 3, p.B / 3, 255})
		cv.bezier(150, 130, 190, 110, 210, 130, 205, 160, 2, color.RGBA{p.R / 3, p.G / 3, p.B / 3, 255})
		// Chains
		for y := 230.0; y < 310; y += 8 {
			cv.filledCircle(80, y, 3, dimGold)
			cv.filledCircle(160, y, 3, dimGold)
		}
		// Captive figures
		cv.filledCircle(60, 330, 14, skin)
		cv.filledRect(50, 348, 20, 35, color.RGBA{120, 80, 70, 255})
		cv.filledCircle(180, 330, 14, skin)
		cv.filledRect(170, 348, 20, 35, color.RGBA{120, 80, 70, 255})

	case 16: // Tower — crumbling tower, lightning, falling figures
		// Tower
		cv.filledRect(85, 80, 70, 250, color.RGBA{100, 70, 50, 255})
		// Tower top (crumbling)
		cv.filledRect(80, 70, 80, 20, color.RGBA{120, 80, 55, 255})
		// Crown fragments falling
		cv.filledRect(60, 50, 15, 10, color.RGBA{100, 70, 50, 255})
		cv.filledRect(170, 60, 12, 8, color.RGBA{100, 70, 50, 255})
		// Windows
		for y := 120.0; y < 300; y += 50 {
			cv.filledRect(100, y, 15, 20, color.RGBA{30, 15, 10, 255})
			cv.arc(107, y, 7, math.Pi, 2*math.Pi, 1.5, color.RGBA{80, 55, 40, 255})
			cv.filledRect(130, y+25, 15, 20, color.RGBA{30, 15, 10, 255})
			cv.arc(137, y+25, 7, math.Pi, 2*math.Pi, 1.5, color.RGBA{80, 55, 40, 255})
		}
		// Lightning bolt
		cv.line(150, 10, 135, 40, 4, amber)
		cv.line(135, 40, 155, 55, 4, amber)
		cv.line(155, 55, 125, 75, 5, amber)
		cv.glow(140, 50, 20, amber)
		cv.glow(130, 70, 25, warmWhite)
		// Explosion glow at top
		cv.glow(120, 75, 30, s)
		// Falling figures
		cv.filledCircle(45, 200, 12, skin)
		cv.filledRect(38, 215, 15, 25, p)
		cv.filledCircle(200, 230, 12, skin)
		cv.filledRect(193, 245, 15, 25, p)
		// Flames at base
		for x := 80.0; x < 160; x += 3 {
			h := 20 + 15*math.Sin(x*0.2)
			for y := 330 - h; y < 330; y++ {
				t := (330 - y) / h
				col := lerpColor(deepRose, amber, t)
				cv.blend(int(x), int(y), col, 0.7)
			}
		}

	case 17: // Star — large star, water bearer, seven smaller stars
		// Large 8-pointed star
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			for r := 0.0; r < 50; r += 1 {
				px := 120 + r*math.Cos(a)
				py := 90 + r*math.Sin(a)
				cv.blend(int(px), int(py), p, 0.8-r/80)
			}
		}
		cv.glow(120, 90, 30, p)
		cv.filledCircle(120, 90, 12, warmWhite)
		// Seven smaller stars
		starPos := [][2]float64{{50, 50}, {190, 40}, {30, 120}, {210, 100}, {60, 170}, {185, 160}, {40, 60}}
		for _, sp := range starPos {
			cv.glow(sp[0], sp[1], 8, p)
			cv.filledCircle(sp[0], sp[1], 3, warmWhite)
		}
		// Water bearer figure
		cv.filledCircle(120, 230, 18, skin)
		cv.glow(120, 230, 10, warmWhite)
		cv.filledRect(105, 250, 30, 60, cream)
		// Pouring water (two streams)
		cv.bezier(95, 265, 70, 290, 60, 330, 50, 380, 3, p)
		cv.bezier(145, 265, 170, 290, 180, 330, 190, 380, 3, p)
		cv.glow(65, 340, 15, p)
		cv.glow(175, 340, 15, p)
		// Pool
		for x := 20; x < 220; x++ {
			for y := 370; y < 395; y++ {
				cv.blend(x, y, p, 0.2+0.1*math.Sin(float64(x)*0.1))
			}
		}

	case 18: // Moon — crescent, two towers, path, wolves
		// Large crescent moon
		cv.filledCircle(120, 80, 40, p)
		cv.filledCircle(138, 70, 40, bgMid) // crescent cutout
		cv.glow(105, 85, 35, p)
		// Moon drops / rays
		for i := 0; i < 8; i++ {
			x := 70 + float64(i)*20
			cv.line(x, 130, x+5, 150, 1.5, dimGold)
		}
		// Two towers
		cv.filledRect(25, 180, 35, 150, color.RGBA{70, 50, 45, 255})
		cv.filledRect(180, 180, 35, 150, color.RGBA{70, 50, 45, 255})
		// Tower tops (crenellations)
		for x := 25.0; x < 60; x += 10 {
			cv.filledRect(x, 170, 7, 12, color.RGBA{70, 50, 45, 255})
		}
		for x := 180.0; x < 215; x += 10 {
			cv.filledRect(x, 170, 7, 12, color.RGBA{70, 50, 45, 255})
		}
		// Window lights
		cv.glow(42, 220, 8, amber)
		cv.glow(197, 220, 8, amber)
		// Winding path
		cv.bezier(120, 395, 100, 360, 140, 320, 120, 280, 4, dimGold)
		cv.bezier(120, 280, 100, 250, 110, 220, 120, 180, 3, dimGold)
		// Wolves/dogs
		// Left wolf
		cv.filledCircle(65, 340, 12, color.RGBA{100, 80, 60, 255})
		cv.filledRect(55, 348, 25, 15, color.RGBA{90, 70, 55, 255})
		cv.filledRect(58, 363, 8, 12, color.RGBA{80, 65, 50, 255})
		cv.filledRect(70, 363, 8, 12, color.RGBA{80, 65, 50, 255})
		// Right wolf
		cv.filledCircle(175, 340, 12, color.RGBA{100, 80, 60, 255})
		cv.filledRect(165, 348, 25, 15, color.RGBA{90, 70, 55, 255})
		cv.filledRect(168, 363, 8, 12, color.RGBA{80, 65, 50, 255})
		cv.filledRect(180, 363, 8, 12, color.RGBA{80, 65, 50, 255})

	case 19: // Sun — radiant sun, child, sunflowers
		// Large sun
		cv.filledCircle(120, 100, 50, p)
		cv.glow(120, 100, 45, p)
		cv.glow(120, 100, 25, warmWhite)
		// Sun face
		cv.filledCircle(108, 95, 5, color.RGBA{s.R / 2, s.G / 2, s.B / 2, 255})
		cv.filledCircle(132, 95, 5, color.RGBA{s.R / 2, s.G / 2, s.B / 2, 255})
		cv.arc(120, 108, 15, 0.2, math.Pi-0.2, 2, color.RGBA{s.R / 2, s.G / 2, s.B / 2, 255})
		// Rays
		for i := 0; i < 16; i++ {
			a := float64(i) * math.Pi / 8
			r1 := 55.0
			r2 := 80.0
			if i%2 == 0 {
				r2 = 90
			}
			cv.line(120+r1*math.Cos(a), 100+r1*math.Sin(a),
				120+r2*math.Cos(a), 100+r2*math.Sin(a), 2.5, s)
		}
		// Child figure
		cv.filledCircle(120, 260, 16, skin)
		cv.glow(120, 260, 8, warmWhite)
		cv.filledRect(110, 278, 20, 40, warmWhite)
		cv.filledRect(112, 318, 10, 25, skin)
		cv.filledRect(128, 318, 10, 25, skin)
		// Arms up (joyful)
		cv.line(110, 285, 90, 260, 2, skin)
		cv.line(130, 285, 150, 260, 2, skin)
		// Sunflowers
		cv.floral(40, 350, 15, gold)
		cv.floral(80, 360, 12, gold)
		cv.floral(160, 360, 12, gold)
		cv.floral(200, 350, 15, gold)
		// Stems
		cv.line(40, 365, 40, 400, 2, color.RGBA{80, 120, 50, 255})
		cv.line(80, 372, 80, 400, 2, color.RGBA{80, 120, 50, 255})
		cv.line(160, 372, 160, 400, 2, color.RGBA{80, 120, 50, 255})
		cv.line(200, 365, 200, 400, 2, color.RGBA{80, 120, 50, 255})

	case 20: // Judgement — angel trumpet, rising figures, clouds
		// Angel
		cv.filledCircle(120, 60, 20, skin)
		cv.glow(120, 60, 12, warmWhite)
		// Halo
		cv.circle(120, 45, 22, 2, gold)
		cv.glow(120, 45, 15, gold)
		// Wings
		cv.bezier(100, 70, 50, 30, 20, 40, 25, 80, 3, gold)
		cv.bezier(140, 70, 190, 30, 220, 40, 215, 80, 3, gold)
		for i := 0; i < 4; i++ {
			t := float64(i) * 0.15
			cv.bezier(100, 70, 50+t*40, 30+t*20, 20+t*30, 40+t*20, 25+t*20, 80, 1.5, dimGold)
			cv.bezier(140, 70, 190-t*40, 30+t*20, 220-t*30, 40+t*20, 215-t*20, 80, 1.5, dimGold)
		}
		// Trumpet
		cv.line(120, 85, 185, 105, 3, gold)
		cv.filledCircle(190, 107, 8, gold)
		cv.glow(190, 107, 10, amber)
		// Sound waves
		cv.arc(195, 107, 15, -0.5, 0.5, 1.5, amber)
		cv.arc(195, 107, 22, -0.4, 0.4, 1.5, dimGold)
		// Clouds
		for x := 20.0; x < 220; x += 40 {
			cy := 130 + 10*math.Sin(x*0.05)
			cv.filledCircle(x, cy, 18, color.RGBA{60, 45, 40, 255})
			cv.filledCircle(x+15, cy-5, 15, color.RGBA{55, 42, 38, 255})
		}
		// Rising figures
		figs := []float64{60, 120, 180}
		for _, fx := range figs {
			cv.filledCircle(fx, 260, 14, skin)
			cv.glow(fx, 260, 8, warmWhite)
			cv.filledRect(fx-10, 278, 20, 40, cream)
			// Arms raised
			cv.line(fx-10, 285, fx-20, 265, 2, skin)
			cv.line(fx+10, 285, fx+20, 265, 2, skin)
		}
		// Coffins / ground
		cv.filledRect(10, 330, 220, 8, copper)
		cv.line(10, 330, 230, 330, 2, gold)
		// Glow around rising
		cv.glow(120, 290, 40, dimGold)

	case 21: // World — wreath, dancing figure, four corners
		// Wreath (oval of vines and flowers)
		for a := 0.0; a < 2*math.Pi; a += 0.08 {
			rx := 80.0
			ry := 120.0
			x := 120 + rx*math.Cos(a)
			y := 200 + ry*math.Sin(a)
			cv.filledCircle(x, y, 4, color.RGBA{80, 120, 50, 255})
		}
		// Flowers on wreath
		for a := 0.0; a < 2*math.Pi; a += math.Pi / 6 {
			x := 120 + 80*math.Cos(a)
			y := 200 + 120*math.Sin(a)
			cv.floral(x, y, 8, dimGold)
		}
		cv.glow(120, 80, 12, gold)
		cv.glow(120, 320, 12, gold)
		cv.glow(40, 200, 12, gold)
		cv.glow(200, 200, 12, gold)
		// Red ribbon ties
		cv.filledCircle(120, 80, 5, deepRose)
		cv.filledCircle(120, 320, 5, deepRose)
		cv.filledCircle(40, 200, 5, deepRose)
		cv.filledCircle(200, 200, 5, deepRose)
		// Dancing figure inside
		cv.filledCircle(120, 165, 18, skin)
		cv.glow(120, 165, 10, warmWhite)
		// Draped cloth
		cv.bezier(105, 180, 95, 220, 100, 260, 110, 280, 2.5, s)
		cv.bezier(135, 180, 145, 220, 140, 260, 130, 280, 2.5, s)
		cv.filledRect(108, 185, 24, 50, color.RGBA{s.R / 2, s.G / 2, s.B / 2, 255})
		// Arms out
		cv.line(105, 195, 75, 175, 2.5, skin)
		cv.line(135, 195, 165, 175, 2.5, skin)
		// Wands in hands
		cv.line(70, 170, 80, 160, 2, gold)
		cv.line(160, 170, 170, 160, 2, gold)
		// Legs (dancing pose)
		cv.line(113, 235, 95, 280, 2.5, skin)
		cv.line(127, 235, 145, 280, 2.5, skin)
		// Four corner symbols
		cv.filledCircle(25, 35, 12, gold)  // eagle
		cv.glow(25, 35, 10, amber)
		cv.filledCircle(215, 35, 12, gold) // bull
		cv.glow(215, 35, 10, amber)
		cv.filledCircle(25, 365, 12, gold) // lion
		cv.glow(25, 365, 10, amber)
		cv.filledCircle(215, 365, 12, gold) // human
		cv.glow(215, 365, 10, amber)
	}

	return cv
}

// ========== main ==========

func main() {
	base := "decks/nouveau"

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
