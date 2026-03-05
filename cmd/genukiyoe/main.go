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

// ========== Ukiyo-e decorative elements ==========

// wave draws a Hokusai-style wave curl
func (c *HiResCanvas) wave(cx, cy, size float64, col color.RGBA) {
	// Main curl
	for a := 0.0; a < 2.5; a += 0.05 {
		r := size * (1 - a/4)
		px := cx + r*math.Cos(a)
		py := cy - r*math.Sin(a)
		c.filledCircle(px, py, 1.5, col)
	}
	// Spray dots
	foam := color.RGBA{
		clampU8(float64(col.R)*0.7 + 80),
		clampU8(float64(col.G)*0.7 + 80),
		clampU8(float64(col.B)*0.7 + 80),
		255,
	}
	for i := 0; i < 5; i++ {
		a := 0.3 + float64(i)*0.4
		px := cx + (size+5)*math.Cos(a)
		py := cy - (size+5)*math.Sin(a)
		c.filledCircle(px, py, 1.5, foam)
	}
}

// cherryBlossom draws a 5-petal sakura flower
func (c *HiResCanvas) cherryBlossom(cx, cy, size float64, col color.RGBA) {
	for i := 0; i < 5; i++ {
		a := float64(i)*2*math.Pi/5 - math.Pi/2
		px := cx + size*0.5*math.Cos(a)
		py := cy + size*0.5*math.Sin(a)
		c.filledCircle(px, py, size*0.35, col)
		// Notch in each petal (darker center line)
		notch := color.RGBA{col.R / 2, col.G / 2, col.B / 2, 255}
		c.filledCircle(px, py, size*0.1, notch)
	}
	// Center pistils
	center := color.RGBA{220, 200, 50, 255}
	c.filledCircle(cx, cy, size*0.15, center)
}

// bamboo draws a bamboo stalk segment
func (c *HiResCanvas) bamboo(x, y1, y2 float64, col color.RGBA) {
	c.line(x, y1, x, y2, 3, col)
	// Node rings
	nodeCol := color.RGBA{
		clampU8(float64(col.R) * 0.7),
		clampU8(float64(col.G) * 0.7),
		clampU8(float64(col.B) * 0.7),
		255,
	}
	segLen := (y2 - y1) / 4
	for i := 1; i < 4; i++ {
		ny := y1 + segLen*float64(i)
		c.line(x-4, ny, x+4, ny, 2, nodeCol)
	}
}

// bambooLeaf draws an angled leaf
func (c *HiResCanvas) bambooLeaf(cx, cy, angle, size float64, col color.RGBA) {
	// Thin elongated ellipse
	for t := 0.0; t <= 1.0; t += 0.01 {
		along := (t - 0.5) * size
		across := math.Sin(t*math.Pi) * size * 0.15
		px := cx + along*math.Cos(angle) - across*math.Sin(angle)
		py := cy + along*math.Sin(angle) + across*math.Cos(angle)
		c.filledCircle(px, py, 1.2, col)
	}
}

// torii draws a simplified torii gate
func (c *HiResCanvas) torii(cx, cy, w, h float64, col color.RGBA) {
	// Two pillars
	c.line(cx-w/2, cy, cx-w/2, cy+h, 3, col)
	c.line(cx+w/2, cy, cx+w/2, cy+h, 3, col)
	// Top beam (curved upward at ends)
	c.bezier(cx-w/2-10, cy-5, cx-w/4, cy-10, cx+w/4, cy-10, cx+w/2+10, cy-5, 3, col)
	// Lower cross beam
	c.line(cx-w/2+2, cy+h*0.2, cx+w/2-2, cy+h*0.2, 2.5, col)
}

// mountain draws a simple mountain silhouette
func (c *HiResCanvas) mountain(cx, cy, w, h float64, col color.RGBA) {
	for y := 0.0; y < h; y++ {
		t := y / h
		hw := w * t / 2
		for x := cx - hw; x <= cx+hw; x++ {
			c.blend(int(x), int(cy+y), col, 0.8)
		}
	}
	// Snow cap
	snow := color.RGBA{240, 235, 225, 255}
	for y := 0.0; y < h*0.25; y++ {
		t := y / (h * 0.25)
		hw := w * t / 2 * 0.3
		for x := cx - hw; x <= cx+hw; x++ {
			c.blend(int(x), int(cy+y), snow, 0.6)
		}
	}
}

// cloud draws a stylized Japanese cloud
func (c *HiResCanvas) cloud(cx, cy, size float64, col color.RGBA) {
	c.filledCircle(cx, cy, size*0.4, col)
	c.filledCircle(cx-size*0.35, cy+size*0.1, size*0.3, col)
	c.filledCircle(cx+size*0.35, cy+size*0.1, size*0.3, col)
	c.filledCircle(cx-size*0.2, cy-size*0.15, size*0.25, col)
	c.filledCircle(cx+size*0.2, cy-size*0.15, size*0.25, col)
}

// woodblockBorder draws a simple double-line border evoking woodblock print framing
func (c *HiResCanvas) woodblockBorder(margin, thickness float64, col color.RGBA) {
	w := float64(hiW)
	h := float64(hiH)
	m := margin
	c.line(m, m, w-m, m, thickness, col)
	c.line(w-m, m, w-m, h-m, thickness, col)
	c.line(w-m, h-m, m, h-m, thickness, col)
	c.line(m, h-m, m, m, thickness, col)
	// Inner line
	m2 := margin + 6
	c.line(m2, m2, w-m2, m2, thickness*0.5, col)
	c.line(w-m2, m2, w-m2, h-m2, thickness*0.5, col)
	c.line(w-m2, h-m2, m2, h-m2, thickness*0.5, col)
	c.line(m2, h-m2, m2, m2, thickness*0.5, col)
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

// ========== color palette ==========

var (
	ricePaper  = color.RGBA{235, 225, 200, 255}
	riceDark   = color.RGBA{200, 185, 155, 255}
	indigo     = color.RGBA{40, 50, 100, 255}
	indigoDark = color.RGBA{20, 25, 55, 255}
	indigoMid  = color.RGBA{55, 70, 130, 255}
	vermillion = color.RGBA{200, 50, 30, 255}
	vermDark   = color.RGBA{140, 35, 20, 255}
	sumi       = color.RGBA{30, 25, 20, 255}     // ink black
	sumiGray   = color.RGBA{60, 55, 45, 255}     // gray ink
	sakuraPink = color.RGBA{230, 160, 170, 255}
	sakuraDark = color.RGBA{190, 110, 120, 255}
	goldLeaf   = color.RGBA{200, 170, 50, 255}
	waveBlue   = color.RGBA{70, 120, 170, 255}
	waveLight  = color.RGBA{120, 170, 210, 255}
	bambooGrn  = color.RGBA{80, 120, 60, 255}
	bambooD    = color.RGBA{50, 80, 40, 255}
	skyBlue    = color.RGBA{140, 170, 200, 255}
	sunset     = color.RGBA{220, 130, 70, 255}
	moonWhite  = color.RGBA{240, 235, 225, 255}
	skin       = color.RGBA{225, 195, 160, 255}
)

// suit colors — mapped to Japanese aesthetics
var (
	cupsBlue     = color.RGBA{70, 110, 170, 255}  // water/wave blue
	cupsBlueDim  = color.RGBA{40, 60, 100, 255}
	wandsGreen   = color.RGBA{90, 130, 65, 255}   // bamboo green
	wandsGreenD  = color.RGBA{50, 80, 35, 255}
	swordsSteel  = color.RGBA{150, 155, 170, 255}  // katana steel
	swordsSteelD = color.RGBA{80, 85, 100, 255}
	pentRed      = color.RGBA{190, 60, 40, 255}    // vermillion coin
	pentRedDim   = color.RGBA{120, 35, 25, 255}
)

type suitColors struct {
	primary, dim color.RGBA
}

var suitPalette = []suitColors{
	{cupsBlue, cupsBlueDim},
	{wandsGreen, wandsGreenD},
	{swordsSteel, swordsSteelD},
	{pentRed, pentRedDim},
}

var suitFileNames = []string{"cups", "wands", "swords", "pentacles"}

// ========== backgrounds ==========

func ukiyoeBG() *HiResCanvas {
	cv := newCanvas()
	cv.fillGradient(ricePaper, riceDark)
	return cv
}

// subtle rice paper texture
func (c *HiResCanvas) paperTexture() {
	// Very subtle noise-like dots to simulate paper grain
	for y := 0; y < hiH; y += 3 {
		for x := 0; x < hiW; x += 3 {
			// Simple deterministic "noise"
			v := math.Sin(float64(x)*0.7+float64(y)*1.3) * 0.5
			if v > 0.3 {
				c.blend(x, y, sumi, 0.02)
			}
		}
	}
}

// ========== number glyphs ==========

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
	for i, ch := range bmp {
		if ch == 'X' {
			x := sx + float64(i%3)*scale
			y := sy + float64(i/3)*scale
			c.filledRect(x, y, scale, scale, col)
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

// ========== suit pip drawings ==========

func (c *HiResCanvas) drawCup(cx, cy, size float64, col color.RGBA) {
	// Tea bowl / sake cup
	c.arc(cx, cy, size*0.35, 0, math.Pi, 2.5, col)
	c.line(cx-size*0.35, cy, cx+size*0.35, cy, 2, col)
	// Steam wisps
	steamCol := color.RGBA{col.R/2 + 80, col.G/2 + 80, col.B/2 + 80, 255}
	c.bezier(cx-5, cy-size*0.2, cx-10, cy-size*0.5, cx+5, cy-size*0.5, cx, cy-size*0.7, 1.0, steamCol)
	c.bezier(cx+5, cy-size*0.15, cx+10, cy-size*0.45, cx-5, cy-size*0.5, cx+3, cy-size*0.65, 1.0, steamCol)
}

func (c *HiResCanvas) drawWand(cx, cy, size float64, col color.RGBA) {
	// Bamboo stick
	c.line(cx, cy-size*0.55, cx, cy+size*0.55, 2.5, col)
	// Nodes
	nodeCol := color.RGBA{
		clampU8(float64(col.R) * 0.7),
		clampU8(float64(col.G) * 0.7),
		clampU8(float64(col.B) * 0.7),
		255,
	}
	c.line(cx-3, cy-size*0.15, cx+3, cy-size*0.15, 1.5, nodeCol)
	c.line(cx-3, cy+size*0.2, cx+3, cy+size*0.2, 1.5, nodeCol)
	// Leaf at top
	c.bambooLeaf(cx, cy-size*0.5, -0.5, size*0.4, col)
}

func (c *HiResCanvas) drawSword(cx, cy, size float64, col color.RGBA) {
	// Katana
	// Curved blade
	c.bezier(cx, cy-size*0.55, cx+3, cy-size*0.3, cx+3, cy+size*0.1, cx, cy+size*0.35, 2.5, col)
	// Tsuba (guard) — small circle
	c.circle(cx, cy+size*0.1, size*0.08, 2, sumi)
	// Handle wrap
	c.line(cx, cy+size*0.15, cx, cy+size*0.45, 3, sumi)
	// Tip glow
	c.glow(cx, cy-size*0.55, size*0.08, moonWhite)
}

func (c *HiResCanvas) drawPentacle(cx, cy, size float64, col color.RGBA) {
	// Mon (family crest) circle with inner design
	c.circle(cx, cy, size*0.4, 2.5, col)
	// Inner circle
	c.circle(cx, cy, size*0.25, 1.5, col)
	// Cross pattern inside
	c.line(cx-size*0.25, cy, cx+size*0.25, cy, 1.5, col)
	c.line(cx, cy-size*0.25, cx, cy+size*0.25, 1.5, col)
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
	cv.fillGradient(indigoDark, indigo)
	cv.paperTexture()

	// Woodblock border
	cv.woodblockBorder(8, 2.5, goldLeaf)

	// Central mon/crest
	cv.circle(120, 200, 50, 3, goldLeaf)
	cv.circle(120, 200, 35, 2, goldLeaf)
	// Inner tomoe pattern (triple comma)
	for i := 0; i < 3; i++ {
		a := float64(i) * 2 * math.Pi / 3
		px := 120 + 20*math.Cos(a)
		py := 200 + 20*math.Sin(a)
		cv.filledCircle(px, py, 8, goldLeaf)
		// Tail of each tomoe
		ta := a + math.Pi/2
		cv.arc(px, py, 12, ta-0.5, ta+1.5, 2, goldLeaf)
	}
	cv.filledCircle(120, 200, 8, indigoDark)

	// Wave pattern along bottom
	for x := 20.0; x < 230; x += 35 {
		cv.wave(x, 360, 15, waveBlue)
	}
	// Wave pattern along top
	for x := 35.0; x < 230; x += 35 {
		cv.wave(x, 50, 12, waveBlue)
	}

	// Cherry blossoms scattered
	cv.cherryBlossom(40, 100, 10, sakuraPink)
	cv.cherryBlossom(200, 100, 8, sakuraPink)
	cv.cherryBlossom(50, 300, 8, sakuraPink)
	cv.cherryBlossom(190, 300, 10, sakuraPink)

	return cv
}

func genPip(si, num int) *HiResCanvas {
	cv := ukiyoeBG()
	cv.paperTexture()
	sc := suitPalette[si]

	cv.woodblockBorder(6, 2.0, sumi)

	// Corner numbers in sumi ink
	cv.drawNum(15, 15, num, sumi, 5)
	cv.drawNum(185, 350, num, sumi, 5)

	// Suit-specific background accents
	switch si {
	case 0: // Cups — wave pattern at bottom
		for x := 20.0; x < 230; x += 40 {
			cv.wave(x, 370, 12, color.RGBA{sc.dim.R, sc.dim.G, sc.dim.B, 255})
		}
	case 1: // Wands — bamboo along sides
		cv.bamboo(25, 30, 370, bambooD)
		cv.bamboo(215, 30, 370, bambooD)
		cv.bambooLeaf(30, 80, 0.5, 20, bambooD)
		cv.bambooLeaf(210, 120, -0.5, 20, bambooD)
	case 2: // Swords — subtle clouds
		cv.cloud(40, 50, 20, color.RGBA{180, 180, 190, 255})
		cv.cloud(200, 360, 18, color.RGBA{180, 180, 190, 255})
	case 3: // Pentacles — cherry blossoms
		cv.cherryBlossom(30, 50, 8, sakuraPink)
		cv.cherryBlossom(210, 50, 7, sakuraPink)
		cv.cherryBlossom(30, 360, 7, sakuraPink)
		cv.cherryBlossom(210, 360, 8, sakuraPink)
	}

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
	cv := ukiyoeBG()
	cv.paperTexture()
	sc := suitPalette[si]

	cv.woodblockBorder(5, 2.0, sumi)

	switch rank {
	case 11: // Page — young attendant
		// Head
		cv.filledCircle(120, 100, 22, skin)
		// Hair (black, traditional)
		cv.filledCircle(120, 88, 20, sumi)
		cv.filledRect(102, 88, 36, 8, sumi)
		// Kimono body
		for y := 125.0; y < 280; y++ {
			t := (y - 125) / 155
			w := 25 + t*30
			cv.filledRect(120-w, y, w*2, 1, sc.primary)
		}
		// Obi (belt)
		cv.filledRect(88, 190, 64, 12, sc.dim)
		// Legs/hakama
		cv.filledRect(95, 280, 20, 50, sc.dim)
		cv.filledRect(125, 280, 20, 50, sc.dim)
		// Suit symbol
		cv.drawSuitPip(si, 120, 220, 25)
		// Cherry blossoms
		cv.cherryBlossom(45, 80, 8, sakuraPink)
		cv.cherryBlossom(195, 80, 8, sakuraPink)

	case 12: // Knight — samurai warrior
		// Kabuto (helmet)
		cv.arc(105, 72, 28, math.Pi, 2*math.Pi, 3, sc.primary)
		cv.filledRect(78, 72, 54, 6, sc.primary)
		// Crest on helmet
		cv.line(105, 45, 105, 68, 2.5, goldLeaf)
		cv.filledCircle(105, 42, 5, goldLeaf)
		// Face
		cv.filledCircle(105, 90, 18, skin)
		// Mempo (face guard)
		cv.arc(105, 95, 14, 0.2, math.Pi-0.2, 2, sumi)
		// Armor body
		cv.filledRect(80, 112, 50, 90, sc.primary)
		cv.line(80, 112, 130, 112, 2, goldLeaf)
		cv.line(80, 140, 130, 140, 1.5, goldLeaf)
		cv.line(80, 168, 130, 168, 1.5, goldLeaf)
		// Katana extended
		cv.bezier(135, 140, 170, 130, 200, 110, 215, 90, 3, swordsSteelD)
		cv.glow(215, 88, 6, moonWhite)
		// Legs
		cv.filledRect(85, 202, 18, 60, sc.dim)
		cv.filledRect(108, 202, 18, 60, sc.dim)
		// Suit symbol
		cv.drawSuitPip(si, 180, 260, 25)

	case 13: // Queen — noble lady / empress
		// Elaborate hair (traditional nihongami)
		cv.filledCircle(120, 75, 28, sumi)
		cv.filledCircle(120, 60, 18, sumi)
		// Kanzashi (hair ornament)
		cv.filledCircle(140, 55, 5, goldLeaf)
		cv.filledCircle(145, 50, 4, vermillion)
		cv.cherryBlossom(100, 52, 6, sakuraPink)
		// Face
		cv.filledCircle(120, 82, 18, skin)
		// Elaborate junihitoe (layered robes)
		layers := []color.RGBA{sc.primary, sakuraPink, sc.dim, vermillion, sc.primary}
		for i, col := range layers {
			offset := float64(i) * 5
			for y := 105 + offset; y < 320; y++ {
				t := (y - 105 - offset) / (215 - offset)
				w := 28 + t*65 + offset*2
				dimCol := color.RGBA{
					clampU8(float64(col.R) * (1 - float64(i)*0.05)),
					clampU8(float64(col.G) * (1 - float64(i)*0.05)),
					clampU8(float64(col.B) * (1 - float64(i)*0.05)),
					255,
				}
				cv.filledRect(120-w, y, w*2, 1, dimCol)
			}
		}
		// Fan in hand
		cv.arc(160, 200, 25, -0.5, 1.2, 2, goldLeaf)
		cv.line(160, 200, 175, 218, 2, sumi)
		// Suit symbol
		cv.drawSuitPip(si, 65, 200, 25)

	case 14: // King — daimyo / shogun
		// Eboshi (court hat)
		cv.filledRect(105, 35, 30, 40, sumi)
		cv.filledRect(100, 70, 40, 8, sumi)
		// Face
		cv.filledCircle(120, 85, 22, skin)
		// Beard
		cv.filledCircle(120, 100, 10, sumiGray)
		// Broad formal robes (kariginu)
		cv.filledRect(70, 110, 100, 8, goldLeaf) // collar
		for y := 118.0; y < 300; y++ {
			t := (y - 118) / 182
			w := 40 + t*55
			cv.filledRect(120-w, y, w*2, 1, sc.primary)
		}
		// Mon (crest) on chest
		cv.circle(120, 170, 15, 2, goldLeaf)
		cv.drawSuitPip(si, 120, 170, 22)
		// Shaku (flat scepter)
		cv.filledRect(165, 130, 6, 80, ricePaper)
		cv.line(165, 130, 171, 130, 1.5, sumi)
		// Legs
		cv.filledRect(90, 300, 25, 45, sc.dim)
		cv.filledRect(125, 300, 25, 45, sc.dim)
		// Throne/platform
		cv.filledRect(50, 345, 140, 8, sumi)
	}

	// Corner rank letter
	rankLetters := map[int]string{
		11: ".X." + "X.X" + "XXX" + "X.." + "X..",
		12: "X.X" + "X.X" + "XX." + "X.X" + "X.X",
		13: "XX." + "X.X" + "X.X" + "X.X" + "XX.",
		14: "X.X" + "X.X" + "XX." + "X.X" + "X.X",
	}
	if bmp, ok := rankLetters[rank]; ok {
		for i, ch := range bmp {
			if ch == 'X' {
				x := 15 + float64(i%3)*5
				y := 15 + float64(i/3)*5
				cv.filledRect(x, y, 5, 5, sumi)
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
	{sakuraPink, waveBlue},    // 0  Fool
	{vermillion, goldLeaf},    // 1  Magician
	{indigoMid, moonWhite},    // 2  High Priestess
	{sakuraPink, bambooGrn},   // 3  Empress
	{vermillion, goldLeaf},    // 4  Emperor
	{goldLeaf, vermillion},    // 5  Hierophant
	{sakuraPink, vermillion},  // 6  Lovers
	{indigo, goldLeaf},        // 7  Chariot
	{vermillion, goldLeaf},    // 8  Strength
	{sumiGray, goldLeaf},      // 9  Hermit
	{goldLeaf, vermillion},    // 10 Wheel
	{indigo, goldLeaf},        // 11 Justice
	{waveBlue, vermillion},    // 12 Hanged Man
	{moonWhite, vermillion},   // 13 Death
	{waveBlue, goldLeaf},      // 14 Temperance
	{vermillion, sumi},        // 15 Devil
	{vermillion, sunset},      // 16 Tower
	{moonWhite, waveBlue},     // 17 Star
	{moonWhite, indigoMid},    // 18 Moon
	{goldLeaf, vermillion},    // 19 Sun
	{moonWhite, goldLeaf},     // 20 Judgement
	{bambooGrn, goldLeaf},     // 21 World
}

func genMajor(num int) *HiResCanvas {
	cv := ukiyoeBG()
	cv.paperTexture()
	sc := majorSchemes[num]
	p, s := sc.primary, sc.secondary

	cv.woodblockBorder(5, 2.0, sumi)
	cv.drawNum(15, 15, num, sumi, 5)

	switch num {
	case 0: // Fool — wanderer on cliff, cherry blossoms, wind
		// Cliff path
		cv.bezier(0, 280, 80, 270, 140, 290, 170, 280, 3, sumiGray)
		for y := 280; y < 400; y++ {
			t := float64(y-280) / 120
			col := lerpColor(sumiGray, riceDark, t)
			for x := 0; x < 170; x++ {
				cv.blend(x, y, col, 0.5)
			}
		}
		// Figure (wanderer with bundle on stick)
		cv.filledCircle(100, 175, 20, skin)
		cv.filledCircle(100, 162, 18, sumi) // hair
		// Kimono
		for y := 200.0; y < 280; y++ {
			t := (y - 200) / 80
			w := 18 + t*15
			cv.filledRect(100-w, y, w*2, 1, p)
		}
		// Staff with bundle
		cv.line(130, 140, 130, 280, 2.5, sumiGray)
		cv.filledCircle(130, 135, 12, s)
		// Falling cherry blossoms
		cv.cherryBlossom(50, 60, 10, sakuraPink)
		cv.cherryBlossom(180, 80, 8, sakuraPink)
		cv.cherryBlossom(160, 140, 6, sakuraPink)
		cv.cherryBlossom(40, 130, 7, sakuraPink)
		// Void/sea below
		for x := 170.0; x < 240; x += 25 {
			cv.wave(x, 340, 12, waveBlue)
		}

	case 1: // Magician — onmyoji with elements on table
		// Torii gate frame
		cv.torii(120, 40, 120, 80, vermillion)
		// Figure
		cv.filledCircle(120, 130, 20, skin)
		cv.filledCircle(120, 118, 18, sumi)
		// Robes
		for y := 155.0; y < 250; y++ {
			t := (y - 155) / 95
			w := 20 + t*25
			cv.filledRect(120-w, y, w*2, 1, p)
		}
		// Arms with ofuda (paper talismans)
		cv.line(95, 175, 55, 160, 2, skin)
		cv.filledRect(45, 150, 12, 18, moonWhite)
		cv.line(145, 175, 185, 160, 2, skin)
		cv.glow(185, 160, 10, s)
		// Table with four elements
		cv.filledRect(40, 270, 160, 6, sumiGray)
		cv.drawSuitPip(0, 60, 300, 22)
		cv.drawSuitPip(1, 100, 300, 22)
		cv.drawSuitPip(2, 140, 300, 22)
		cv.drawSuitPip(3, 180, 300, 22)
		// Spiritual energy
		cv.glow(120, 130, 25, goldLeaf)

	case 2: // High Priestess — miko (shrine maiden), moon, veil
		// Moon
		cv.filledCircle(120, 55, 28, moonWhite)
		cv.filledCircle(134, 48, 28, ricePaper)
		cv.glow(110, 55, 22, p)
		// Bamboo grove (veil/curtain)
		for x := 20.0; x < 50; x += 12 {
			cv.bamboo(x, 80, 380, bambooD)
		}
		for x := 195.0; x < 225; x += 12 {
			cv.bamboo(x, 80, 380, bambooD)
		}
		// Bamboo leaves
		cv.bambooLeaf(35, 100, 0.7, 25, bambooGrn)
		cv.bambooLeaf(205, 120, -0.7, 25, bambooGrn)
		// Figure (miko)
		cv.filledCircle(120, 145, 20, skin)
		cv.filledCircle(120, 132, 18, sumi) // hair
		// White haori + red hakama
		cv.filledRect(100, 170, 40, 60, moonWhite)
		for y := 230.0; y < 340; y++ {
			t := (y - 230) / 110
			w := 20 + t*30
			cv.filledRect(120-w, y, w*2, 1, vermillion)
		}
		// Scroll
		cv.filledRect(105, 350, 30, 15, ricePaper)
		cv.line(105, 350, 135, 350, 1.5, sumi)
		cv.line(105, 365, 135, 365, 1.5, sumi)

	case 3: // Empress — court lady in layered robes, garden
		// Elaborate hair
		cv.filledCircle(120, 60, 25, sumi)
		cv.filledCircle(120, 48, 16, sumi)
		// Kanzashi ornaments
		cv.cherryBlossom(100, 42, 6, sakuraPink)
		cv.filledCircle(140, 45, 4, goldLeaf)
		// Face
		cv.filledCircle(120, 72, 18, skin)
		// Layered juni-hitoe
		robeColors := []color.RGBA{p, sakuraDark, bambooGrn, sakuraPink, p}
		for i, col := range robeColors {
			off := float64(i) * 4
			for y := 95 + off; y < 290; y++ {
				t := (y - 95 - off) / (195 - off)
				w := 22 + t*70 + off*2
				cv.filledRect(120-w, y, w*2, 1, col)
			}
		}
		// Garden below
		for x := 0.0; x < 240; x += 20 {
			cv.cherryBlossom(x, 340+10*math.Sin(x*0.1), 7, sakuraPink)
		}
		// Bamboo and flowers
		cv.bamboo(20, 300, 395, bambooGrn)
		cv.bamboo(220, 310, 395, bambooGrn)

	case 4: // Emperor — shogun on throne, armor
		// Throne/platform
		cv.filledRect(30, 300, 180, 10, sumi)
		cv.filledRect(35, 310, 170, 80, sumiGray)
		// Pillars
		cv.filledRect(35, 50, 15, 260, vermDark)
		cv.filledRect(190, 50, 15, 260, vermDark)
		// Mon crest above
		cv.circle(120, 40, 20, 2, goldLeaf)
		// Head with kabuto
		cv.filledRect(100, 55, 40, 30, sumi)
		cv.line(100, 55, 140, 55, 2, goldLeaf)
		cv.line(120, 40, 120, 55, 2, goldLeaf)
		cv.filledCircle(120, 95, 20, skin)
		// Armor
		cv.filledRect(85, 120, 70, 100, p)
		cv.line(85, 120, 155, 120, 2, goldLeaf)
		cv.line(85, 150, 155, 150, 1.5, goldLeaf)
		cv.line(85, 180, 155, 180, 1.5, goldLeaf)
		// Katana at side
		cv.bezier(60, 130, 55, 200, 50, 260, 55, 300, 2.5, swordsSteelD)
		// Legs
		cv.filledRect(95, 220, 22, 80, vermDark)
		cv.filledRect(125, 220, 22, 80, vermDark)
		// Imperial fan
		cv.arc(170, 170, 25, -0.8, 0.8, 2, goldLeaf)

	case 5: // Hierophant — Buddhist monk, temple
		// Temple roof hint
		cv.bezier(10, 60, 60, 30, 180, 30, 230, 60, 3, sumi)
		cv.bezier(30, 55, 80, 35, 160, 35, 210, 55, 2, vermillion)
		// Pillars
		cv.filledRect(30, 60, 15, 310, vermDark)
		cv.filledRect(195, 60, 15, 310, vermDark)
		// Figure (monk in saffron)
		cv.filledCircle(120, 130, 22, skin)
		// Shaved head
		cv.filledCircle(120, 120, 18, color.RGBA{190, 165, 130, 255})
		// Robes (saffron/gold)
		for y := 155.0; y < 300; y++ {
			t := (y - 155) / 145
			w := 25 + t*35
			cv.filledRect(120-w, y, w*2, 1, lerpColor(goldLeaf, sunset, t*0.3))
		}
		// Prayer beads
		cv.arc(120, 200, 25, 0, math.Pi, 2, sumi)
		for a := 0.0; a < math.Pi; a += 0.4 {
			bx := 120 + 25*math.Cos(a)
			by := 200 + 25*math.Sin(a)
			cv.filledCircle(bx, by, 3, sumi)
		}
		// Blessing gesture
		cv.glow(120, 160, 15, goldLeaf)
		// Two disciples
		cv.filledCircle(65, 340, 12, skin)
		cv.filledRect(55, 355, 20, 30, sumiGray)
		cv.filledCircle(175, 340, 12, skin)
		cv.filledRect(165, 355, 20, 30, sumiGray)

	case 6: // Lovers — two figures under cherry tree
		// Cherry tree
		cv.line(120, 0, 120, 120, 4, sumiGray)
		cv.bezier(120, 40, 60, 30, 30, 50, 20, 40, 3, sumiGray)
		cv.bezier(120, 40, 180, 30, 210, 50, 220, 40, 3, sumiGray)
		cv.bezier(120, 70, 70, 60, 50, 75, 40, 65, 2, sumiGray)
		cv.bezier(120, 70, 170, 60, 190, 75, 200, 65, 2, sumiGray)
		// Blossoms on tree
		for _, pos := range [][2]float64{{30, 35}, {70, 25}, {170, 25}, {210, 35}, {50, 65}, {190, 60}, {90, 45}, {150, 45}} {
			cv.cherryBlossom(pos[0], pos[1], 8, p)
		}
		// Falling petals
		cv.cherryBlossom(80, 100, 5, sakuraPink)
		cv.cherryBlossom(160, 130, 4, sakuraPink)
		// Two figures
		// Left figure
		cv.filledCircle(80, 200, 18, skin)
		cv.filledCircle(80, 188, 16, sumi)
		for y := 220.0; y < 320; y++ {
			t := (y - 220) / 100
			w := 15 + t*18
			cv.filledRect(80-w, y, w*2, 1, waveBlue)
		}
		// Right figure
		cv.filledCircle(160, 200, 18, skin)
		cv.filledCircle(160, 188, 16, sumi)
		for y := 220.0; y < 320; y++ {
			t := (y - 220) / 100
			w := 15 + t*18
			cv.filledRect(160-w, y, w*2, 1, sakuraPink)
		}
		// Hands reaching
		cv.line(95, 240, 145, 240, 2, skin)
		// Ground
		cv.line(10, 340, 230, 340, 2, sumiGray)

	case 7: // Chariot — ox cart (gissha), determination
		// Sky
		cv.cloud(50, 40, 25, color.RGBA{200, 195, 185, 255})
		cv.cloud(180, 50, 20, color.RGBA{200, 195, 185, 255})
		// Figure on cart
		cv.filledCircle(120, 100, 20, skin)
		cv.filledCircle(120, 88, 18, sumi)
		cv.filledRect(100, 125, 40, 50, p)
		cv.line(100, 125, 140, 125, 2, goldLeaf)
		// Cart body
		cv.filledRect(50, 180, 140, 60, sumiGray)
		cv.line(50, 180, 190, 180, 2.5, sumi)
		cv.line(50, 240, 190, 240, 2.5, sumi)
		// Mon on cart
		cv.circle(120, 210, 18, 2, goldLeaf)
		// Wheels
		cv.circle(65, 270, 25, 3, sumi)
		cv.filledCircle(65, 270, 6, sumi)
		cv.circle(175, 270, 25, 3, sumi)
		cv.filledCircle(175, 270, 6, sumi)
		for i := 0; i < 6; i++ {
			a := float64(i) * math.Pi / 3
			cv.line(65+25*math.Cos(a), 270+25*math.Sin(a), 65, 270, 1.5, sumi)
			cv.line(175+25*math.Cos(a), 270+25*math.Sin(a), 175, 270, 1.5, sumi)
		}
		// Ox
		cv.filledCircle(120, 330, 20, sumiGray)
		cv.filledRect(100, 340, 40, 20, sumiGray)
		cv.filledRect(103, 360, 10, 15, sumi)
		cv.filledRect(127, 360, 10, 15, sumi)
		// Horns
		cv.line(110, 318, 100, 305, 2, sumi)
		cv.line(130, 318, 140, 305, 2, sumi)

	case 8: // Strength — figure taming tiger/lion
		// Bamboo frame
		cv.bamboo(20, 10, 390, bambooGrn)
		cv.bamboo(220, 10, 390, bambooGrn)
		cv.bambooLeaf(25, 30, 0.6, 30, bambooGrn)
		cv.bambooLeaf(215, 50, -0.6, 30, bambooGrn)
		// Figure
		cv.filledCircle(90, 120, 20, skin)
		cv.filledCircle(90, 108, 18, sumi)
		for y := 145.0; y < 260; y++ {
			t := (y - 145) / 115
			w := 15 + t*20
			cv.filledRect(90-w, y, w*2, 1, moonWhite)
		}
		// Hand reaching to tiger
		cv.line(110, 170, 145, 200, 2.5, skin)
		// Tiger
		cv.filledCircle(160, 210, 28, sunset)
		// Stripes
		for i := 0; i < 4; i++ {
			y := 198 + float64(i)*8
			cv.line(145, y, 150, y+4, 2, sumi)
			cv.line(170, y, 175, y+4, 2, sumi)
		}
		// Tiger body
		cv.filledRect(135, 235, 55, 30, sunset)
		// Legs
		cv.filledRect(138, 265, 10, 25, sunset)
		cv.filledRect(155, 265, 10, 25, sunset)
		cv.filledRect(172, 265, 10, 25, sunset)
		// Tiger tail
		cv.bezier(190, 240, 200, 230, 210, 235, 215, 225, 2, sunset)
		// Tiger eyes
		cv.filledCircle(152, 205, 3, goldLeaf)
		cv.filledCircle(168, 205, 3, goldLeaf)

	case 9: // Hermit — mountain monk, lantern
		// Mountain Fuji in background
		cv.mountain(120, 30, 200, 180, color.RGBA{80, 85, 100, 255})
		// Path
		cv.bezier(120, 350, 100, 300, 140, 250, 120, 200, 2, sumiGray)
		// Figure
		cv.filledCircle(110, 180, 18, skin)
		// Straw hat (kasa)
		cv.arc(110, 170, 25, math.Pi, 2*math.Pi, 3, sumiGray)
		cv.filledRect(86, 170, 48, 4, sumiGray)
		// Robes
		cv.filledRect(92, 200, 36, 80, p)
		cv.filledRect(95, 280, 14, 30, sumiGray)
		cv.filledRect(113, 280, 14, 30, sumiGray)
		// Staff
		cv.line(75, 170, 75, 310, 2.5, sumiGray)
		// Lantern (chochin)
		cv.filledCircle(155, 165, 15, vermillion)
		cv.glow(155, 165, 20, sunset)
		cv.glow(155, 165, 10, goldLeaf)
		cv.line(155, 150, 155, 180, 1, sumi) // frame
		// Stars
		cv.glow(60, 30, 5, moonWhite)
		cv.glow(190, 20, 4, moonWhite)

	case 10: // Wheel of Fortune — dharma wheel
		// Large wheel
		cv.circle(120, 200, 75, 3, goldLeaf)
		cv.circle(120, 200, 60, 2, s)
		cv.circle(120, 200, 25, 2, goldLeaf)
		// Eight spokes (dharma wheel)
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			cv.line(120+25*math.Cos(a), 200+25*math.Sin(a),
				120+75*math.Cos(a), 200+75*math.Sin(a), 2, goldLeaf)
			// Spoke tips curl
			tipX := 120 + 75*math.Cos(a)
			tipY := 200 + 75*math.Sin(a)
			cv.arc(tipX, tipY, 8, a-1, a+1, 1.5, goldLeaf)
		}
		cv.filledCircle(120, 200, 12, goldLeaf)
		cv.glow(120, 200, 15, goldLeaf)
		// Four creatures in corners (Japanese zodiac-inspired)
		// Dragon (top-left)
		cv.filledCircle(30, 40, 12, vermillion)
		cv.glow(30, 40, 8, vermillion)
		// Tiger (top-right)
		cv.filledCircle(210, 40, 12, sunset)
		cv.glow(210, 40, 8, sunset)
		// Turtle (bottom-left)
		cv.filledCircle(30, 360, 12, bambooGrn)
		cv.glow(30, 360, 8, bambooGrn)
		// Phoenix (bottom-right)
		cv.filledCircle(210, 360, 12, goldLeaf)
		cv.glow(210, 360, 8, goldLeaf)
		// Clouds
		cv.cloud(60, 80, 18, color.RGBA{200, 195, 185, 255})
		cv.cloud(180, 320, 18, color.RGBA{200, 195, 185, 255})

	case 11: // Justice — magistrate with scales
		// Pillars
		cv.filledRect(25, 60, 15, 300, vermDark)
		cv.filledRect(200, 60, 15, 300, vermDark)
		// Beam
		cv.line(25, 60, 215, 60, 3, sumi)
		// Figure
		cv.filledCircle(120, 110, 22, skin)
		cv.filledCircle(120, 98, 18, sumi)
		// Eboshi hat
		cv.filledRect(108, 78, 24, 18, sumi)
		// Robes
		for y := 135.0; y < 280; y++ {
			t := (y - 135) / 145
			w := 22 + t*35
			cv.filledRect(120-w, y, w*2, 1, p)
		}
		// Scales bar
		cv.line(50, 160, 190, 160, 2.5, goldLeaf)
		cv.line(120, 140, 120, 160, 2, goldLeaf)
		// Scale pans
		cv.arc(65, 195, 18, 0, math.Pi, 2, goldLeaf)
		cv.arc(175, 195, 18, 0, math.Pi, 2, goldLeaf)
		cv.line(65, 160, 65, 195, 1.5, goldLeaf)
		cv.line(175, 160, 175, 195, 1.5, goldLeaf)
		// Katana (justice)
		cv.bezier(120, 280, 123, 310, 123, 340, 120, 360, 2.5, swordsSteelD)
		cv.glow(120, 280, 5, moonWhite)

	case 12: // Hanged Man — figure hanging from torii, enlightened
		// Torii gate
		cv.torii(120, 30, 140, 100, vermillion)
		// Rope
		cv.line(120, 130, 120, 175, 2, sumiGray)
		// Inverted figure (feet tied to beam)
		cv.filledCircle(120, 145, 6, skin) // ankle
		// Legs
		cv.line(120, 150, 110, 200, 3, p)
		cv.line(120, 150, 130, 200, 3, p)
		// Body
		cv.filledRect(106, 200, 28, 55, s)
		// Arms hanging behind
		cv.line(106, 225, 85, 260, 2, skin)
		cv.line(134, 225, 155, 260, 2, skin)
		// Head at bottom with halo
		cv.filledCircle(120, 278, 20, skin)
		cv.filledCircle(120, 268, 16, sumi) // hair
		cv.circle(120, 278, 30, 2.5, goldLeaf)
		cv.glow(120, 278, 30, goldLeaf)
		// Cherry blossoms falling
		cv.cherryBlossom(50, 200, 7, sakuraPink)
		cv.cherryBlossom(190, 220, 6, sakuraPink)
		cv.cherryBlossom(70, 340, 5, sakuraPink)
		cv.cherryBlossom(170, 350, 6, sakuraPink)

	case 13: // Death — skeletal figure, chrysanthemum (rebirth)
		// Skull face
		cv.filledCircle(120, 95, 32, moonWhite)
		cv.filledCircle(108, 88, 8, sumi)
		cv.filledCircle(132, 88, 8, sumi)
		cv.filledCircle(120, 102, 4, sumi)
		cv.filledRect(108, 112, 24, 4, sumi)
		for x := 108.0; x < 132; x += 6 {
			cv.line(x, 112, x, 116, 1, moonWhite)
		}
		cv.glow(120, 95, 20, p)
		// Scythe (naginata style)
		cv.line(170, 60, 80, 350, 3, sumiGray)
		cv.arc(135, 80, 45, math.Pi+0.5, 2*math.Pi-0.2, 3, swordsSteelD)
		cv.glow(140, 55, 10, moonWhite)
		// Skeletal body
		cv.line(120, 130, 120, 235, 2.5, moonWhite)
		for i := 0; i < 4; i++ {
			y := 150 + float64(i)*18
			cv.arc(120, y, 18, 0, math.Pi, 1.5, moonWhite)
		}
		cv.line(120, 235, 100, 305, 2, moonWhite)
		cv.line(120, 235, 140, 305, 2, moonWhite)
		// Chrysanthemum (rebirth)
		for i := 0; i < 12; i++ {
			a := float64(i) * math.Pi / 6
			px := 120 + 18*math.Cos(a)
			py := 360 + 18*math.Sin(a)
			cv.filledCircle(px, py, 6, s)
		}
		cv.filledCircle(120, 360, 8, goldLeaf)
		cv.glow(120, 360, 12, s)

	case 14: // Temperance — angel-like figure pouring water between vessels
		// Wings (crane-like)
		cv.bezier(100, 95, 50, 50, 20, 55, 25, 85, 3, moonWhite)
		cv.bezier(140, 95, 190, 50, 220, 55, 215, 85, 3, moonWhite)
		for i := 0; i < 4; i++ {
			t := float64(i) * 0.12
			cv.bezier(100, 95, 50+t*35, 50+t*15, 20+t*25, 55+t*15, 25+t*15, 85, 1.5, riceDark)
			cv.bezier(140, 95, 190-t*35, 50+t*15, 220-t*25, 55+t*15, 215-t*15, 85, 1.5, riceDark)
		}
		// Halo
		cv.circle(120, 62, 16, 2, goldLeaf)
		cv.glow(120, 62, 10, goldLeaf)
		// Head
		cv.filledCircle(120, 82, 20, skin)
		cv.filledCircle(120, 72, 16, sumi)
		// Robes
		for y := 105.0; y < 280; y++ {
			t := (y - 105) / 175
			w := 20 + t*40
			cv.filledRect(120-w, y, w*2, 1, lerpColor(moonWhite, p, t*0.4))
		}
		// Two vessels
		cv.arc(65, 185, 12, 0, math.Pi, 2, goldLeaf)
		cv.line(53, 185, 77, 185, 2, goldLeaf)
		cv.arc(175, 215, 12, 0, math.Pi, 2, goldLeaf)
		cv.line(163, 215, 187, 215, 2, goldLeaf)
		// Water flowing
		cv.bezier(75, 190, 100, 195, 150, 210, 165, 218, 2.5, p)
		cv.glow(120, 205, 12, p)
		// Water/pond
		for x := 20.0; x < 220; x += 30 {
			cv.wave(x, 350, 10, waveLight)
		}

	case 15: // Devil — oni (demon), chains
		// Oni horns
		cv.line(95, 75, 75, 35, 4, p)
		cv.line(145, 75, 165, 35, 4, p)
		// Oni face
		cv.filledCircle(120, 90, 30, vermDark)
		cv.glow(120, 90, 18, p)
		// Glowing eyes
		cv.glow(108, 85, 6, goldLeaf)
		cv.glow(132, 85, 6, goldLeaf)
		cv.filledCircle(108, 85, 3, moonWhite)
		cv.filledCircle(132, 85, 3, moonWhite)
		// Fangs
		cv.line(112, 108, 112, 118, 2, moonWhite)
		cv.line(128, 108, 128, 118, 2, moonWhite)
		// Body
		cv.filledRect(90, 125, 60, 90, vermDark)
		// Tiger-skin loincloth
		cv.filledRect(85, 215, 70, 25, sunset)
		for x := 90.0; x < 155; x += 15 {
			cv.line(x, 218, x+5, 235, 2, sumi)
		}
		// Iron club (kanabo)
		cv.line(170, 100, 185, 280, 4, sumiGray)
		// Studs on club
		for y := 120.0; y < 270; y += 20 {
			cv.filledCircle(178, y, 3, sumi)
		}
		// Chains
		for y := 240.0; y < 310; y += 8 {
			cv.filledCircle(85, y, 3, sumiGray)
			cv.filledCircle(155, y, 3, sumiGray)
		}
		// Captives
		cv.filledCircle(55, 340, 12, skin)
		cv.filledRect(47, 355, 16, 25, sumiGray)
		cv.filledCircle(185, 340, 12, skin)
		cv.filledRect(177, 355, 16, 25, sumiGray)

	case 16: // Tower — castle tower crumbling, lightning
		// Castle (tenshu)
		cv.filledRect(80, 90, 80, 220, sumiGray)
		// Floors
		for y := 90.0; y < 310; y += 55 {
			cv.bezier(70, y, 80, y-12, 160, y-12, 170, y, 2.5, sumi)
		}
		// Windows
		for y := 120.0; y < 290; y += 55 {
			cv.filledRect(100, y, 12, 18, sumi)
			cv.filledRect(128, y, 12, 18, sumi)
		}
		// Top crumbling
		cv.filledRect(65, 60, 15, 20, sumiGray)
		cv.filledRect(165, 55, 12, 18, sumiGray)
		// Lightning
		cv.line(155, 10, 140, 40, 4, goldLeaf)
		cv.line(140, 40, 160, 55, 4, goldLeaf)
		cv.line(160, 55, 130, 80, 5, goldLeaf)
		cv.glow(145, 45, 18, goldLeaf)
		cv.glow(135, 75, 22, moonWhite)
		// Fire
		for x := 78.0; x < 162; x += 3 {
			h := 20 + 15*math.Sin(x*0.25)
			for y := 310 - h; y < 310; y++ {
				t := (310 - y) / h
				col := lerpColor(vermillion, sunset, t)
				cv.blend(int(x), int(y), col, 0.7)
			}
		}
		// Falling figures
		cv.filledCircle(40, 200, 10, skin)
		cv.filledRect(33, 213, 14, 20, p)
		cv.filledCircle(205, 230, 10, skin)
		cv.filledRect(198, 243, 14, 20, p)

	case 17: // Star — large star over water, figure pouring
		// Night sky
		cv.fillGradient(indigoDark, indigo)
		cv.paperTexture()
		cv.woodblockBorder(5, 2.0, goldLeaf)
		cv.drawNum(15, 15, num, goldLeaf, 5)
		// Large star
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			for r := 0.0; r < 45; r++ {
				px := 120 + r*math.Cos(a)
				py := 85 + r*math.Sin(a)
				cv.blend(int(px), int(py), p, 0.8-r/70)
			}
		}
		cv.glow(120, 85, 25, p)
		cv.filledCircle(120, 85, 10, moonWhite)
		// Seven smaller stars
		for _, sp := range [][2]float64{{45, 45}, {195, 35}, {30, 110}, {210, 95}, {55, 160}, {190, 150}, {40, 55}} {
			cv.glow(sp[0], sp[1], 6, p)
			cv.filledCircle(sp[0], sp[1], 3, moonWhite)
		}
		// Figure
		cv.filledCircle(120, 230, 16, skin)
		cv.filledCircle(120, 220, 14, sumi)
		cv.filledRect(105, 248, 30, 50, moonWhite)
		// Pouring water
		cv.bezier(100, 265, 70, 300, 60, 340, 50, 380, 2.5, waveBlue)
		cv.bezier(140, 265, 170, 300, 180, 340, 190, 380, 2.5, waveBlue)
		// Water
		for x := 20.0; x < 220; x += 25 {
			cv.wave(x, 375, 8, waveBlue)
		}

	case 18: // Moon — moon over water, two foxes, path
		// Night sky
		cv.fillGradient(indigoDark, indigo)
		cv.paperTexture()
		cv.woodblockBorder(5, 2.0, goldLeaf)
		cv.drawNum(15, 15, num, goldLeaf, 5)
		// Moon
		cv.filledCircle(120, 70, 38, moonWhite)
		cv.filledCircle(135, 60, 38, indigo)
		cv.glow(108, 75, 30, p)
		// Moon drops
		for i := 0; i < 6; i++ {
			x := 80 + float64(i)*16
			cv.line(x, 120, x+3, 140, 1.5, moonWhite)
		}
		// Two guardian towers (stone lanterns)
		cv.filledRect(30, 190, 20, 120, sumiGray)
		cv.filledRect(25, 185, 30, 8, sumiGray)
		cv.glow(40, 195, 8, sunset)
		cv.filledRect(190, 190, 20, 120, sumiGray)
		cv.filledRect(185, 185, 30, 8, sumiGray)
		cv.glow(200, 195, 8, sunset)
		// Winding path
		cv.bezier(120, 390, 105, 360, 135, 330, 120, 290, 3, sumiGray)
		// Two foxes (kitsune)
		// Left fox
		cv.filledCircle(65, 340, 10, sunset)
		cv.filledRect(55, 348, 22, 12, sunset)
		cv.line(58, 335, 55, 325, 2, sunset) // ear
		cv.line(72, 335, 75, 325, 2, sunset) // ear
		// Right fox
		cv.filledCircle(175, 340, 10, sunset)
		cv.filledRect(165, 348, 22, 12, sunset)
		cv.line(168, 335, 165, 325, 2, sunset)
		cv.line(182, 335, 185, 325, 2, sunset)

	case 19: // Sun — radiant sun, child, sunflowers/chrysanthemums
		// Sun
		cv.filledCircle(120, 95, 48, vermillion)
		cv.glow(120, 95, 40, vermillion)
		cv.glow(120, 95, 25, goldLeaf)
		// Rays
		for i := 0; i < 16; i++ {
			a := float64(i) * math.Pi / 8
			r1 := 52.0
			r2 := 78.0
			if i%2 == 0 {
				r2 = 85
			}
			cv.line(120+r1*math.Cos(a), 95+r1*math.Sin(a),
				120+r2*math.Cos(a), 95+r2*math.Sin(a), 2.5, goldLeaf)
		}
		// Child figure (dancing)
		cv.filledCircle(120, 255, 16, skin)
		cv.filledCircle(120, 245, 12, sumi)
		cv.filledRect(110, 273, 20, 35, moonWhite)
		cv.filledRect(112, 308, 10, 22, skin)
		cv.filledRect(128, 308, 10, 22, skin)
		cv.line(110, 280, 90, 260, 2, skin)
		cv.line(130, 280, 150, 260, 2, skin)
		// Chrysanthemums
		for _, pos := range [][2]float64{{35, 350}, {80, 360}, {160, 360}, {205, 350}} {
			for j := 0; j < 10; j++ {
				a := float64(j) * math.Pi / 5
				px := pos[0] + 12*math.Cos(a)
				py := pos[1] + 12*math.Sin(a)
				cv.filledCircle(px, py, 5, goldLeaf)
			}
			cv.filledCircle(pos[0], pos[1], 5, vermillion)
		}
		// Stems
		cv.line(35, 362, 35, 395, 2, bambooGrn)
		cv.line(80, 372, 80, 395, 2, bambooGrn)
		cv.line(160, 372, 160, 395, 2, bambooGrn)
		cv.line(205, 362, 205, 395, 2, bambooGrn)

	case 20: // Judgement — divine figure with taiko drum, rising spirits
		// Clouds
		cv.cloud(60, 40, 22, color.RGBA{200, 195, 185, 255})
		cv.cloud(170, 35, 20, color.RGBA{200, 195, 185, 255})
		// Divine figure
		cv.filledCircle(120, 70, 18, skin)
		cv.circle(120, 55, 22, 2, goldLeaf)
		cv.glow(120, 55, 15, goldLeaf)
		// Wings/clouds
		cv.cloud(70, 80, 20, moonWhite)
		cv.cloud(170, 80, 20, moonWhite)
		// Taiko drum
		cv.filledCircle(175, 100, 15, vermillion)
		cv.circle(175, 100, 15, 2, goldLeaf)
		cv.line(185, 85, 195, 80, 2, sumiGray) // drumstick
		// Sound waves
		cv.arc(180, 100, 22, -0.5, 0.5, 1.5, goldLeaf)
		cv.arc(180, 100, 30, -0.4, 0.4, 1.0, goldLeaf)
		// Rising spirits
		figs := []float64{60, 120, 180}
		for _, fx := range figs {
			cv.filledCircle(fx, 260, 14, skin)
			cv.filledRect(fx-10, 278, 20, 35, moonWhite)
			cv.line(fx-10, 285, fx-18, 268, 2, skin)
			cv.line(fx+10, 285, fx+18, 268, 2, skin)
		}
		// Ground
		cv.line(15, 330, 225, 330, 2, sumiGray)
		cv.glow(120, 290, 35, goldLeaf)

	case 21: // World — dancing figure in enso circle, four guardians
		// Enso circle (brush stroke)
		// Slightly imperfect circle like a real brushstroke
		for a := 0.1; a < 2*math.Pi-0.15; a += 0.03 {
			r := 85 + 3*math.Sin(a*3)
			px := 120 + r*math.Cos(a)
			py := 200 + r*1.2*math.Sin(a)
			th := 3.0 + math.Sin(a*2)*1.0 // varying thickness
			cv.filledCircle(px, py, th, sumi)
		}
		// Cherry blossoms on the circle
		for a := 0.0; a < 2*math.Pi; a += math.Pi / 5 {
			px := 120 + 85*math.Cos(a)
			py := 200 + 100*math.Sin(a)
			cv.cherryBlossom(px, py, 7, sakuraPink)
		}
		// Dancing figure
		cv.filledCircle(120, 165, 16, skin)
		cv.filledCircle(120, 155, 14, sumi)
		// Flowing robes
		cv.bezier(108, 185, 95, 220, 100, 260, 110, 275, 2.5, p)
		cv.bezier(132, 185, 145, 220, 140, 260, 130, 275, 2.5, p)
		cv.filledRect(108, 185, 24, 45, lerpColor(p, s, 0.3))
		// Arms
		cv.line(108, 195, 80, 178, 2.5, skin)
		cv.line(132, 195, 160, 178, 2.5, skin)
		// Fans in hands
		cv.arc(75, 173, 12, math.Pi+0.3, 2*math.Pi-0.3, 1.5, goldLeaf)
		cv.arc(165, 173, 12, math.Pi+0.3, 2*math.Pi-0.3, 1.5, vermillion)
		// Legs
		cv.line(113, 230, 100, 270, 2, skin)
		cv.line(127, 230, 140, 270, 2, skin)
		// Four guardian creatures
		cv.filledCircle(25, 35, 10, vermillion) // dragon
		cv.glow(25, 35, 8, vermillion)
		cv.filledCircle(215, 35, 10, sunset) // tiger
		cv.glow(215, 35, 8, sunset)
		cv.filledCircle(25, 365, 10, bambooGrn) // turtle
		cv.glow(25, 365, 8, bambooGrn)
		cv.filledCircle(215, 365, 10, goldLeaf) // phoenix
		cv.glow(215, 365, 8, goldLeaf)
	}

	return cv
}

// ========== main ==========

func main() {
	base := "decks/ukiyoe"

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
		for num := 1; num <= 10; num++ {
			path := filepath.Join(base, "minor", fmt.Sprintf("%s-%02d.hex", suitName, num))
			fmt.Printf("Minor %-10s %02d", suitName, num)
			if err := genPip(si, num).writeHexFile(path); err != nil {
				fmt.Fprintf(os.Stderr, " error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(" done")
		}
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
