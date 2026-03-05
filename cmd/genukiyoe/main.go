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

func (c *HiResCanvas) fillSolid(col color.RGBA) {
	for y := 0; y < hiH; y++ {
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

// ========== ukiyo-e color palette ==========

var (
	indigo     = color.RGBA{30, 40, 100, 255}   // aizuri blue
	indigoDeep = color.RGBA{20, 25, 60, 255}    // deep aizuri
	vermillion = color.RGBA{200, 55, 30, 255}   // traditional red
	vermDim    = color.RGBA{130, 35, 20, 255}   // dim red
	cream      = color.RGBA{235, 225, 200, 255} // washi paper
	inkBlack   = color.RGBA{15, 12, 10, 255}    // sumi ink
	bamboo     = color.RGBA{60, 130, 55, 255}   // green bamboo
	bambooDim  = color.RGBA{35, 80, 35, 255}    // dim bamboo
	steel      = color.RGBA{140, 155, 180, 255} // katana steel
	steelDim   = color.RGBA{80, 90, 110, 255}   // dim steel
	ochre      = color.RGBA{195, 160, 50, 255}  // gold/ochre
	ochreDim   = color.RGBA{120, 100, 35, 255}  // dim ochre
	sakura     = color.RGBA{230, 150, 170, 255} // cherry blossom pink
	skin       = color.RGBA{220, 190, 155, 255}
	indigoBot  = color.RGBA{15, 18, 45, 255}    // darker bottom
)

// suit colors
type suitColors struct {
	primary, dim color.RGBA
}

var suitPalette = []suitColors{
	{vermillion, vermDim},   // cups/sakazuki (index 0)
	{bamboo, bambooDim},     // wands/take (index 1)
	{steel, steelDim},       // swords/katana (index 2)
	{ochre, ochreDim},       // pentacles/mon (index 3)
}

var suitFileNames = []string{"cups", "wands", "swords", "pentacles"}

// ========== ukiyo-e visual elements ==========

// seigaiha draws concentric wave arcs (traditional wave pattern)
func (c *HiResCanvas) seigaiha(cx, cy, maxR float64, col color.RGBA) {
	for r := maxR; r > 2; r -= maxR / 4 {
		c.arc(cx, cy, r, math.Pi, 2*math.Pi, 1.5, col)
	}
}

// seigaihaField fills a region with repeating seigaiha wave pattern
func (c *HiResCanvas) seigaihaField(x0, y0, w, h float64, col color.RGBA) {
	spacing := 24.0
	row := 0
	for y := y0; y < y0+h; y += spacing * 0.6 {
		offset := 0.0
		if row%2 == 1 {
			offset = spacing / 2
		}
		for x := x0 - spacing + offset; x < x0+w+spacing; x += spacing {
			c.seigaiha(x, y, spacing/2, col)
		}
		row++
	}
}

// cloudBand draws a stylized kumo (cloud) band
func (c *HiResCanvas) cloudBand(cx, cy, w float64, col color.RGBA) {
	// Series of overlapping circles forming a cloud
	for x := cx - w/2; x <= cx+w/2; x += w / 5 {
		r := w/6 + 3*math.Sin(x*0.1)
		c.filledCircle(x, cy, r, col)
	}
	// Bold outline
	for x := cx - w/2; x <= cx+w/2; x += w / 5 {
		r := w/6 + 3*math.Sin(x*0.1)
		c.circle(x, cy, r, 2, inkBlack)
	}
}

// ukiyoeBorder draws a double-line frame with wave motifs
func (c *HiResCanvas) ukiyoeBorder(margin float64) {
	w := float64(hiW)
	h := float64(hiH)
	m := margin

	// Outer frame — bold black
	c.line(m, m, w-m, m, 3, inkBlack)
	c.line(w-m, m, w-m, h-m, 3, inkBlack)
	c.line(w-m, h-m, m, h-m, 3, inkBlack)
	c.line(m, h-m, m, m, 3, inkBlack)

	// Inner frame
	m2 := margin + 6
	c.line(m2, m2, w-m2, m2, 1.5, inkBlack)
	c.line(w-m2, m2, w-m2, h-m2, 1.5, inkBlack)
	c.line(w-m2, h-m2, m2, h-m2, 1.5, inkBlack)
	c.line(m2, h-m2, m2, m2, 1.5, inkBlack)

	// Small wave arcs along top border
	for x := m2 + 10; x < w-m2-10; x += 18 {
		c.arc(x, m2, 6, math.Pi, 2*math.Pi, 1, indigo)
	}
	// Small wave arcs along bottom border
	for x := m2 + 10; x < w-m2-10; x += 18 {
		c.arc(x, h-m2, 6, 0, math.Pi, 1, indigo)
	}
}

// boldOutline draws a thick black outline rectangle
func (c *HiResCanvas) boldOutline(x, y, w, h, thickness float64) {
	c.line(x, y, x+w, y, thickness, inkBlack)
	c.line(x+w, y, x+w, y+h, thickness, inkBlack)
	c.line(x+w, y+h, x, y+h, thickness, inkBlack)
	c.line(x, y+h, x, y, thickness, inkBlack)
}

// cherryBlossom draws a small sakura dot cluster
func (c *HiResCanvas) cherryBlossom(cx, cy, size float64) {
	for i := 0; i < 5; i++ {
		a := float64(i) * 2 * math.Pi / 5
		px := cx + size*0.6*math.Cos(a)
		py := cy + size*0.6*math.Sin(a)
		c.filledCircle(px, py, size*0.4, sakura)
	}
	c.filledCircle(cx, cy, size*0.25, vermillion)
}

// monCrest draws a circular mon (family crest) — circle with inscribed cross
func (c *HiResCanvas) monCrest(cx, cy, r float64, col color.RGBA) {
	c.circle(cx, cy, r, 2.5, col)
	c.line(cx-r*0.6, cy, cx+r*0.6, cy, 2, col)
	c.line(cx, cy-r*0.6, cx, cy+r*0.6, 2, col)
	c.circle(cx, cy, r*0.4, 1.5, col)
}

// toriiGate draws a simplified torii gate
func (c *HiResCanvas) toriiGate(cx, cy, w, h float64, col color.RGBA) {
	// Two pillars
	pw := w * 0.08
	c.filledRect(cx-w/2, cy, pw, h, col)
	c.filledRect(cx+w/2-pw, cy, pw, h, col)
	c.boldOutline(cx-w/2, cy, pw, h, 2)
	c.boldOutline(cx+w/2-pw, cy, pw, h, 2)
	// Top beam (kasagi) — extends beyond pillars
	c.filledRect(cx-w/2-w*0.1, cy, w*1.2, h*0.08, col)
	c.line(cx-w/2-w*0.1, cy, cx+w/2+w*0.1, cy, 2.5, inkBlack)
	c.line(cx-w/2-w*0.1, cy+h*0.08, cx+w/2+w*0.1, cy+h*0.08, 2.5, inkBlack)
	// Second beam (nuki)
	c.filledRect(cx-w/2+pw, cy+h*0.18, w-2*pw, h*0.06, col)
	c.line(cx-w/2+pw, cy+h*0.18, cx+w/2-pw, cy+h*0.18, 1.5, inkBlack)
	c.line(cx-w/2+pw, cy+h*0.24, cx+w/2-pw, cy+h*0.24, 1.5, inkBlack)
}

// ========== suit pip shapes (ukiyo-e style) ==========

func (c *HiResCanvas) drawCup(cx, cy, size float64, col color.RGBA) {
	// Sakazuki — flat wide sake cup
	// Wide shallow bowl
	w := size * 0.5
	h := size * 0.15
	c.filledRect(cx-w, cy-h, w*2, h*2, col)
	// Curved rim
	c.arc(cx, cy-h, w, math.Pi, 2*math.Pi, 2, col)
	// Base/foot
	c.filledRect(cx-size*0.12, cy+h, size*0.24, size*0.2, col)
	c.filledRect(cx-size*0.2, cy+h+size*0.2, size*0.4, size*0.06, col)
	// Bold outlines
	c.arc(cx, cy-h, w+1, math.Pi, 2*math.Pi, 2, inkBlack)
	c.line(cx-w, cy-h, cx-w, cy+h, 2, inkBlack)
	c.line(cx+w, cy-h, cx+w, cy+h, 2, inkBlack)
	c.line(cx-w, cy+h, cx+w, cy+h, 1.5, inkBlack)
}

func (c *HiResCanvas) drawWand(cx, cy, size float64, col color.RGBA) {
	// Take — bamboo stalk with node marks
	w := size * 0.1
	c.filledRect(cx-w, cy-size*0.5, w*2, size, col)
	// Node rings
	for i := 0; i < 3; i++ {
		ny := cy - size*0.3 + float64(i)*size*0.3
		c.filledRect(cx-w*1.5, ny-1, w*3, 3, lerpColor(col, inkBlack, 0.3))
	}
	// Small leaves at top node
	c.line(cx+w, cy-size*0.3, cx+size*0.3, cy-size*0.45, 2, col)
	c.line(cx-w, cy-size*0.3, cx-size*0.3, cy-size*0.45, 2, col)
	// Bold outline
	c.line(cx-w, cy-size*0.5, cx-w, cy+size*0.5, 2, inkBlack)
	c.line(cx+w, cy-size*0.5, cx+w, cy+size*0.5, 2, inkBlack)
}

func (c *HiResCanvas) drawSword(cx, cy, size float64, col color.RGBA) {
	// Katana — curved blade with tsuba guard
	// Curved blade
	for i := 0; i <= 100; i++ {
		t := float64(i) / 100
		py := cy - size*0.5 + t*size*0.75
		curve := math.Sin(t*math.Pi) * size * 0.08
		c.filledRect(cx+curve-1.5, py, 3, 1, col)
	}
	// Tsuba (guard) — small circle
	guardY := cy + size*0.25
	c.filledCircle(cx, guardY, size*0.1, ochre)
	c.circle(cx, guardY, size*0.1, 1.5, inkBlack)
	// Handle (tsuka)
	c.filledRect(cx-2, guardY+size*0.1, 4, size*0.2, color.RGBA{60, 40, 25, 255})
	// Bold outline on blade
	for i := 0; i <= 100; i++ {
		t := float64(i) / 100
		py := cy - size*0.5 + t*size*0.75
		curve := math.Sin(t*math.Pi) * size * 0.08
		c.set(int(cx+curve-2), int(py), inkBlack)
		c.set(int(cx+curve+2), int(py), inkBlack)
	}
	// Blade tip
	c.filledCircle(cx, cy-size*0.5, 2, col)
}

func (c *HiResCanvas) drawPentacle(cx, cy, size float64, col color.RGBA) {
	// Mon — circle with inscribed cross (family crest)
	c.filledCircle(cx, cy, size*0.38, col)
	c.circle(cx, cy, size*0.38, 2.5, inkBlack)
	// Inner cross
	c.line(cx-size*0.25, cy, cx+size*0.25, cy, 2, inkBlack)
	c.line(cx, cy-size*0.25, cx, cy+size*0.25, 2, inkBlack)
	// Inner circle
	c.circle(cx, cy, size*0.18, 1.5, inkBlack)
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

// ========== number glyphs (hi-res, scale 10) ==========

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

// ========== ukiyo-e backgrounds ==========

func ukiyoeBG() *HiResCanvas {
	cv := newCanvas()
	cv.fillGradient(indigo, indigoBot)
	return cv
}

func creamBG() *HiResCanvas {
	cv := newCanvas()
	cv.fillSolid(cream)
	return cv
}

// ========== card generators ==========

func genBack() *HiResCanvas {
	cv := newCanvas()
	cv.fillSolid(indigoDeep)

	// Seigaiha wave pattern covering the card
	cv.seigaihaField(0, 0, float64(hiW), float64(hiH), indigo)

	// Border
	cv.ukiyoeBorder(8)

	// Central mon crest circle
	cv.filledCircle(120, 200, 45, indigoDeep)
	cv.monCrest(120, 200, 40, ochre)
	cv.glow(120, 200, 25, ochre)

	// Corner cherry blossoms
	cv.cherryBlossom(30, 30, 10)
	cv.cherryBlossom(210, 30, 10)
	cv.cherryBlossom(30, 370, 10)
	cv.cherryBlossom(210, 370, 10)

	return cv
}

func genPip(si, num int) *HiResCanvas {
	cv := creamBG()
	sc := suitPalette[si]

	// Subtle wave background
	cv.seigaihaField(0, 0, float64(hiW), float64(hiH), lerpColor(cream, indigo, 0.08))

	// Border
	cv.ukiyoeBorder(6)

	// Corner numbers — scale 10 for readability
	const numScale = 10.0
	cv.drawNum(15, 15, num, sc.primary, numScale)
	// Bottom-right: mirror top-left, accounting for digit width
	numW := 3.0 * numScale // single digit width
	if num >= 10 {
		numW = 7 * numScale // two digits: 3 + 1 gap + 3, each × scale
	}
	cv.drawNum(float64(hiW)-15-numW, float64(hiH)-15-5*numScale, num, sc.primary, numScale)

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
	cv := creamBG()
	sc := suitPalette[si]

	// Subtle wave background
	cv.seigaihaField(0, 0, float64(hiW), float64(hiH), lerpColor(cream, indigo, 0.06))

	// Border
	cv.ukiyoeBorder(5)

	p := sc.primary

	switch rank {
	case 11: // Page — youth
		// Background panel
		cv.filledRect(30, 50, 180, 290, lerpColor(cream, p, 0.1))
		cv.boldOutline(30, 50, 180, 290, 2)
		// Head
		cv.filledCircle(120, 110, 25, skin)
		cv.circle(120, 110, 25, 2, inkBlack)
		// Hair (black, traditional)
		cv.arc(120, 100, 26, math.Pi, 2*math.Pi, 4, inkBlack)
		// Kimono body
		cv.filledRect(95, 140, 50, 120, p)
		// Kimono overlap (V-neck)
		cv.line(120, 140, 100, 180, 2, inkBlack)
		cv.line(120, 140, 140, 180, 2, inkBlack)
		// Obi (belt)
		cv.filledRect(95, 210, 50, 12, sc.dim)
		cv.boldOutline(95, 210, 50, 12, 1.5)
		// Legs/hakama
		cv.filledRect(100, 260, 18, 55, sc.dim)
		cv.filledRect(125, 260, 18, 55, sc.dim)
		// Bold body outline
		cv.boldOutline(95, 140, 50, 120, 2.5)
		// Suit symbol in hand
		cv.drawSuitPip(si, 165, 190, 25)
		// Cherry blossoms accent
		cv.cherryBlossom(50, 70, 7)
		cv.cherryBlossom(190, 70, 7)

	case 12: // Knight — samurai warrior
		cv.filledRect(30, 50, 180, 290, lerpColor(cream, p, 0.1))
		cv.boldOutline(30, 50, 180, 290, 2)
		// Kabuto (helmet)
		cv.filledCircle(110, 90, 28, steelDim)
		cv.arc(110, 80, 35, math.Pi+0.3, 2*math.Pi-0.3, 3, steel)
		// Helmet crest (maedate)
		cv.line(110, 52, 110, 75, 3, ochre)
		cv.filledCircle(110, 50, 5, ochre)
		cv.circle(110, 90, 28, 2, inkBlack)
		// Face
		cv.filledCircle(110, 98, 16, skin)
		// Armor body (do)
		cv.filledRect(85, 125, 50, 100, p)
		cv.boldOutline(85, 125, 50, 100, 2.5)
		// Shoulder plates (sode)
		cv.filledRect(60, 125, 28, 35, sc.dim)
		cv.filledRect(132, 125, 28, 35, sc.dim)
		cv.boldOutline(60, 125, 28, 35, 2)
		cv.boldOutline(132, 125, 28, 35, 2)
		// Extended arm with weapon
		cv.line(160, 155, 195, 100, 3, skin)
		// Suit symbol as weapon ornament
		cv.drawSuitPip(si, 195, 85, 22)
		// Legs
		cv.filledRect(90, 225, 16, 65, sc.dim)
		cv.filledRect(115, 225, 16, 65, sc.dim)

	case 13: // Queen — noblewoman/geisha
		cv.filledRect(30, 50, 180, 290, lerpColor(cream, p, 0.1))
		cv.boldOutline(30, 50, 180, 290, 2)
		// Elaborate hair (shimada style)
		cv.filledCircle(120, 78, 28, inkBlack)
		cv.filledRect(100, 50, 40, 20, inkBlack)
		// Hair ornament (kanzashi)
		cv.filledCircle(140, 60, 5, sakura)
		cv.filledCircle(135, 55, 4, ochre)
		// Face
		cv.filledCircle(120, 90, 18, skin)
		cv.circle(120, 90, 18, 1.5, inkBlack)
		// Flowing kimono
		for y := 115.0; y < 320; y++ {
			t := (y - 115) / 205
			w := 25 + t*60
			col := lerpColor(p, sc.dim, t*0.4)
			cv.filledRect(120-w, y, w*2, 1, col)
		}
		// Kimono overlap
		cv.line(120, 115, 98, 170, 2, inkBlack)
		cv.line(120, 115, 142, 170, 2, inkBlack)
		// Obi
		cv.filledRect(85, 195, 70, 18, ochre)
		cv.boldOutline(85, 195, 70, 18, 2)
		// Sleeves (furisode — long)
		cv.bezier(95, 140, 50, 170, 40, 220, 55, 240, 2.5, inkBlack)
		cv.bezier(145, 140, 190, 170, 200, 220, 185, 240, 2.5, inkBlack)
		// Fan or suit symbol
		cv.drawSuitPip(si, 170, 260, 25)
		// Bold outline
		cv.line(120-85, 320, 120+85, 320, 2.5, inkBlack)

	case 14: // King — daimyo
		cv.filledRect(30, 50, 180, 290, lerpColor(cream, p, 0.1))
		cv.boldOutline(30, 50, 180, 290, 2)
		// Eboshi/court hat
		cv.filledRect(105, 45, 30, 30, inkBlack)
		cv.filledRect(100, 75, 40, 8, inkBlack)
		// Head
		cv.filledCircle(120, 92, 22, skin)
		cv.circle(120, 92, 22, 2, inkBlack)
		// Beard
		cv.filledCircle(120, 108, 12, color.RGBA{60, 45, 30, 255})
		// Broad formal robes (kariginu)
		cv.filledRect(65, 120, 110, 130, p)
		cv.boldOutline(65, 120, 110, 130, 3)
		// Inner robe visible at neck
		cv.filledRect(100, 120, 40, 30, cream)
		cv.line(120, 120, 105, 150, 2, inkBlack)
		cv.line(120, 120, 135, 150, 2, inkBlack)
		// Mon crest on chest
		cv.monCrest(120, 185, 18, inkBlack)
		// Scepter/fan (shaku)
		cv.filledRect(170, 130, 6, 80, ochre)
		cv.boldOutline(170, 130, 6, 80, 1.5)
		// Legs/hakama
		cv.filledRect(85, 250, 25, 55, sc.dim)
		cv.filledRect(130, 250, 25, 55, sc.dim)
		// Suit symbol
		cv.drawSuitPip(si, 55, 290, 22)
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
				x := 15 + float64(i%3)*10
				y := 15 + float64(i/3)*10
				cv.filledRect(x, y, 10, 10, sc.primary)
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
	bg                 color.RGBA
}

var majorSchemes = []majorScheme{
	{cream, ochre, color.RGBA{40, 50, 80, 255}},                // 0  Fool — wandering ronin
	{ochre, indigo, color.RGBA{35, 35, 55, 255}},               // 1  Magician — onmyoji
	{vermillion, cream, color.RGBA{30, 25, 50, 255}},           // 2  High Priestess — miko
	{ochre, vermillion, color.RGBA{50, 40, 30, 255}},           // 3  Empress — Amaterasu
	{vermillion, ochre, color.RGBA{35, 25, 30, 255}},           // 4  Emperor — shogun
	{ochre, cream, color.RGBA{40, 35, 25, 255}},                // 5  Hierophant — Buddhist monk
	{sakura, cream, color.RGBA{45, 35, 40, 255}},               // 6  Lovers — cherry blossoms
	{steel, ochre, color.RGBA{30, 35, 50, 255}},                // 7  Chariot — samurai horseback
	{ochre, bamboo, color.RGBA{40, 38, 28, 255}},               // 8  Strength — tiger
	{cream, ochreDim, color.RGBA{35, 35, 40, 255}},             // 9  Hermit — yamabushi
	{ochre, vermillion, color.RGBA{30, 30, 45, 255}},           // 10 Wheel — dharma/enso
	{steel, ochre, color.RGBA{35, 32, 42, 255}},                // 11 Justice — magistrate
	{indigo, cream, color.RGBA{25, 30, 50, 255}},               // 12 Hanged Man — zen
	{cream, vermillion, color.RGBA{30, 25, 30, 255}},           // 13 Death — skeletal
	{indigo, ochre, color.RGBA{25, 30, 50, 255}},               // 14 Temperance — pouring
	{vermillion, ochre, color.RGBA{45, 20, 20, 255}},           // 15 Devil — oni
	{vermillion, ochre, color.RGBA{40, 25, 25, 255}},           // 16 Tower — pagoda
	{cream, indigo, color.RGBA{20, 25, 50, 255}},               // 17 Star — starfield
	{cream, indigo, color.RGBA{18, 22, 45, 255}},               // 18 Moon — waves/pines
	{ochre, vermillion, color.RGBA{55, 45, 20, 255}},           // 19 Sun — rising sun
	{vermillion, ochre, color.RGBA{35, 28, 35, 255}},           // 20 Judgement — taiko
	{ochre, vermillion, color.RGBA{30, 30, 40, 255}},           // 21 World — torii gate
}

func genMajor(num int) *HiResCanvas {
	cv := newCanvas()
	sc := majorSchemes[num]
	p, s := sc.primary, sc.secondary

	cv.fillSolid(sc.bg)

	// Subtle wave pattern
	cv.seigaihaField(0, 0, float64(hiW), float64(hiH), lerpColor(sc.bg, indigo, 0.15))

	// Number in corner — scale 10
	cv.drawNum(15, 15, num, ochre, 10)

	// Border
	cv.ukiyoeBorder(5)

	// Scene panel
	cv.filledRect(25, 55, 190, 305, lerpColor(sc.bg, cream, 0.08))
	cv.boldOutline(25, 55, 190, 305, 2)

	switch num {
	case 0: // Fool → wandering ronin at cliff edge
		// Rocky cliff
		for y := 280; y < 360; y++ {
			t := float64(y-280) / 80
			w := 70 + t*50
			col := lerpColor(color.RGBA{80, 65, 50, 255}, color.RGBA{50, 40, 30, 255}, t)
			cv.filledRect(30, float64(y), w, 1, col)
		}
		cv.line(30, 280, 100+50, 280, 2.5, inkBlack)
		// Void below cliff
		cv.filledRect(140, 280, 75, 80, indigoDeep)
		// Figure (ronin)
		cv.filledCircle(100, 160, 22, skin)
		cv.circle(100, 160, 22, 2, inkBlack)
		// Ronin hair (topknot)
		cv.filledRect(95, 138, 10, 8, inkBlack)
		cv.filledCircle(100, 135, 6, inkBlack)
		// Kimono
		cv.filledRect(82, 185, 36, 90, p)
		cv.boldOutline(82, 185, 36, 90, 2.5)
		cv.line(100, 185, 85, 215, 2, inkBlack)
		cv.line(100, 185, 115, 215, 2, inkBlack)
		// Walking stick
		cv.line(130, 170, 140, 310, 2.5, color.RGBA{100, 70, 40, 255})
		// Bundle on stick
		cv.filledCircle(132, 165, 12, s)
		cv.circle(132, 165, 12, 2, inkBlack)
		// Legs
		cv.filledRect(88, 275, 12, 40, color.RGBA{80, 60, 45, 255})
		cv.filledRect(103, 275, 12, 40, color.RGBA{80, 60, 45, 255})
		// Cherry blossoms floating
		cv.cherryBlossom(170, 80, 8)
		cv.cherryBlossom(55, 100, 6)
		cv.cherryBlossom(185, 150, 7)

	case 1: // Magician → onmyoji (yin-yang diviner)
		// Mystic table
		cv.filledRect(45, 255, 150, 8, ochre)
		cv.line(45, 255, 195, 255, 2.5, inkBlack)
		// Five elements on table
		cv.drawSuitPip(0, 65, 290, 22)
		cv.drawSuitPip(1, 105, 290, 22)
		cv.drawSuitPip(2, 145, 290, 22)
		cv.drawSuitPip(3, 185, 290, 22)
		// Figure
		cv.filledCircle(120, 130, 22, skin)
		cv.circle(120, 130, 22, 2, inkBlack)
		// Eboshi hat
		cv.filledRect(107, 100, 26, 22, inkBlack)
		// Robes
		cv.filledRect(95, 155, 50, 95, p)
		cv.boldOutline(95, 155, 50, 95, 2.5)
		cv.line(120, 155, 100, 185, 2, inkBlack)
		cv.line(120, 155, 140, 185, 2, inkBlack)
		// Arms raised — mystic gesture
		cv.line(95, 175, 55, 145, 3, skin)
		cv.line(145, 175, 185, 145, 3, skin)
		// Yin-yang above
		cv.filledCircle(120, 72, 18, cream)
		cv.filledCircle(120, 72, 18, cream)
		cv.filledRect(120, 54, 18, 36, inkBlack)
		cv.filledCircle(120, 63, 9, inkBlack)
		cv.filledCircle(120, 81, 9, cream)
		cv.circle(120, 72, 18, 2, inkBlack)
		// Glow around hands
		cv.glow(55, 145, 12, ochre)
		cv.glow(185, 145, 12, ochre)

	case 2: // High Priestess → miko (shrine maiden)
		// Torii gate background
		cv.toriiGate(120, 60, 160, 180, vermillion)
		// Figure
		cv.filledCircle(120, 145, 20, skin)
		cv.circle(120, 145, 20, 2, inkBlack)
		// Long black hair
		cv.filledRect(105, 130, 30, 45, inkBlack)
		cv.bezier(105, 140, 90, 180, 85, 220, 90, 250, 3, inkBlack)
		cv.bezier(135, 140, 150, 180, 155, 220, 150, 250, 3, inkBlack)
		// White top (chihaya)
		cv.filledRect(100, 170, 40, 50, cream)
		cv.boldOutline(100, 170, 40, 50, 2)
		// Red hakama
		cv.filledRect(95, 220, 50, 100, vermillion)
		cv.boldOutline(95, 220, 50, 100, 2.5)
		// Kagura bell/suzu in hand
		cv.line(140, 190, 170, 175, 2, ochre)
		cv.filledCircle(173, 172, 6, ochre)
		cv.circle(173, 172, 6, 1.5, inkBlack)
		// Shimenawa (sacred rope) in background
		cv.line(35, 90, 205, 90, 3, cream)
		// Paper streamers (shide)
		for x := 55.0; x < 200; x += 35 {
			cv.filledRect(x, 90, 8, 15, cream)
			cv.line(x, 90, x, 105, 1, inkBlack)
		}

	case 3: // Empress → Amaterasu radiating light
		// Radiating sun lines
		for i := 0; i < 16; i++ {
			a := float64(i) * math.Pi / 8
			r1 := 40.0
			r2 := 90.0
			cv.line(120+r1*math.Cos(a), 140+r1*math.Sin(a),
				120+r2*math.Cos(a), 140+r2*math.Sin(a), 2, ochre)
		}
		cv.glow(120, 140, 45, ochre)
		// Figure
		cv.filledCircle(120, 118, 22, skin)
		cv.circle(120, 118, 22, 2, inkBlack)
		// Elaborate hair with kanzashi
		cv.filledCircle(120, 105, 26, inkBlack)
		cv.filledCircle(140, 98, 5, ochre)
		cv.filledCircle(100, 100, 4, vermillion)
		// Flowing junihitoe (twelve-layer robes)
		for y := 145.0; y < 340; y++ {
			t := (y - 145) / 195
			w := 25 + t*80
			col := lerpColor(p, s, t*0.3)
			cv.filledRect(120-w, y, w*2, 1, col)
		}
		cv.line(120-105, 340, 120+105, 340, 2.5, inkBlack)
		// Robe outlines
		cv.line(95, 145, 35, 340, 2.5, inkBlack)
		cv.line(145, 145, 205, 340, 2.5, inkBlack)
		// Mirror (yata no kagami) — sacred mirror
		cv.filledCircle(120, 185, 18, ochre)
		cv.circle(120, 185, 18, 2, inkBlack)
		cv.glow(120, 185, 12, cream)

	case 4: // Emperor → shogun enthroned
		// Throne/dais
		cv.filledRect(35, 250, 170, 100, color.RGBA{60, 40, 30, 255})
		cv.boldOutline(35, 250, 170, 100, 2.5)
		// Mon crests on throne
		cv.monCrest(65, 290, 12, ochre)
		cv.monCrest(175, 290, 12, ochre)
		// Figure
		cv.filledCircle(120, 118, 24, skin)
		cv.circle(120, 118, 24, 2, inkBlack)
		// Kanmuri (court cap)
		cv.filledRect(105, 88, 30, 20, inkBlack)
		cv.filledRect(100, 108, 40, 6, inkBlack)
		// Tail of cap
		cv.line(135, 95, 175, 80, 2, inkBlack)
		cv.line(175, 80, 180, 95, 1.5, inkBlack)
		// Broad robes
		cv.filledRect(70, 148, 100, 100, p)
		cv.boldOutline(70, 148, 100, 100, 3)
		// Inner robe
		cv.line(120, 148, 100, 185, 2, inkBlack)
		cv.line(120, 148, 140, 185, 2, inkBlack)
		// Shaku (scepter)
		cv.filledRect(165, 155, 6, 70, ochre)
		cv.boldOutline(165, 155, 6, 70, 1.5)
		// Katana at side
		cv.line(60, 180, 50, 260, 2.5, steel)

	case 5: // Hierophant → Buddhist monk
		// Temple columns
		cv.filledRect(30, 70, 18, 280, color.RGBA{80, 55, 35, 255})
		cv.filledRect(192, 70, 18, 280, color.RGBA{80, 55, 35, 255})
		cv.boldOutline(30, 70, 18, 280, 2)
		cv.boldOutline(192, 70, 18, 280, 2)
		// Dharma wheel at top
		cv.circle(120, 70, 22, 2.5, ochre)
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			cv.line(120+10*math.Cos(a), 70+10*math.Sin(a),
				120+20*math.Cos(a), 70+20*math.Sin(a), 2, ochre)
		}
		cv.filledCircle(120, 70, 6, ochre)
		// Figure (shaved head)
		cv.filledCircle(120, 155, 22, skin)
		cv.circle(120, 155, 22, 2, inkBlack)
		// Simple monk robes (kesa)
		cv.filledRect(90, 180, 60, 120, s)
		cv.boldOutline(90, 180, 60, 120, 2.5)
		// Kesa sash
		cv.line(90, 185, 150, 210, 3, ochre)
		// Prayer beads (juzu)
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			bx := 85 + 15*math.Cos(a)
			by := 220 + 15*math.Sin(a)
			cv.filledCircle(bx, by, 3, color.RGBA{80, 50, 30, 255})
		}
		// Seated (zazen)
		cv.filledRect(75, 300, 90, 30, sc.secondary)
		cv.boldOutline(75, 300, 90, 30, 2)
		// Incense
		cv.glow(175, 170, 8, ochre)
		cv.line(175, 170, 175, 140, 1, ochre)

	case 6: // Lovers → two figures under cherry blossoms
		// Cherry tree
		cv.line(120, 55, 120, 160, 4, color.RGBA{80, 50, 30, 255})
		cv.bezier(120, 80, 60, 60, 35, 70, 40, 55, 3, color.RGBA{80, 50, 30, 255})
		cv.bezier(120, 80, 180, 60, 205, 70, 200, 55, 3, color.RGBA{80, 50, 30, 255})
		// Cherry blossoms
		blossoms := [][2]float64{{55, 55}, {85, 45}, {120, 55}, {155, 45}, {185, 55},
			{70, 70}, {100, 65}, {140, 65}, {170, 70}, {50, 85}, {190, 85}}
		for _, b := range blossoms {
			cv.cherryBlossom(b[0], b[1], 7)
		}
		// Falling petals
		cv.filledCircle(90, 120, 3, sakura)
		cv.filledCircle(150, 140, 2, sakura)
		cv.filledCircle(70, 160, 2, sakura)
		// Left figure (man)
		cv.filledCircle(85, 200, 18, skin)
		cv.circle(85, 200, 18, 2, inkBlack)
		cv.filledCircle(85, 190, 20, inkBlack) // hair
		cv.filledRect(68, 222, 34, 80, indigo)
		cv.boldOutline(68, 222, 34, 80, 2)
		// Right figure (woman)
		cv.filledCircle(155, 200, 18, skin)
		cv.circle(155, 200, 18, 2, inkBlack)
		cv.filledCircle(155, 188, 22, inkBlack) // hair
		cv.filledCircle(170, 183, 4, sakura)    // kanzashi
		cv.filledRect(138, 222, 34, 80, vermillion)
		cv.boldOutline(138, 222, 34, 80, 2)
		// Hands reaching toward each other
		cv.line(102, 250, 118, 250, 2.5, skin)
		cv.line(138, 250, 122, 250, 2.5, skin)
		cv.glow(120, 250, 8, sakura)
		// Ground
		cv.line(30, 310, 210, 310, 2, inkBlack)

	case 7: // Chariot → samurai on horseback
		// Horse
		cv.filledCircle(115, 240, 35, color.RGBA{60, 45, 30, 255}) // body
		cv.filledRect(90, 250, 55, 40, color.RGBA{60, 45, 30, 255})
		// Horse head
		cv.filledCircle(155, 210, 20, color.RGBA{60, 45, 30, 255})
		cv.filledRect(155, 195, 25, 15, color.RGBA{60, 45, 30, 255})
		// Legs
		cv.filledRect(88, 285, 8, 40, color.RGBA{50, 35, 22, 255})
		cv.filledRect(100, 288, 8, 37, color.RGBA{50, 35, 22, 255})
		cv.filledRect(130, 285, 8, 40, color.RGBA{50, 35, 22, 255})
		cv.filledRect(142, 288, 8, 37, color.RGBA{50, 35, 22, 255})
		// Bold horse outline
		cv.circle(115, 240, 35, 2.5, inkBlack)
		// Rider (samurai)
		cv.filledCircle(115, 170, 20, skin)
		cv.circle(115, 170, 20, 2, inkBlack)
		// Kabuto
		cv.arc(115, 160, 24, math.Pi, 2*math.Pi, 3, steel)
		cv.line(115, 138, 115, 150, 3, ochre) // crest
		// Armor
		cv.filledRect(98, 195, 34, 45, p)
		cv.boldOutline(98, 195, 34, 45, 2.5)
		// Banner (sashimono)
		cv.line(85, 180, 85, 90, 2, color.RGBA{80, 55, 35, 255})
		cv.filledRect(55, 85, 30, 40, s)
		cv.boldOutline(55, 85, 30, 40, 2)
		cv.monCrest(70, 105, 10, inkBlack)
		// Ground
		cv.line(30, 330, 210, 330, 2, inkBlack)

	case 8: // Strength → figure calming tiger
		// Figure
		cv.filledCircle(90, 130, 20, skin)
		cv.circle(90, 130, 20, 2, inkBlack)
		// Long flowing hair
		cv.bezier(75, 130, 60, 160, 55, 200, 65, 230, 3, inkBlack)
		cv.bezier(105, 130, 120, 160, 125, 200, 115, 230, 3, inkBlack)
		// Robes
		cv.filledRect(72, 155, 36, 100, cream)
		cv.boldOutline(72, 155, 36, 100, 2.5)
		// Hand on tiger
		cv.line(105, 175, 135, 195, 3, skin)
		// Tiger (bold flat colors)
		cv.filledCircle(160, 220, 32, ochre)
		cv.filledCircle(170, 195, 22, ochre) // head
		// Tiger stripes
		for i := 0; i < 5; i++ {
			y := 200 + float64(i)*10
			cv.line(140, y, 150, y-5, 2.5, inkBlack)
		}
		// Tiger face
		cv.filledCircle(163, 192, 4, inkBlack) // eyes
		cv.filledCircle(177, 192, 4, inkBlack)
		cv.filledCircle(170, 200, 3, vermillion) // nose
		// Tiger body/legs
		cv.filledRect(140, 240, 45, 25, ochre)
		cv.filledRect(142, 265, 10, 30, ochre)
		cv.filledRect(168, 265, 10, 30, ochre)
		cv.circle(160, 220, 32, 2.5, inkBlack)
		cv.circle(170, 195, 22, 2, inkBlack)
		// Bamboo in background
		cv.line(195, 60, 195, 340, 3, bamboo)
		cv.filledRect(192, 130, 6, 4, bambooDim)
		cv.filledRect(192, 220, 6, 4, bambooDim)

	case 9: // Hermit → mountain ascetic (yamabushi)
		// Mountain
		for y := 160; y < 360; y++ {
			t := float64(y-160) / 200
			w := t * 110
			col := lerpColor(color.RGBA{60, 55, 50, 255}, color.RGBA{40, 35, 30, 255}, t)
			cv.filledRect(120-w, float64(y), w*2, 1, col)
		}
		cv.line(120, 160, 10, 360, 2, inkBlack)
		cv.line(120, 160, 230, 360, 2, inkBlack)
		// Path
		cv.bezier(120, 250, 110, 280, 130, 320, 120, 355, 2, ochreDim)
		// Figure
		cv.filledCircle(120, 120, 18, skin)
		cv.circle(120, 120, 18, 2, inkBlack)
		// Tokin (yamabushi headpiece)
		cv.filledRect(110, 100, 20, 10, cream)
		cv.boldOutline(110, 100, 20, 10, 1.5)
		// Robes
		cv.filledRect(102, 142, 36, 80, cream)
		cv.boldOutline(102, 142, 36, 80, 2.5)
		// Shakujo staff
		cv.line(85, 120, 85, 280, 2.5, ochre)
		cv.circle(85, 112, 10, 2, ochre)
		// Conch shell (horagai) at belt
		cv.filledCircle(145, 175, 8, cream)
		cv.circle(145, 175, 8, 1.5, inkBlack)
		// Mist/clouds
		cv.cloudBand(120, 200, 140, lerpColor(cream, sc.bg, 0.5))

	case 10: // Wheel → dharma wheel / enso circle
		// Large enso circle (brush stroke)
		cv.circle(120, 200, 75, 6, inkBlack)
		// Leave a gap at top right (enso style)
		cv.filledRect(155, 128, 30, 15, sc.bg)
		cv.seigaihaField(155, 128, 30, 15, lerpColor(sc.bg, indigo, 0.15))
		// Dharma wheel spokes inside
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			cv.line(120+20*math.Cos(a), 200+20*math.Sin(a),
				120+60*math.Cos(a), 200+60*math.Sin(a), 2, ochre)
		}
		// Central hub
		cv.filledCircle(120, 200, 18, ochre)
		cv.circle(120, 200, 18, 2, inkBlack)
		cv.monCrest(120, 200, 14, inkBlack)
		// Rim
		cv.circle(120, 200, 60, 3, ochre)
		// Cloud bands at corners
		cv.cloudBand(60, 70, 80, lerpColor(cream, sc.bg, 0.3))
		cv.cloudBand(180, 340, 80, lerpColor(cream, sc.bg, 0.3))

	case 11: // Justice → magistrate with scales
		// Figure (magistrate / bugyō)
		cv.filledCircle(120, 130, 22, skin)
		cv.circle(120, 130, 22, 2, inkBlack)
		// Eboshi hat
		cv.filledRect(107, 102, 26, 20, inkBlack)
		// Formal robes
		cv.filledRect(85, 155, 70, 110, p)
		cv.boldOutline(85, 155, 70, 110, 2.5)
		cv.line(120, 155, 95, 190, 2, inkBlack)
		cv.line(120, 155, 145, 190, 2, inkBlack)
		// Scales
		cv.line(50, 200, 190, 200, 3, inkBlack)
		cv.line(120, 180, 120, 200, 2, inkBlack)
		// Left pan
		cv.arc(65, 220, 20, 0, math.Pi, 2, ochre)
		cv.line(50, 200, 65, 220, 1.5, inkBlack)
		cv.line(80, 200, 65, 220, 1.5, inkBlack)
		// Right pan
		cv.arc(175, 220, 20, 0, math.Pi, 2, ochre)
		cv.line(160, 200, 175, 220, 1.5, inkBlack)
		cv.line(190, 200, 175, 220, 1.5, inkBlack)
		// Seated on platform
		cv.filledRect(70, 280, 100, 20, ochre)
		cv.boldOutline(70, 280, 100, 20, 2)
		// Mon behind
		cv.monCrest(120, 330, 20, ochreDim)

	case 12: // Hanged Man → inverted figure, zen paradox
		// Bamboo frame
		cv.line(80, 60, 80, 180, 4, bamboo)
		cv.line(160, 60, 160, 180, 4, bamboo)
		cv.line(75, 80, 165, 80, 3, bamboo)
		// Node marks
		cv.filledRect(77, 100, 6, 4, bambooDim)
		cv.filledRect(157, 120, 6, 4, bambooDim)
		// Rope
		cv.line(120, 80, 120, 130, 2, ochreDim)
		// Inverted figure — foot tied at top
		cv.line(120, 130, 110, 165, 3, indigo)
		cv.line(120, 130, 130, 165, 3, indigo)
		// Body
		cv.filledRect(103, 165, 34, 60, p)
		cv.boldOutline(103, 165, 34, 60, 2)
		// Arms hanging down
		cv.line(103, 195, 85, 235, 2.5, skin)
		cv.line(137, 195, 155, 235, 2.5, skin)
		// Head at bottom with golden halo
		cv.filledCircle(120, 250, 20, skin)
		cv.circle(120, 250, 20, 2, inkBlack)
		cv.circle(120, 250, 28, 2.5, ochre)
		cv.glow(120, 250, 30, ochreDim)
		// Serene expression
		cv.filledCircle(114, 248, 2, inkBlack)
		cv.filledCircle(126, 248, 2, inkBlack)
		cv.arc(120, 254, 6, 0.2, math.Pi-0.2, 1, inkBlack)
		// Cherry petals floating
		cv.filledCircle(60, 290, 3, sakura)
		cv.filledCircle(180, 300, 2, sakura)
		cv.filledCircle(100, 320, 2, sakura)

	case 13: // Death → skeletal figure with scythe, cherry petals
		// Skull
		cv.filledCircle(120, 110, 32, cream)
		cv.filledCircle(108, 105, 9, inkBlack) // eye sockets
		cv.filledCircle(132, 105, 9, inkBlack)
		cv.filledCircle(120, 118, 4, inkBlack) // nose
		cv.filledRect(108, 128, 24, 4, inkBlack) // jaw
		for x := 108.0; x < 132; x += 6 {
			cv.line(x, 128, x, 132, 1, cream)
		}
		cv.circle(120, 110, 32, 2.5, inkBlack)
		// Scythe
		cv.line(175, 80, 75, 340, 3, color.RGBA{80, 55, 35, 255})
		cv.arc(140, 85, 42, math.Pi+0.3, 2*math.Pi-0.2, 3, steel)
		cv.glow(140, 65, 10, steel)
		// Skeletal body
		cv.line(120, 145, 120, 245, 3, cream)
		for i := 0; i < 4; i++ {
			y := 165 + float64(i)*18
			cv.arc(120, y, 16, 0, math.Pi, 1.5, cream)
		}
		cv.line(120, 245, 100, 310, 2, cream)
		cv.line(120, 245, 140, 310, 2, cream)
		// Black kimono remnants
		cv.filledRect(105, 160, 30, 50, inkBlack)
		// Cherry petals falling everywhere (beauty in death)
		petals := [][2]float64{{50, 80}, {180, 100}, {40, 200}, {190, 250},
			{65, 300}, {170, 320}, {100, 340}, {155, 350}}
		for _, pt := range petals {
			cv.filledCircle(pt[0], pt[1], 3, sakura)
		}

	case 14: // Temperance → figure pouring between vessels
		// Figure
		cv.filledCircle(120, 120, 22, skin)
		cv.circle(120, 120, 22, 2, inkBlack)
		// Hair
		cv.filledCircle(120, 108, 24, inkBlack)
		// Robes
		for y := 145.0; y < 300; y++ {
			t := (y - 145) / 155
			w := 22 + t*35
			cv.filledRect(120-w, y, w*2, 1, lerpColor(p, s, t*0.3))
		}
		cv.line(98, 300, 142, 300, 2.5, inkBlack)
		cv.line(85, 145, 120-57, 300, 2.5, inkBlack)
		cv.line(155, 145, 120+57, 300, 2.5, inkBlack)
		// Left vessel (held high)
		cv.filledRect(55, 155, 22, 30, ochre)
		cv.boldOutline(55, 155, 22, 30, 2)
		// Right vessel (held low)
		cv.filledRect(163, 210, 22, 30, ochre)
		cv.boldOutline(163, 210, 22, 30, 2)
		// Pouring water stream
		cv.bezier(75, 185, 100, 190, 140, 200, 165, 215, 3, indigo)
		cv.glow(120, 200, 12, indigo)
		// Pool at bottom
		cv.filledRect(45, 330, 150, 15, lerpColor(indigo, sc.bg, 0.3))
		cv.line(45, 330, 195, 330, 2, inkBlack)
		// Wave ripples in pool
		cv.arc(120, 340, 20, math.Pi, 2*math.Pi, 1, indigo)
		cv.arc(100, 340, 15, math.Pi, 2*math.Pi, 1, indigo)

	case 15: // Devil → oni with horns
		// Horns
		cv.bezier(100, 105, 85, 55, 65, 45, 55, 60, 4, vermillion)
		cv.bezier(140, 105, 155, 55, 175, 45, 185, 60, 4, vermillion)
		cv.line(100, 105, 55, 60, 2, inkBlack)
		cv.line(140, 105, 185, 60, 2, inkBlack)
		// Head (red oni)
		cv.filledCircle(120, 120, 30, vermillion)
		cv.circle(120, 120, 30, 2.5, inkBlack)
		// Wild hair
		cv.filledCircle(120, 95, 20, inkBlack)
		// Fierce eyes
		cv.filledCircle(108, 118, 6, cream)
		cv.filledCircle(132, 118, 6, cream)
		cv.filledCircle(108, 118, 3, inkBlack)
		cv.filledCircle(132, 118, 3, inkBlack)
		// Fangs
		cv.filledRect(112, 138, 4, 8, cream)
		cv.filledRect(124, 138, 4, 8, cream)
		// Mouth
		cv.line(105, 136, 135, 136, 2, inkBlack)
		// Body (massive)
		cv.filledRect(80, 155, 80, 120, vermDim)
		cv.boldOutline(80, 155, 80, 120, 3)
		// Tiger-skin loincloth
		cv.filledRect(85, 275, 70, 30, ochre)
		for i := 0; i < 4; i++ {
			x := 95 + float64(i)*15
			cv.line(x, 275, x+5, 300, 2, inkBlack)
		}
		cv.boldOutline(85, 275, 70, 30, 2)
		// Kanabō (iron club)
		cv.filledRect(175, 130, 10, 150, color.RGBA{80, 80, 90, 255})
		cv.boldOutline(175, 130, 10, 150, 2)
		// Studs on club
		for y := 140.0; y < 270; y += 15 {
			cv.filledCircle(180, y, 3, steel)
		}
		// Legs
		cv.filledRect(90, 305, 18, 40, vermDim)
		cv.filledRect(140, 305, 18, 40, vermDim)

	case 16: // Tower → pagoda struck by lightning
		// Five-story pagoda
		for i := 0; i < 5; i++ {
			y := 80 + float64(i)*55
			w := 35 + float64(i)*10
			// Roof (flared)
			cv.filledRect(120-w-10, y, (w+10)*2, 8, vermillion)
			cv.line(120-w-10, y, 120+w+10, y, 2, inkBlack)
			cv.line(120-w-10, y+8, 120+w+10, y+8, 1.5, inkBlack)
			// Wall
			cv.filledRect(120-w+5, y+8, (w-5)*2, 47, cream)
			cv.boldOutline(120-w+5, y+8, (w-5)*2, 47, 1.5)
		}
		// Spire at top
		cv.line(120, 40, 120, 80, 3, ochre)
		cv.filledCircle(120, 38, 4, ochre)
		// Lightning bolt
		cv.line(170, 45, 145, 75, 5, ochre)
		cv.line(145, 75, 165, 85, 5, ochre)
		cv.line(165, 85, 130, 120, 5, ochre)
		cv.glow(155, 70, 20, ochre)
		cv.glow(140, 100, 25, cream)
		// Explosion / fire at top
		cv.glow(120, 95, 25, vermillion)
		// Falling debris
		cv.filledRect(55, 180, 8, 12, color.RGBA{80, 60, 45, 255})
		cv.filledRect(185, 220, 10, 8, color.RGBA{80, 60, 45, 255})
		// Flames at base
		for x := 80.0; x < 160; x += 3 {
			h := 15 + 10*math.Sin(x*0.2)
			for y := 345 - h; y < 345; y++ {
				t := (345 - y) / h
				col := lerpColor(vermillion, ochre, t)
				cv.blend(int(x), int(y), col, 0.7)
			}
		}

	case 17: // Star → figure beneath starfield, water
		// Starfield
		starPos := [][2]float64{{120, 70}, {60, 55}, {180, 60}, {45, 90}, {195, 85},
			{80, 75}, {160, 78}, {100, 58}}
		// Large central star
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			cv.line(120+10*math.Cos(a), 70+10*math.Sin(a),
				120+35*math.Cos(a), 70+35*math.Sin(a), 2, ochre)
		}
		cv.glow(120, 70, 25, ochre)
		cv.filledCircle(120, 70, 8, cream)
		// Smaller stars
		for _, sp := range starPos[1:] {
			cv.glow(sp[0], sp[1], 6, cream)
			cv.filledCircle(sp[0], sp[1], 3, cream)
		}
		// Water / river
		cv.filledRect(30, 280, 180, 70, lerpColor(indigo, sc.bg, 0.3))
		cv.line(30, 280, 210, 280, 2, inkBlack)
		// Wave ripples
		for x := 40.0; x < 210; x += 25 {
			cv.arc(x, 300, 10, math.Pi, 2*math.Pi, 1.5, indigo)
		}
		// Figure kneeling at water's edge
		cv.filledCircle(120, 215, 18, skin)
		cv.circle(120, 215, 18, 2, inkBlack)
		cv.filledCircle(120, 205, 20, inkBlack) // hair
		// Robes
		cv.filledRect(100, 237, 40, 40, cream)
		cv.boldOutline(100, 237, 40, 40, 2)
		// Pouring water from vessel
		cv.filledRect(145, 245, 15, 20, ochre)
		cv.boldOutline(145, 245, 15, 20, 1.5)
		cv.bezier(155, 265, 160, 275, 155, 280, 155, 285, 2, indigo)

	case 18: // Moon → moon over waves and pines
		// Large moon
		cv.filledCircle(120, 80, 40, cream)
		cv.filledCircle(135, 70, 40, sc.bg)
		cv.glow(105, 85, 35, cream)
		// Moon drops
		for x := 60.0; x < 180; x += 15 {
			cv.line(x, 130, x+3, 145, 1.5, cream)
		}
		// Pine trees
		// Left pine
		for i := 0; i < 4; i++ {
			y := 180 + float64(i)*25
			w := 15 + float64(i)*8
			cv.line(55-w, y+20, 55, y, 2.5, bambooDim)
			cv.line(55+w, y+20, 55, y, 2.5, bambooDim)
			cv.filledCircle(55, y+10, w*0.6, bambooDim)
		}
		cv.line(55, 180, 55, 320, 3, color.RGBA{70, 45, 25, 255})
		// Right pine
		for i := 0; i < 4; i++ {
			y := 190 + float64(i)*25
			w := 12 + float64(i)*7
			cv.line(185-w, y+20, 185, y, 2.5, bambooDim)
			cv.line(185+w, y+20, 185, y, 2.5, bambooDim)
			cv.filledCircle(185, y+10, w*0.6, bambooDim)
		}
		cv.line(185, 190, 185, 320, 3, color.RGBA{70, 45, 25, 255})
		// Waves at bottom (bold seigaiha style)
		for row := 0; row < 3; row++ {
			y := 310 + float64(row)*15
			offset := float64(row%2) * 20
			for x := 20.0 + offset; x < 230; x += 40 {
				cv.arc(x, y, 18, math.Pi, 2*math.Pi, 2.5, indigo)
				cv.arc(x, y, 18, math.Pi, 2*math.Pi, 1, inkBlack)
			}
		}

	case 19: // Sun → great rising sun with child
		// Rising sun — half circle at top
		cv.filledCircle(120, 60, 55, vermillion)
		cv.glow(120, 60, 50, vermillion)
		cv.glow(120, 60, 30, ochre)
		// Rays
		for i := 0; i < 16; i++ {
			a := float64(i)*math.Pi/8 + math.Pi
			r1 := 58.0
			r2 := 120.0
			cv.line(120+r1*math.Cos(a), 60+r1*math.Sin(a),
				120+r2*math.Cos(a), 60+r2*math.Sin(a), 3, ochre)
		}
		// Child figure
		cv.filledCircle(120, 245, 16, skin)
		cv.circle(120, 245, 16, 2, inkBlack)
		cv.filledRect(110, 263, 20, 35, cream)
		cv.boldOutline(110, 263, 20, 35, 2)
		// Arms raised joyfully
		cv.line(110, 270, 92, 250, 2, skin)
		cv.line(130, 270, 148, 250, 2, skin)
		// Legs
		cv.filledRect(112, 298, 8, 18, skin)
		cv.filledRect(124, 298, 8, 18, skin)
		// Garden / field
		cv.line(30, 320, 210, 320, 2, inkBlack)
		for x := 40.0; x < 210; x += 20 {
			cv.line(x, 320, x-3, 340, 1.5, bamboo)
			cv.line(x, 320, x+3, 340, 1.5, bamboo)
		}
		// Cherry blossoms
		cv.cherryBlossom(50, 180, 8)
		cv.cherryBlossom(190, 200, 7)

	case 20: // Judgement → figure with taiko drum, rising spirits
		// Taiko drum (large)
		cv.filledCircle(120, 200, 42, vermillion)
		cv.circle(120, 200, 42, 3, inkBlack)
		cv.circle(120, 200, 35, 2, ochre)
		// Drum head pattern
		cv.monCrest(120, 200, 20, ochre)
		// Drum stand
		cv.line(78, 240, 78, 280, 3, color.RGBA{80, 55, 35, 255})
		cv.line(162, 240, 162, 280, 3, color.RGBA{80, 55, 35, 255})
		cv.line(78, 280, 162, 280, 2, color.RGBA{80, 55, 35, 255})
		// Drummer figure
		cv.filledCircle(120, 115, 20, skin)
		cv.circle(120, 115, 20, 2, inkBlack)
		// Hair (topknot)
		cv.filledRect(115, 93, 10, 8, inkBlack)
		cv.filledCircle(120, 90, 5, inkBlack)
		// Robes
		cv.filledRect(100, 140, 40, 55, p)
		cv.boldOutline(100, 140, 40, 55, 2.5)
		// Arms holding drumsticks (bachi)
		cv.line(100, 160, 75, 190, 3, skin)
		cv.line(140, 160, 165, 190, 3, skin)
		cv.line(70, 185, 85, 200, 2, ochre)
		cv.line(170, 185, 155, 200, 2, ochre)
		// Rising spirits/souls above
		for i := 0; i < 3; i++ {
			x := 70 + float64(i)*40
			cv.glow(x, 70, 12, cream)
			cv.filledCircle(x, 70, 6, cream)
			cv.line(x, 78, x, 90, 1, cream)
		}
		// Sound waves
		cv.arc(120, 200, 55, -0.5, 0.5, 1.5, ochre)
		cv.arc(120, 200, 55, math.Pi-0.5, math.Pi+0.5, 1.5, ochre)
		cv.arc(120, 200, 65, -0.3, 0.3, 1, ochreDim)
		cv.arc(120, 200, 65, math.Pi-0.3, math.Pi+0.3, 1, ochreDim)

	case 21: // World → dancer within torii gate circle
		// Large circular torii-inspired frame
		cv.circle(120, 200, 80, 4, vermillion)
		cv.circle(120, 200, 75, 2, vermillion)

		// Torii crossbeams at top
		cv.filledRect(35, 120, 170, 8, vermillion)
		cv.line(35, 120, 205, 120, 2.5, inkBlack)
		cv.line(35, 128, 205, 128, 2.5, inkBlack)
		cv.filledRect(50, 136, 140, 5, vermillion)
		cv.line(50, 136, 190, 136, 1.5, inkBlack)
		cv.line(50, 141, 190, 141, 1.5, inkBlack)

		// Pillars
		cv.filledRect(48, 128, 12, 152, vermillion)
		cv.filledRect(180, 128, 12, 152, vermillion)
		cv.boldOutline(48, 128, 12, 152, 2)
		cv.boldOutline(180, 128, 12, 152, 2)

		// Dancing figure inside circle
		cv.filledCircle(120, 175, 18, skin)
		cv.circle(120, 175, 18, 2, inkBlack)
		// Flowing hair
		cv.bezier(105, 175, 85, 195, 80, 215, 90, 230, 2, inkBlack)
		cv.bezier(135, 175, 155, 195, 160, 215, 150, 230, 2, inkBlack)
		// Dancer's robes (flowing)
		cv.bezier(108, 190, 95, 220, 100, 250, 110, 265, 3, p)
		cv.bezier(132, 190, 145, 220, 140, 250, 130, 265, 3, p)
		cv.filledRect(108, 195, 24, 40, lerpColor(p, s, 0.3))
		// Arms extended in dance
		cv.line(108, 200, 78, 180, 2.5, skin)
		cv.line(132, 200, 162, 180, 2.5, skin)
		// Legs
		cv.line(113, 235, 100, 265, 2, skin)
		cv.line(127, 235, 140, 265, 2, skin)
		// Ribbons / cloth streamers
		cv.bezier(78, 180, 55, 170, 45, 185, 50, 200, 2, s)
		cv.bezier(162, 180, 185, 170, 195, 185, 190, 200, 2, s)

		// Four corner elements (four seasons)
		// Spring — cherry blossom
		cv.cherryBlossom(38, 65, 10)
		// Summer — bamboo
		cv.line(195, 55, 195, 95, 3, bamboo)
		cv.filledRect(192, 70, 6, 3, bambooDim)
		// Autumn — maple (vermillion dot)
		cv.filledCircle(38, 340, 10, vermillion)
		cv.circle(38, 340, 10, 1.5, inkBlack)
		// Winter — wave
		cv.arc(195, 340, 10, math.Pi, 2*math.Pi, 2, indigo)
		cv.arc(195, 340, 10, math.Pi, 2*math.Pi, 1, inkBlack)
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
