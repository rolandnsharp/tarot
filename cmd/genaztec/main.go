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

// ========== aztec color palette ==========

var (
	obsidian    = color.RGBA{20, 22, 30, 255}    // near-black with blue tint
	obsidianDim = color.RGBA{12, 13, 20, 255}    // deeper obsidian
	jade        = color.RGBA{40, 140, 70, 255}   // jade green
	jadeDim     = color.RGBA{25, 90, 45, 255}    // dim jade
	turquoise   = color.RGBA{50, 170, 170, 255}  // turquoise blue-green
	turqDim     = color.RGBA{30, 105, 105, 255}  // dim turquoise
	sunGold     = color.RGBA{210, 170, 40, 255}  // warm sun gold
	goldDim     = color.RGBA{140, 110, 25, 255}  // dim gold
	bloodRed    = color.RGBA{170, 30, 25, 255}   // sacrifice red
	bloodDim    = color.RGBA{110, 20, 18, 255}   // dim blood red
	stone       = color.RGBA{185, 170, 140, 255} // sandstone tan
	stoneDim    = color.RGBA{130, 120, 100, 255}  // dim stone
	jungle      = color.RGBA{30, 85, 35, 255}    // deep jungle green
	jungleDim   = color.RGBA{18, 55, 22, 255}    // dim jungle
	skin        = color.RGBA{170, 120, 75, 255}  // warm brown skin
	skinDim     = color.RGBA{120, 80, 50, 255}   // dim skin
)

// suit colors: Cenote(cups), Atlatl(wands), Tecpatl(swords), Jade(pentacles)
type suitColors struct {
	primary, dim color.RGBA
}

var suitPalette = []suitColors{
	{turquoise, turqDim},   // cups/cenote (index 0)
	{jade, jadeDim},        // wands/atlatl (index 1)
	{stoneDim, obsidian},   // swords/tecpatl (index 2) — obsidian blade
	{jade, jadeDim},        // pentacles/jade disc (index 3)
}

var suitFileNames = []string{"cups", "wands", "swords", "pentacles"}

// ========== aztec visual elements ==========

// steppedBorder draws a xicalcoliuhqui (stepped fret / Greek key) border
func (c *HiResCanvas) steppedBorder(margin float64) {
	w := float64(hiW)
	h := float64(hiH)
	m := margin

	// Outer frame
	c.line(m, m, w-m, m, 3, sunGold)
	c.line(w-m, m, w-m, h-m, 3, sunGold)
	c.line(w-m, h-m, m, h-m, 3, sunGold)
	c.line(m, h-m, m, m, 3, sunGold)

	// Inner frame
	m2 := margin + 6
	c.line(m2, m2, w-m2, m2, 1.5, goldDim)
	c.line(w-m2, m2, w-m2, h-m2, 1.5, goldDim)
	c.line(w-m2, h-m2, m2, h-m2, 1.5, goldDim)
	c.line(m2, h-m2, m2, m2, 1.5, goldDim)

	// Stepped fret motifs along top border
	step := 14.0
	for x := m2 + 8; x < w-m2-step*2; x += step * 2 {
		y := m2 + 1
		// Step pattern: right, down, right, up
		c.line(x, y, x+step, y, 1.5, sunGold)
		c.line(x+step, y, x+step, y+step*0.5, 1.5, sunGold)
		c.line(x+step, y+step*0.5, x+step*1.5, y+step*0.5, 1.5, sunGold)
		c.line(x+step*1.5, y+step*0.5, x+step*1.5, y-1, 1.5, sunGold)
	}
	// Stepped fret motifs along bottom border
	for x := m2 + 8; x < w-m2-step*2; x += step * 2 {
		y := h - m2 - 1
		c.line(x, y, x+step, y, 1.5, sunGold)
		c.line(x+step, y, x+step, y-step*0.5, 1.5, sunGold)
		c.line(x+step, y-step*0.5, x+step*1.5, y-step*0.5, 1.5, sunGold)
		c.line(x+step*1.5, y-step*0.5, x+step*1.5, y+1, 1.5, sunGold)
	}
}

// pyramid draws a stepped pyramid silhouette
func (c *HiResCanvas) pyramid(cx, baseY, w, h float64, col color.RGBA) {
	levels := 5
	levelH := h / float64(levels)
	for i := 0; i < levels; i++ {
		t := float64(i) / float64(levels)
		lw := w * (1 - t*0.6)
		ly := baseY - float64(i+1)*levelH
		c.filledRect(cx-lw/2, ly, lw, levelH, col)
		// Step edge line
		c.line(cx-lw/2, ly, cx+lw/2, ly, 1.5, lerpColor(col, sunGold, 0.3))
	}
	// Bold outline
	for i := 0; i < levels; i++ {
		t := float64(i) / float64(levels)
		lw := w * (1 - t*0.6)
		ly := baseY - float64(i+1)*levelH
		c.line(cx-lw/2, ly, cx-lw/2, ly+levelH, 1.5, obsidian)
		c.line(cx+lw/2, ly, cx+lw/2, ly+levelH, 1.5, obsidian)
	}
}

// serpentCurve draws a feathered serpent body curve
func (c *HiResCanvas) serpentCurve(x1, y1, x2, y2 float64, col color.RGBA) {
	mx := (x1 + x2) / 2
	my := (y1 + y2) / 2
	dx := x2 - x1
	c.bezier(x1, y1, x1+dx*0.3, y1-20, mx, my+20, x2, y2, 3, col)
	// Scales along the curve
	steps := 8
	for i := 1; i < steps; i++ {
		t := float64(i) / float64(steps)
		px, py := bezierPoint(t, x1, y1, x1+dx*0.3, y1-20, mx, my+20, x2, y2)
		c.filledCircle(px, py, 2, lerpColor(col, sunGold, 0.3))
	}
}

// sunDisc draws an Aztec sun stone / calendar disc
func (c *HiResCanvas) sunDisc(cx, cy, r float64) {
	// Outer ring
	c.filledCircle(cx, cy, r, goldDim)
	c.circle(cx, cy, r, 2.5, sunGold)
	// Inner rings
	c.circle(cx, cy, r*0.7, 2, sunGold)
	c.circle(cx, cy, r*0.4, 1.5, sunGold)
	// Center face
	c.filledCircle(cx, cy, r*0.35, sunGold)
	// Eyes
	c.filledCircle(cx-r*0.12, cy-r*0.05, r*0.06, obsidian)
	c.filledCircle(cx+r*0.12, cy-r*0.05, r*0.06, obsidian)
	// Mouth
	c.line(cx-r*0.1, cy+r*0.12, cx+r*0.1, cy+r*0.12, 1.5, obsidian)
	// Radiating triangular rays
	for i := 0; i < 16; i++ {
		a := float64(i) * math.Pi / 8
		r1 := r * 0.45
		r2 := r * 0.65
		c.line(cx+r1*math.Cos(a), cy+r1*math.Sin(a),
			cx+r2*math.Cos(a), cy+r2*math.Sin(a), 2, sunGold)
	}
	// Cardinal direction markers
	for i := 0; i < 4; i++ {
		a := float64(i) * math.Pi / 2
		r1 := r * 0.72
		r2 := r * 0.95
		c.line(cx+r1*math.Cos(a), cy+r1*math.Sin(a),
			cx+r2*math.Cos(a), cy+r2*math.Sin(a), 3, sunGold)
	}
}

// glyphBlock draws an abstract Mayan glyph rectangle
func (c *HiResCanvas) glyphBlock(cx, cy, size float64, col color.RGBA) {
	s := size / 2
	c.filledRect(cx-s, cy-s, size, size, col)
	// Inner stepped detail
	c.filledRect(cx-s*0.6, cy-s*0.6, size*0.6, size*0.6, lerpColor(col, sunGold, 0.3))
	// Border
	c.line(cx-s, cy-s, cx+s, cy-s, 1.5, obsidian)
	c.line(cx+s, cy-s, cx+s, cy+s, 1.5, obsidian)
	c.line(cx+s, cy+s, cx-s, cy+s, 1.5, obsidian)
	c.line(cx-s, cy+s, cx-s, cy-s, 1.5, obsidian)
	// Dot eyes
	c.filledCircle(cx-s*0.2, cy-s*0.1, 2, obsidian)
	c.filledCircle(cx+s*0.2, cy-s*0.1, 2, obsidian)
}

// jaguarSpots draws a jaguar spot pattern fill
func (c *HiResCanvas) jaguarSpots(cx, cy, r float64) {
	col := color.RGBA{50, 35, 15, 255} // dark brown spots
	spots := [][2]float64{
		{-0.4, -0.3}, {0.3, -0.5}, {0.5, 0.1}, {-0.2, 0.4},
		{0.1, -0.1}, {-0.5, 0.1}, {0.4, 0.4}, {-0.1, -0.5},
	}
	for _, s := range spots {
		sx := cx + s[0]*r
		sy := cy + s[1]*r
		c.filledCircle(sx, sy, r*0.12, col)
		c.circle(sx, sy, r*0.12, 1, color.RGBA{30, 20, 8, 255})
	}
}

// jungleVine draws a hanging vine with leaves
func (c *HiResCanvas) jungleVine(x, y1, y2 float64) {
	// Main vine
	c.bezier(x, y1, x+8, y1+(y2-y1)*0.3, x-8, y1+(y2-y1)*0.6, x, y2, 2, jungle)
	// Leaves
	for i := 0; i < 4; i++ {
		t := float64(i+1) / 5
		ly := y1 + (y2-y1)*t
		lx := x + 6*math.Sin(t*math.Pi*2)
		// Simple leaf: two lines forming a pointed oval
		c.line(lx, ly, lx+8, ly-3, 2, jade)
		c.line(lx, ly, lx+8, ly+3, 2, jade)
	}
}

// serpentHead draws a simplified feathered serpent head
func (c *HiResCanvas) serpentHead(cx, cy, size float64, col color.RGBA, facingRight bool) {
	dir := 1.0
	if !facingRight {
		dir = -1
	}
	// Head
	c.filledCircle(cx, cy, size*0.4, col)
	// Snout
	c.filledRect(cx+dir*size*0.2, cy-size*0.15, size*0.4, size*0.3, col)
	// Eye
	c.filledCircle(cx+dir*size*0.1, cy-size*0.08, size*0.08, sunGold)
	c.filledCircle(cx+dir*size*0.1, cy-size*0.08, size*0.04, obsidian)
	// Jaw/fangs
	c.line(cx+dir*size*0.6, cy, cx+dir*size*0.6, cy+size*0.15, 1.5, obsidian)
	// Feather crest
	for i := 0; i < 3; i++ {
		a := math.Pi/2 + float64(i)*0.4 - 0.4
		fx := cx - dir*size*0.1 + size*0.5*math.Cos(a)*(-dir)
		fy := cy - size*0.15 - size*0.4*math.Sin(a)
		c.line(cx-dir*size*0.1, cy-size*0.2, fx, fy, 2, jade)
	}
}

// boldOutline draws a thick outline rectangle
func (c *HiResCanvas) boldOutline(x, y, w, h, thickness float64) {
	c.line(x, y, x+w, y, thickness, obsidian)
	c.line(x+w, y, x+w, y+h, thickness, obsidian)
	c.line(x+w, y+h, x, y+h, thickness, obsidian)
	c.line(x, y+h, x, y, thickness, obsidian)
}

// ========== suit pip shapes (Mesoamerican style) ==========

func (c *HiResCanvas) drawCup(cx, cy, size float64, col color.RGBA) {
	// Cenote — round vessel/pot with water ripple
	w := size * 0.4
	h := size * 0.35
	// Pot body
	c.filledRect(cx-w, cy-h*0.3, w*2, h*1.3, col)
	c.arc(cx, cy+h, w, 0, math.Pi, 2, col)
	// Rim
	c.filledRect(cx-w*1.1, cy-h*0.3, w*2.2, size*0.08, lerpColor(col, sunGold, 0.4))
	// Water ripple inside
	c.arc(cx, cy+h*0.2, w*0.5, math.Pi, 2*math.Pi, 1.5, lerpColor(col, stone, 0.3))
	// Bold outline
	c.line(cx-w, cy-h*0.3, cx-w, cy+h, 2, obsidian)
	c.line(cx+w, cy-h*0.3, cx+w, cy+h, 2, obsidian)
	c.arc(cx, cy+h, w, 0, math.Pi, 2, obsidian)
	c.line(cx-w*1.1, cy-h*0.3, cx+w*1.1, cy-h*0.3, 2, obsidian)
}

func (c *HiResCanvas) drawWand(cx, cy, size float64, col color.RGBA) {
	// Atlatl — diagonal spear with feathered end
	// Shaft (diagonal)
	c.line(cx-size*0.35, cy+size*0.35, cx+size*0.35, cy-size*0.35, 2.5, col)
	// Spear point (obsidian tip)
	c.line(cx+size*0.35, cy-size*0.35, cx+size*0.45, cy-size*0.5, 3, stoneDim)
	// Feathered end
	c.line(cx-size*0.35, cy+size*0.35, cx-size*0.5, cy+size*0.3, 2, bloodRed)
	c.line(cx-size*0.35, cy+size*0.35, cx-size*0.5, cy+size*0.45, 2, jade)
	c.line(cx-size*0.35, cy+size*0.35, cx-size*0.45, cy+size*0.5, 2, sunGold)
}

func (c *HiResCanvas) drawSword(cx, cy, size float64, col color.RGBA) {
	// Tecpatl — obsidian flint knife (leaf shape, pointed)
	// Blade (leaf-shaped)
	for i := 0; i <= 100; i++ {
		t := float64(i) / 100
		py := cy - size*0.45 + t*size*0.65
		w := math.Sin(t*math.Pi) * size * 0.18
		c.filledRect(cx-w, py, w*2, 1, col)
	}
	// Blade sheen
	for i := 0; i <= 80; i++ {
		t := float64(i) / 80
		py := cy - size*0.35 + t*size*0.5
		w := math.Sin(t*math.Pi) * size * 0.06
		c.filledRect(cx-size*0.04-w, py, w*2, 1, lerpColor(col, stone, 0.5))
	}
	// Handle (wrapped)
	c.filledRect(cx-size*0.06, cy+size*0.2, size*0.12, size*0.25, skinDim)
	// Handle wrapping lines
	for i := 0; i < 3; i++ {
		wy := cy + size*0.22 + float64(i)*size*0.07
		c.line(cx-size*0.06, wy, cx+size*0.06, wy, 1, bloodRed)
	}
	// Bold outline
	c.line(cx, cy-size*0.45, cx-size*0.18, cy+size*0.1, 1.5, obsidian)
	c.line(cx, cy-size*0.45, cx+size*0.18, cy+size*0.1, 1.5, obsidian)
}

func (c *HiResCanvas) drawPentacle(cx, cy, size float64, col color.RGBA) {
	// Jade disc — circular with center dot and ring
	c.filledCircle(cx, cy, size*0.38, col)
	c.circle(cx, cy, size*0.38, 2.5, obsidian)
	// Inner ring
	c.circle(cx, cy, size*0.22, 2, lerpColor(col, sunGold, 0.4))
	// Center dot
	c.filledCircle(cx, cy, size*0.08, lerpColor(col, sunGold, 0.5))
	// Cardinal notches
	for i := 0; i < 4; i++ {
		a := float64(i) * math.Pi / 2
		nx := cx + size*0.3*math.Cos(a)
		ny := cy + size*0.3*math.Sin(a)
		c.filledCircle(nx, ny, 2, obsidian)
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

// ========== aztec backgrounds ==========

func aztecBG() *HiResCanvas {
	cv := newCanvas()
	cv.fillGradient(
		color.RGBA{35, 40, 30, 255},  // dark jungle top
		color.RGBA{20, 25, 18, 255},  // darker bottom
	)
	return cv
}

func stoneBG() *HiResCanvas {
	cv := newCanvas()
	cv.fillSolid(stone)
	// Subtle texture — scattered darker pixels
	for y := 0; y < hiH; y += 7 {
		for x := 0; x < hiW; x += 11 {
			cv.blend(x, y, stoneDim, 0.15)
		}
	}
	return cv
}

// ========== card generators ==========

func genBack() *HiResCanvas {
	cv := newCanvas()
	cv.fillSolid(obsidian)

	// Stone texture background
	for y := 0; y < hiH; y += 5 {
		for x := 0; x < hiW; x += 7 {
			cv.blend(x, y, color.RGBA{30, 32, 38, 255}, 0.2)
		}
	}

	// Stepped fret border
	cv.steppedBorder(8)

	// Central sun stone
	cv.sunDisc(120, 200, 55)
	cv.glow(120, 200, 35, sunGold)

	// Serpent in each corner
	cv.serpentHead(35, 35, 25, jade, true)
	cv.serpentHead(205, 35, 25, jade, false)
	cv.serpentHead(35, 365, 25, jade, true)
	cv.serpentHead(205, 365, 25, jade, false)

	return cv
}

func genPip(si, num int) *HiResCanvas {
	cv := stoneBG()
	sc := suitPalette[si]

	// Border
	cv.steppedBorder(6)

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
	cv := stoneBG()
	sc := suitPalette[si]

	// Border
	cv.steppedBorder(5)

	p := sc.primary

	switch rank {
	case 11: // Page → young warrior/acolyte
		// Background panel
		cv.filledRect(30, 50, 180, 290, lerpColor(stone, p, 0.1))
		cv.boldOutline(30, 50, 180, 290, 2)
		// Head
		cv.filledCircle(120, 110, 25, skin)
		cv.circle(120, 110, 25, 2, obsidian)
		// Short dark hair
		cv.arc(120, 100, 26, math.Pi, 2*math.Pi, 4, obsidian)
		// Simple tunic
		cv.filledRect(95, 140, 50, 100, p)
		cv.boldOutline(95, 140, 50, 100, 2.5)
		// Loincloth/maxtlatl
		cv.filledRect(100, 240, 40, 20, lerpColor(p, sunGold, 0.3))
		cv.boldOutline(100, 240, 40, 20, 1.5)
		// Legs
		cv.filledRect(100, 260, 16, 50, skin)
		cv.filledRect(128, 260, 16, 50, skin)
		// Sandals
		cv.filledRect(98, 305, 20, 6, skinDim)
		cv.filledRect(126, 305, 20, 6, skinDim)
		// Suit symbol held
		cv.drawSuitPip(si, 170, 190, 25)
		// Small glyph accents
		cv.glyphBlock(50, 75, 16, goldDim)
		cv.glyphBlock(190, 75, 16, goldDim)

	case 12: // Knight → jaguar warrior
		cv.filledRect(30, 50, 180, 290, lerpColor(stone, p, 0.1))
		cv.boldOutline(30, 50, 180, 290, 2)
		// Jaguar helmet/head
		cv.filledCircle(110, 90, 30, color.RGBA{190, 160, 60, 255}) // jaguar gold
		cv.jaguarSpots(110, 90, 30)
		cv.circle(110, 90, 30, 2, obsidian)
		// Jaguar ears
		cv.filledCircle(88, 65, 8, color.RGBA{190, 160, 60, 255})
		cv.filledCircle(132, 65, 8, color.RGBA{190, 160, 60, 255})
		// Face visible inside jaws
		cv.filledCircle(110, 98, 15, skin)
		// Warrior body — quilted armor (ichcahuipilli)
		cv.filledRect(85, 125, 50, 100, p)
		cv.boldOutline(85, 125, 50, 100, 2.5)
		// Armor quilting
		for y := 135.0; y < 220; y += 15 {
			cv.line(85, y, 135, y, 1, lerpColor(p, obsidian, 0.2))
		}
		// Shield (chimalli)
		cv.filledCircle(165, 170, 22, bloodRed)
		cv.circle(165, 170, 22, 2, obsidian)
		cv.circle(165, 170, 14, 1.5, sunGold)
		cv.filledCircle(165, 170, 6, sunGold)
		// Weapon arm
		cv.line(85, 165, 55, 140, 3, skin)
		// Suit symbol as weapon
		cv.drawSuitPip(si, 45, 125, 22)
		// Legs
		cv.filledRect(90, 225, 16, 65, skin)
		cv.filledRect(115, 225, 16, 65, skin)

	case 13: // Queen → noble woman / priestess
		cv.filledRect(30, 50, 180, 290, lerpColor(stone, p, 0.1))
		cv.boldOutline(30, 50, 180, 290, 2)
		// Elaborate headdress
		cv.filledRect(95, 50, 50, 30, jade)
		cv.filledRect(90, 55, 60, 8, sunGold)
		// Feathers in headdress
		cv.line(110, 50, 100, 30, 2, jade)
		cv.line(120, 50, 120, 25, 2, bloodRed)
		cv.line(130, 50, 140, 30, 2, turquoise)
		// Face
		cv.filledCircle(120, 92, 20, skin)
		cv.circle(120, 92, 20, 2, obsidian)
		// Jade ear spools
		cv.filledCircle(98, 92, 5, jade)
		cv.filledCircle(142, 92, 5, jade)
		// Flowing huipil (dress)
		for y := 118.0; y < 320; y++ {
			t := (y - 118) / 202
			w := 25 + t*55
			col := lerpColor(p, sc.dim, t*0.4)
			cv.filledRect(120-w, y, w*2, 1, col)
		}
		// Dress border pattern
		cv.filledRect(75, 290, 90, 6, sunGold)
		// Necklace
		cv.arc(120, 115, 18, 0.3, math.Pi-0.3, 2, turquoise)
		// Central jade pendant
		cv.filledCircle(120, 132, 5, jade)
		// Suit symbol
		cv.drawSuitPip(si, 170, 260, 25)
		// Bold outline bottom
		cv.line(45, 320, 195, 320, 2.5, obsidian)

	case 14: // King → tlatoani with feathered headdress
		cv.filledRect(30, 50, 180, 290, lerpColor(stone, p, 0.1))
		cv.boldOutline(30, 50, 180, 290, 2)
		// Grand feathered headdress (copilli)
		for i := 0; i < 9; i++ {
			a := math.Pi + float64(i)*math.Pi/10 + math.Pi/10
			fx := 120 + 55*math.Cos(a)
			fy := 80 + 55*math.Sin(a)
			cols := []color.RGBA{jade, bloodRed, turquoise, sunGold, jade, bloodRed, turquoise, sunGold, jade}
			cv.line(120, 80, fx, fy, 3, cols[i])
		}
		// Headdress base band
		cv.filledRect(95, 75, 50, 10, sunGold)
		// Head
		cv.filledCircle(120, 95, 22, skin)
		cv.circle(120, 95, 22, 2, obsidian)
		// Jade ear spools
		cv.filledCircle(96, 95, 6, jade)
		cv.filledCircle(144, 95, 6, jade)
		// Nose plug (gold)
		cv.filledCircle(120, 100, 3, sunGold)
		// Broad royal robes/tilmatli
		cv.filledRect(65, 125, 110, 120, p)
		cv.boldOutline(65, 125, 110, 120, 3)
		// Robe border pattern
		cv.filledRect(65, 235, 110, 8, sunGold)
		for x := 70.0; x < 170; x += 15 {
			cv.filledRect(x, 237, 6, 4, bloodRed)
		}
		// Inner robe / chest emblem
		cv.sunDisc(120, 175, 22)
		// Scepter
		cv.filledRect(172, 130, 6, 80, sunGold)
		cv.boldOutline(172, 130, 6, 80, 1.5)
		// Legs
		cv.filledRect(85, 245, 22, 55, skin)
		cv.filledRect(133, 245, 22, 55, skin)
		// Suit symbol
		cv.drawSuitPip(si, 52, 290, 22)
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
	{jade, sunGold, color.RGBA{30, 45, 28, 255}},               // 0  Fool → adventurer entering jungle
	{sunGold, obsidian, color.RGBA{35, 30, 22, 255}},            // 1  Magician → shaman
	{turquoise, stone, color.RGBA{22, 35, 40, 255}},             // 2  High Priestess → Ix Chel
	{jade, bloodRed, color.RGBA{30, 35, 25, 255}},               // 3  Empress → Coatlicue
	{sunGold, bloodRed, color.RGBA{40, 30, 20, 255}},            // 4  Emperor → tlatoani
	{sunGold, stone, color.RGBA{40, 35, 25, 255}},               // 5  Hierophant → high priest
	{bloodRed, jade, color.RGBA{35, 30, 25, 255}},               // 6  Lovers → before temple
	{turquoise, sunGold, color.RGBA{25, 35, 35, 255}},           // 7  Chariot → serpent canoe
	{sunGold, bloodRed, color.RGBA{38, 35, 20, 255}},            // 8  Strength → wrestling jaguar
	{stone, sunGold, color.RGBA{30, 28, 25, 255}},               // 9  Hermit → shaman in cave
	{sunGold, bloodRed, color.RGBA{35, 30, 22, 255}},            // 10 Wheel → sun stone
	{stoneDim, sunGold, color.RGBA{30, 28, 30, 255}},            // 11 Justice → obsidian blade
	{turquoise, stone, color.RGBA{20, 30, 35, 255}},             // 12 Hanged Man → over cenote
	{stone, bloodRed, color.RGBA{28, 22, 20, 255}},              // 13 Death → Mictlantecuhtli
	{turquoise, jade, color.RGBA{22, 30, 32, 255}},              // 14 Temperance → pouring water
	{obsidian, bloodRed, color.RGBA{25, 20, 25, 255}},           // 15 Devil → Tezcatlipoca
	{stone, jade, color.RGBA{32, 30, 22, 255}},                  // 16 Tower → pyramid crumbling
	{sunGold, turquoise, color.RGBA{18, 22, 30, 255}},           // 17 Star → Venus over jungle
	{turquoise, sunGold, color.RGBA{18, 22, 28, 255}},           // 18 Moon → moon over jungle
	{sunGold, bloodRed, color.RGBA{50, 40, 18, 255}},            // 19 Sun → Tonatiuh
	{jade, sunGold, color.RGBA{25, 35, 28, 255}},                // 20 Judgement → Quetzalcoatl
	{sunGold, jade, color.RGBA{30, 30, 25, 255}},                // 21 World → dancer in calendar
}

func genMajor(num int) *HiResCanvas {
	cv := newCanvas()
	sc := majorSchemes[num]
	p, s := sc.primary, sc.secondary

	cv.fillSolid(sc.bg)

	// Number in corner — scale 10
	cv.drawNum(15, 15, num, sunGold, 10)

	// Border
	cv.steppedBorder(5)

	// Scene panel
	cv.filledRect(25, 55, 190, 305, lerpColor(sc.bg, stone, 0.08))
	cv.boldOutline(25, 55, 190, 305, 2)

	switch num {
	case 0: // Fool → young adventurer entering jungle
		// Jungle canopy at top
		for x := 25.0; x < 215; x += 12 {
			r := 20 + 8*math.Sin(x*0.15)
			cv.filledCircle(x, 75, r, jungle)
		}
		for x := 30.0; x < 210; x += 15 {
			r := 15 + 5*math.Cos(x*0.2)
			cv.filledCircle(x, 85, r, jade)
		}
		// Jungle vines
		cv.jungleVine(55, 60, 160)
		cv.jungleVine(185, 60, 140)
		// Path into jungle
		cv.bezier(120, 350, 115, 300, 125, 250, 120, 200, 3, stoneDim)
		// Figure (young adventurer)
		cv.filledCircle(120, 195, 22, skin)
		cv.circle(120, 195, 22, 2, obsidian)
		// Simple headband
		cv.filledRect(100, 185, 40, 5, bloodRed)
		// Hair
		cv.arc(120, 185, 23, math.Pi, 2*math.Pi, 3, obsidian)
		// Simple tunic
		cv.filledRect(100, 220, 40, 60, p)
		cv.boldOutline(100, 220, 40, 60, 2.5)
		// Walking stick
		cv.line(148, 230, 155, 340, 2.5, skinDim)
		// Bundle
		cv.filledCircle(150, 225, 12, s)
		cv.circle(150, 225, 12, 2, obsidian)
		// Legs
		cv.filledRect(104, 280, 14, 40, skin)
		cv.filledRect(122, 280, 14, 40, skin)
		// Macaw parrot on shoulder
		cv.filledCircle(90, 210, 8, bloodRed)
		cv.filledCircle(85, 205, 5, sunGold)

	case 1: // Magician → shaman with cacao and incense
		// Altar / table
		cv.filledRect(45, 255, 150, 10, stone)
		cv.line(45, 255, 195, 255, 2.5, obsidian)
		// Four suit elements on altar
		cv.drawSuitPip(0, 65, 290, 22)
		cv.drawSuitPip(1, 105, 290, 22)
		cv.drawSuitPip(2, 145, 290, 22)
		cv.drawSuitPip(3, 185, 290, 22)
		// Figure (shaman)
		cv.filledCircle(120, 130, 22, skin)
		cv.circle(120, 130, 22, 2, obsidian)
		// Feathered headdress
		cv.line(120, 108, 100, 75, 3, jade)
		cv.line(120, 108, 115, 78, 2, bloodRed)
		cv.line(120, 108, 130, 80, 2, turquoise)
		cv.line(120, 108, 140, 75, 3, sunGold)
		// Robes
		cv.filledRect(95, 155, 50, 95, p)
		cv.boldOutline(95, 155, 50, 95, 2.5)
		// Arms raised
		cv.line(95, 175, 55, 145, 3, skin)
		cv.line(145, 175, 185, 145, 3, skin)
		// Cacao pod in left hand
		cv.filledCircle(50, 140, 8, skinDim)
		// Incense smoke from right hand
		cv.glow(188, 140, 10, stone)
		cv.glow(185, 125, 8, stoneDim)
		cv.glow(190, 110, 6, stone)
		// Glyph behind head
		cv.glyphBlock(120, 68, 20, goldDim)

	case 2: // High Priestess → moon goddess Ix Chel
		// Moon behind
		cv.filledCircle(120, 80, 40, turquoise)
		cv.filledCircle(135, 72, 38, sc.bg)
		cv.glow(108, 85, 30, turquoise)
		// Figure
		cv.filledCircle(120, 145, 22, skin)
		cv.circle(120, 145, 22, 2, obsidian)
		// Moon headdress
		cv.arc(120, 130, 28, math.Pi, 2*math.Pi, 3, turquoise)
		cv.filledCircle(120, 125, 5, turquoise)
		// Long hair
		cv.bezier(100, 145, 85, 180, 80, 230, 85, 260, 3, obsidian)
		cv.bezier(140, 145, 155, 180, 160, 230, 155, 260, 3, obsidian)
		// Flowing dress
		for y := 170.0; y < 330; y++ {
			t := (y - 170) / 160
			w := 22 + t*50
			col := lerpColor(p, s, t*0.3)
			cv.filledRect(120-w, y, w*2, 1, col)
		}
		cv.line(50, 330, 190, 330, 2.5, obsidian)
		// Water vessel (pouring)
		cv.filledRect(155, 200, 18, 25, stone)
		cv.boldOutline(155, 200, 18, 25, 1.5)
		cv.bezier(170, 225, 175, 240, 170, 260, 165, 280, 2, turquoise)
		// Rabbit (moon rabbit symbol)
		cv.filledCircle(70, 250, 10, stone)
		cv.filledCircle(67, 240, 5, stone)
		cv.circle(70, 250, 10, 1.5, obsidian)

	case 3: // Empress → Coatlicue (earth/fertility goddess)
		// Serpent skirt base
		for i := 0; i < 6; i++ {
			x := 50 + float64(i)*28
			cv.serpentCurve(x, 290, x+15, 350, jade)
		}
		// Figure — massive, frontal
		cv.filledCircle(120, 130, 28, skin)
		cv.circle(120, 130, 28, 2, obsidian)
		// Skull necklace (Coatlicue's iconic feature)
		for i := 0; i < 5; i++ {
			nx := 80 + float64(i)*20
			cv.filledCircle(nx, 170, 7, stone)
			cv.filledCircle(nx-2, 168, 2, obsidian) // eye
			cv.filledCircle(nx+2, 168, 2, obsidian) // eye
		}
		// Grand headdress
		cv.filledRect(85, 95, 70, 12, sunGold)
		cv.line(100, 95, 90, 65, 3, jade)
		cv.line(120, 95, 120, 60, 3, bloodRed)
		cv.line(140, 95, 150, 65, 3, jade)
		// Broad body / earth form
		for y := 180.0; y < 290; y++ {
			t := (y - 180) / 110
			w := 40 + t*45
			col := lerpColor(p, s, t*0.4)
			cv.filledRect(120-w, y, w*2, 1, col)
		}
		cv.line(35, 290, 205, 290, 2.5, obsidian)
		// Clawed hands at sides
		cv.line(70, 200, 45, 230, 3, p)
		cv.line(170, 200, 195, 230, 3, p)

	case 4: // Emperor → Aztec tlatoani on jaguar throne
		// Jaguar throne
		cv.filledRect(40, 230, 160, 100, color.RGBA{190, 160, 60, 255})
		cv.jaguarSpots(120, 280, 80)
		cv.boldOutline(40, 230, 160, 100, 2.5)
		// Throne armrests — jaguar heads
		cv.filledCircle(55, 240, 15, color.RGBA{190, 160, 60, 255})
		cv.circle(55, 240, 15, 2, obsidian)
		cv.filledCircle(185, 240, 15, color.RGBA{190, 160, 60, 255})
		cv.circle(185, 240, 15, 2, obsidian)
		// Figure
		cv.filledCircle(120, 118, 24, skin)
		cv.circle(120, 118, 24, 2, obsidian)
		// Grand feathered headdress
		for i := 0; i < 7; i++ {
			a := math.Pi + float64(i)*math.Pi/8 + math.Pi/8
			fx := 120 + 50*math.Cos(a)
			fy := 95 + 45*math.Sin(a)
			cols := []color.RGBA{jade, bloodRed, turquoise, sunGold, turquoise, bloodRed, jade}
			cv.line(120, 95, fx, fy, 3, cols[i])
		}
		cv.filledRect(97, 93, 46, 8, sunGold)
		// Nose plug
		cv.filledCircle(120, 123, 3, sunGold)
		// Broad royal tilmatli
		cv.filledRect(75, 148, 90, 80, p)
		cv.boldOutline(75, 148, 90, 80, 3)
		// Scepter
		cv.filledRect(170, 155, 6, 70, sunGold)
		cv.boldOutline(170, 155, 6, 70, 1.5)

	case 5: // Hierophant → high priest atop pyramid
		// Pyramid
		cv.pyramid(120, 350, 180, 180, stoneDim)
		// Temple at top of pyramid
		cv.filledRect(95, 165, 50, 40, stone)
		cv.boldOutline(95, 165, 50, 40, 2)
		// Temple door
		cv.filledRect(112, 180, 16, 25, obsidian)
		// Fire atop temple
		cv.glow(120, 155, 15, sunGold)
		cv.glow(120, 150, 10, bloodRed)
		// Figure (priest) standing before temple
		cv.filledCircle(120, 130, 18, skin)
		cv.circle(120, 130, 18, 2, obsidian)
		// Priestly headdress
		cv.filledRect(105, 110, 30, 8, sunGold)
		cv.line(115, 110, 110, 88, 2, jade)
		cv.line(125, 110, 130, 88, 2, jade)
		// Robes
		cv.filledRect(102, 150, 36, 50, s)
		cv.boldOutline(102, 150, 36, 50, 2)
		// Incense burner
		cv.filledRect(155, 150, 12, 20, stone)
		cv.glow(161, 145, 8, sunGold)
		// Stairs on pyramid
		cv.filledRect(112, 205, 16, 145, lerpColor(stone, stoneDim, 0.5))

	case 6: // Lovers → two figures before temple, entwined serpents
		// Temple in background
		cv.filledRect(70, 55, 100, 50, stone)
		cv.boldOutline(70, 55, 100, 50, 2)
		cv.filledRect(105, 75, 30, 30, obsidian) // doorway
		// Entwined serpents above
		cv.serpentCurve(60, 65, 120, 55, jade)
		cv.serpentCurve(120, 55, 180, 65, bloodRed)
		// Left figure (man)
		cv.filledCircle(85, 170, 20, skin)
		cv.circle(85, 170, 20, 2, obsidian)
		cv.arc(85, 160, 21, math.Pi, 2*math.Pi, 3, obsidian)
		cv.filledRect(68, 195, 34, 80, jade)
		cv.boldOutline(68, 195, 34, 80, 2)
		// Right figure (woman)
		cv.filledCircle(155, 170, 20, skin)
		cv.circle(155, 170, 20, 2, obsidian)
		cv.filledRect(148, 155, 14, 25, obsidian) // hair
		cv.filledCircle(165, 158, 4, turquoise) // earring
		cv.filledRect(138, 195, 34, 80, bloodRed)
		cv.boldOutline(138, 195, 34, 80, 2)
		// Hands reaching
		cv.line(102, 230, 118, 230, 2.5, skin)
		cv.line(138, 230, 122, 230, 2.5, skin)
		cv.glow(120, 230, 8, sunGold)
		// Flowers on ground
		cv.filledCircle(60, 310, 5, bloodRed)
		cv.filledCircle(180, 310, 5, sunGold)
		cv.line(30, 300, 210, 300, 2, obsidian)

	case 7: // Chariot → warrior in feathered serpent canoe
		// Water
		cv.filledRect(25, 260, 190, 100, lerpColor(turquoise, sc.bg, 0.5))
		for x := 30.0; x < 210; x += 25 {
			cv.arc(x, 270, 10, math.Pi, 2*math.Pi, 1.5, turquoise)
		}
		// Serpent canoe
		cv.arc(120, 280, 70, 0.15, math.Pi-0.15, 4, jade)
		cv.filledRect(55, 260, 130, 12, skinDim)
		cv.boldOutline(55, 260, 130, 12, 2)
		// Serpent head at prow
		cv.serpentHead(185, 255, 30, jade, true)
		// Serpent tail at stern
		cv.bezier(55, 265, 40, 255, 35, 240, 45, 230, 3, jade)
		// Warrior figure standing in canoe
		cv.filledCircle(120, 175, 22, skin)
		cv.circle(120, 175, 22, 2, obsidian)
		// Feathered headdress
		cv.line(120, 153, 105, 120, 3, jade)
		cv.line(120, 153, 120, 118, 2, bloodRed)
		cv.line(120, 153, 135, 120, 3, turquoise)
		// Armor
		cv.filledRect(100, 200, 40, 55, p)
		cv.boldOutline(100, 200, 40, 55, 2.5)
		// Shield
		cv.filledCircle(160, 220, 18, s)
		cv.circle(160, 220, 18, 2, obsidian)
		// Spear
		cv.line(82, 195, 75, 130, 2.5, skinDim)

	case 8: // Strength → figure wrestling jaguar
		// Figure
		cv.filledCircle(85, 145, 20, skin)
		cv.circle(85, 145, 20, 2, obsidian)
		// Headband
		cv.filledRect(67, 137, 36, 5, bloodRed)
		// Robes
		cv.filledRect(68, 170, 34, 80, stone)
		cv.boldOutline(68, 170, 34, 80, 2.5)
		// Hand on jaguar
		cv.line(102, 190, 130, 210, 3, skin)
		// Jaguar (bold)
		cv.filledCircle(158, 230, 35, color.RGBA{190, 160, 60, 255})
		cv.jaguarSpots(158, 230, 35)
		cv.circle(158, 230, 35, 2.5, obsidian)
		// Jaguar head
		cv.filledCircle(170, 200, 22, color.RGBA{190, 160, 60, 255})
		cv.jaguarSpots(170, 200, 20)
		cv.circle(170, 200, 22, 2, obsidian)
		// Jaguar face
		cv.filledCircle(163, 197, 4, obsidian)
		cv.filledCircle(177, 197, 4, obsidian)
		cv.filledCircle(170, 204, 3, bloodRed)
		// Jaguar legs
		cv.filledRect(140, 258, 10, 30, color.RGBA{180, 150, 50, 255})
		cv.filledRect(168, 258, 10, 30, color.RGBA{180, 150, 50, 255})
		// Jungle background
		cv.jungleVine(200, 60, 180)
		cv.jungleVine(35, 70, 160)
		// Ground
		cv.line(30, 300, 210, 300, 2, obsidian)

	case 9: // Hermit → aged shaman in cave with torch
		// Cave opening
		cv.filledCircle(120, 200, 100, color.RGBA{55, 45, 35, 255})
		cv.filledCircle(120, 280, 110, obsidian)
		cv.arc(120, 200, 100, math.Pi, 2*math.Pi, 3, color.RGBA{70, 55, 40, 255})
		// Cave interior (dark)
		cv.filledRect(30, 200, 180, 160, obsidianDim)
		// Figure (aged shaman)
		cv.filledCircle(110, 170, 18, skin)
		cv.circle(110, 170, 18, 2, obsidian)
		// Grey/white hair
		cv.arc(110, 160, 20, math.Pi, 2*math.Pi, 3, stone)
		// Simple robes
		cv.filledRect(92, 192, 36, 70, s)
		cv.boldOutline(92, 192, 36, 70, 2)
		// Torch in hand
		cv.line(155, 175, 160, 120, 2.5, skinDim)
		cv.glow(160, 115, 18, sunGold)
		cv.glow(160, 110, 12, bloodRed)
		cv.filledCircle(160, 112, 6, sunGold)
		// Cave paintings on wall
		cv.glyphBlock(60, 120, 18, goldDim)
		cv.glyphBlock(170, 140, 14, bloodDim)
		// Staff
		cv.line(80, 180, 75, 300, 2.5, skinDim)

	case 10: // Wheel → Aztec sun stone / calendar wheel
		// Large sun stone
		cv.sunDisc(120, 200, 85)
		cv.glow(120, 200, 50, sunGold)
		// Extra decorative rings
		cv.circle(120, 200, 90, 3, sunGold)
		// Glyphs around outer ring (day signs)
		for i := 0; i < 20; i++ {
			a := float64(i) * math.Pi / 10
			gx := 120 + 78*math.Cos(a)
			gy := 200 + 78*math.Sin(a)
			cv.filledCircle(gx, gy, 4, obsidian)
		}
		// Corner serpent heads
		cv.serpentHead(45, 75, 20, jade, true)
		cv.serpentHead(195, 75, 20, jade, false)
		cv.serpentHead(45, 330, 20, jade, true)
		cv.serpentHead(195, 330, 20, jade, false)

	case 11: // Justice → figure with obsidian blade and scales
		// Figure
		cv.filledCircle(120, 130, 22, skin)
		cv.circle(120, 130, 22, 2, obsidian)
		// Judicial headdress
		cv.filledRect(100, 105, 40, 12, sunGold)
		cv.line(110, 105, 105, 85, 2, jade)
		cv.line(130, 105, 135, 85, 2, jade)
		// Robes
		cv.filledRect(85, 155, 70, 100, p)
		cv.boldOutline(85, 155, 70, 100, 2.5)
		// Obsidian blade in right hand
		cv.drawSword(170, 190, 45, stoneDim)
		// Scales in left hand
		cv.line(50, 180, 50, 200, 2, sunGold)
		cv.line(35, 200, 65, 200, 2, sunGold)
		cv.arc(40, 215, 12, 0, math.Pi, 2, sunGold)
		cv.arc(60, 215, 12, 0, math.Pi, 2, sunGold)
		cv.line(35, 200, 40, 215, 1.5, obsidian)
		cv.line(65, 200, 60, 215, 1.5, obsidian)
		// Platform
		cv.filledRect(60, 270, 120, 15, stone)
		cv.boldOutline(60, 270, 120, 15, 2)
		// Glyph beneath
		cv.glyphBlock(120, 320, 25, goldDim)

	case 12: // Hanged Man → figure suspended over cenote
		// Cenote (circular water pool)
		cv.filledCircle(120, 310, 60, turqDim)
		cv.circle(120, 310, 60, 3, stone)
		cv.circle(120, 310, 50, 1.5, turquoise)
		// Water ripples
		cv.arc(120, 310, 30, 0, 2*math.Pi, 1, turquoise)
		cv.arc(120, 310, 15, 0, 2*math.Pi, 1, lerpColor(turquoise, stone, 0.3))
		// Wooden frame over cenote
		cv.line(60, 80, 60, 200, 3, skinDim)
		cv.line(180, 80, 180, 200, 3, skinDim)
		cv.line(55, 100, 185, 100, 3, skinDim)
		// Rope
		cv.line(120, 100, 120, 150, 2, goldDim)
		// Inverted figure
		cv.line(120, 150, 110, 180, 3, jade)
		cv.line(120, 150, 130, 180, 3, jade)
		// Body
		cv.filledRect(103, 180, 34, 50, p)
		cv.boldOutline(103, 180, 34, 50, 2)
		// Arms hanging
		cv.line(103, 205, 85, 240, 2.5, skin)
		cv.line(137, 205, 155, 240, 2.5, skin)
		// Head at bottom
		cv.filledCircle(120, 250, 18, skin)
		cv.circle(120, 250, 18, 2, obsidian)
		// Serene expression
		cv.filledCircle(114, 248, 2, obsidian)
		cv.filledCircle(126, 248, 2, obsidian)
		cv.arc(120, 254, 5, 0.2, math.Pi-0.2, 1, obsidian)

	case 13: // Death → Mictlantecuhtli (death god), skull face
		// Skull head
		cv.filledCircle(120, 110, 35, stone)
		cv.filledCircle(108, 105, 10, obsidian) // eye sockets
		cv.filledCircle(132, 105, 10, obsidian)
		cv.filledCircle(120, 120, 5, obsidian) // nose
		cv.filledRect(105, 133, 30, 5, obsidian) // jaw
		// Teeth
		for x := 107.0; x < 135; x += 7 {
			cv.filledRect(x, 133, 5, 5, stone)
			cv.line(x, 133, x, 138, 1, obsidian)
		}
		cv.circle(120, 110, 35, 2.5, obsidian)
		// Headdress of bones and feathers
		cv.filledRect(85, 70, 70, 10, bloodRed)
		cv.line(95, 70, 90, 45, 3, obsidian) // bone
		cv.line(120, 70, 120, 40, 3, bloodRed) // feather
		cv.line(145, 70, 150, 45, 3, obsidian) // bone
		// Skeletal body
		cv.line(120, 148, 120, 250, 3, stone)
		// Ribs
		for i := 0; i < 4; i++ {
			y := 165 + float64(i)*18
			cv.arc(120, y, 18, 0, math.Pi, 1.5, stone)
		}
		// Bone legs
		cv.line(120, 250, 100, 320, 2, stone)
		cv.line(120, 250, 140, 320, 2, stone)
		// Dark cloak remnants
		cv.filledRect(105, 160, 30, 40, obsidian)
		// Blood offerings (red drops)
		drops := [][2]float64{{55, 180}, {185, 200}, {45, 280}, {195, 300}, {70, 330}, {175, 340}}
		for _, d := range drops {
			cv.filledCircle(d[0], d[1], 4, bloodRed)
		}

	case 14: // Temperance → figure pouring water between vessels, rain
		// Figure
		cv.filledCircle(120, 120, 22, skin)
		cv.circle(120, 120, 22, 2, obsidian)
		// Headband
		cv.filledRect(100, 112, 40, 5, turquoise)
		// Hair
		cv.filledCircle(120, 108, 24, obsidian)
		// Robes
		for y := 145.0; y < 300; y++ {
			t := (y - 145) / 155
			w := 22 + t*35
			cv.filledRect(120-w, y, w*2, 1, lerpColor(p, s, t*0.3))
		}
		cv.line(85, 300, 155, 300, 2.5, obsidian)
		cv.line(85, 145, 55, 300, 2.5, obsidian)
		cv.line(155, 145, 185, 300, 2.5, obsidian)
		// Left vessel (held high)
		cv.filledRect(55, 155, 22, 30, stone)
		cv.boldOutline(55, 155, 22, 30, 2)
		// Right vessel (held low)
		cv.filledRect(163, 210, 22, 30, stone)
		cv.boldOutline(163, 210, 22, 30, 2)
		// Pouring water stream
		cv.bezier(75, 185, 100, 190, 140, 200, 165, 215, 3, turquoise)
		cv.glow(120, 200, 12, turquoise)
		// Rain from above
		for x := 40.0; x < 200; x += 18 {
			cv.line(x, 60, x-3, 80, 1.5, turquoise)
		}
		// Pool at bottom
		cv.filledRect(45, 330, 150, 15, lerpColor(turquoise, sc.bg, 0.3))
		cv.line(45, 330, 195, 330, 2, obsidian)

	case 15: // Devil → Tezcatlipoca (smoking mirror)
		// Dark background intensified
		cv.filledRect(25, 55, 190, 305, lerpColor(sc.bg, obsidian, 0.3))
		// Figure (Tezcatlipoca)
		cv.filledCircle(120, 125, 28, obsidian)
		cv.circle(120, 125, 28, 2.5, sunGold)
		// Stripe across face (diagnostic feature)
		cv.filledRect(95, 118, 50, 8, sunGold)
		cv.filledRect(95, 128, 50, 8, obsidian)
		// Eyes (fierce)
		cv.filledCircle(108, 120, 6, sunGold)
		cv.filledCircle(132, 120, 6, sunGold)
		cv.filledCircle(108, 120, 3, bloodRed)
		cv.filledCircle(132, 120, 3, bloodRed)
		// Smoking mirror (obsidian disc at chest)
		cv.filledCircle(120, 195, 25, obsidianDim)
		cv.circle(120, 195, 25, 2, sunGold)
		// Smoke rising from mirror
		cv.glow(120, 175, 12, stoneDim)
		cv.glow(118, 160, 10, stone)
		cv.glow(122, 145, 8, stoneDim)
		// Body
		cv.filledRect(90, 155, 60, 90, lerpColor(obsidian, bloodRed, 0.2))
		cv.boldOutline(90, 155, 60, 90, 3)
		// Jaguar foot (one foot is replaced by mirror in myth)
		cv.filledCircle(145, 290, 12, obsidian)
		cv.circle(145, 290, 12, 2, sunGold)
		// Normal leg
		cv.filledRect(95, 245, 18, 55, obsidian)
		// Jaguar companion at side
		cv.filledCircle(55, 300, 18, color.RGBA{190, 160, 60, 255})
		cv.jaguarSpots(55, 300, 18)
		cv.circle(55, 300, 18, 2, obsidian)

	case 16: // Tower → pyramid crumbling, jungle reclaiming
		// Crumbling pyramid
		cv.pyramid(120, 340, 170, 200, stoneDim)
		// Cracks/damage
		cv.line(90, 200, 110, 250, 2, obsidian)
		cv.line(130, 180, 150, 230, 2, obsidian)
		cv.line(100, 260, 80, 310, 2, obsidian)
		// Falling blocks
		cv.filledRect(55, 150, 15, 12, stone)
		cv.filledRect(180, 200, 12, 10, stone)
		cv.filledRect(45, 280, 10, 8, stoneDim)
		// Jungle reclaiming — vines growing up pyramid
		cv.jungleVine(60, 120, 340)
		cv.jungleVine(180, 100, 320)
		cv.jungleVine(100, 160, 330)
		// Tree growing from top
		cv.line(120, 140, 120, 100, 3, skinDim)
		for x := 100.0; x <= 140; x += 10 {
			cv.filledCircle(x, 95, 12, jungle)
		}
		// Lightning
		cv.line(185, 60, 165, 90, 4, sunGold)
		cv.line(165, 90, 178, 100, 4, sunGold)
		cv.line(178, 100, 150, 140, 4, sunGold)
		cv.glow(170, 80, 15, sunGold)

	case 17: // Star → Venus star over jungle canopy
		// Starfield
		starPos := [][2]float64{{120, 75}, {60, 60}, {180, 65}, {45, 95}, {195, 90},
			{80, 78}, {160, 82}, {100, 62}}
		// Large Venus star
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			cv.line(120+12*math.Cos(a), 75+12*math.Sin(a),
				120+38*math.Cos(a), 75+38*math.Sin(a), 2.5, sunGold)
		}
		cv.glow(120, 75, 28, sunGold)
		cv.filledCircle(120, 75, 10, lerpColor(sunGold, stone, 0.3))
		// Smaller stars
		for _, sp := range starPos[1:] {
			cv.glow(sp[0], sp[1], 6, sunGold)
			cv.filledCircle(sp[0], sp[1], 3, sunGold)
		}
		// Jungle canopy below
		for x := 25.0; x < 215; x += 10 {
			r := 18 + 8*math.Sin(x*0.2)
			cv.filledCircle(x, 250, r, jungle)
		}
		for x := 30.0; x < 210; x += 12 {
			r := 14 + 5*math.Cos(x*0.15)
			cv.filledCircle(x, 260, r, jade)
		}
		// Tree trunks
		cv.line(60, 255, 60, 340, 3, skinDim)
		cv.line(120, 260, 120, 340, 3, skinDim)
		cv.line(180, 255, 180, 340, 3, skinDim)
		// Vines hanging
		cv.jungleVine(90, 240, 310)
		cv.jungleVine(150, 235, 300)

	case 18: // Moon → moon over jungle, jaguar below
		// Moon
		cv.filledCircle(120, 80, 42, turquoise)
		cv.filledCircle(138, 72, 40, sc.bg)
		cv.glow(105, 85, 35, turquoise)
		// Moon drops
		for x := 65.0; x < 175; x += 18 {
			cv.line(x, 130, x+2, 148, 1.5, turquoise)
		}
		// Jungle trees
		for i := 0; i < 3; i++ {
			x := 55 + float64(i)*60
			cv.line(x, 180, x, 300, 3, skinDim)
			for j := 0; j < 3; j++ {
				y := 180 + float64(j)*20
				w := 20 + float64(j)*5
				cv.filledCircle(x, y, w*0.6, jungleDim)
			}
		}
		// Jaguar below (prowling)
		cv.filledCircle(120, 320, 25, color.RGBA{190, 160, 60, 255})
		cv.filledCircle(145, 310, 16, color.RGBA{190, 160, 60, 255})
		cv.jaguarSpots(120, 320, 25)
		cv.circle(120, 320, 25, 2, obsidian)
		cv.circle(145, 310, 16, 2, obsidian)
		// Jaguar eyes gleaming
		cv.filledCircle(140, 307, 3, sunGold)
		cv.filledCircle(150, 307, 3, sunGold)
		// Tail
		cv.bezier(95, 320, 80, 315, 70, 305, 65, 310, 2, color.RGBA{190, 160, 60, 255})

	case 19: // Sun → Tonatiuh (sun god) face radiating
		// Giant sun face
		cv.filledCircle(120, 180, 65, sunGold)
		cv.glow(120, 180, 55, sunGold)
		// Rays
		for i := 0; i < 20; i++ {
			a := float64(i) * math.Pi / 10
			r1 := 68.0
			r2 := 110.0
			col := sunGold
			if i%2 == 0 {
				col = bloodRed
			}
			cv.line(120+r1*math.Cos(a), 180+r1*math.Sin(a),
				120+r2*math.Cos(a), 180+r2*math.Sin(a), 3, col)
		}
		// Tonatiuh face details
		cv.filledCircle(105, 170, 12, obsidian) // left eye
		cv.filledCircle(135, 170, 12, obsidian) // right eye
		cv.filledCircle(105, 170, 6, bloodRed)
		cv.filledCircle(135, 170, 6, bloodRed)
		// Tongue (protruding, sacrificial knife — iconic feature)
		cv.filledRect(113, 200, 14, 25, bloodRed)
		cv.line(113, 200, 127, 200, 2, obsidian)
		cv.line(120, 200, 120, 225, 1.5, obsidian) // blade center
		// Nose
		cv.filledRect(114, 180, 12, 15, lerpColor(sunGold, obsidian, 0.2))
		// Cheek decorations
		cv.filledCircle(85, 190, 6, bloodRed)
		cv.filledCircle(155, 190, 6, bloodRed)
		// Inner ring
		cv.circle(120, 180, 50, 3, obsidian)
		// Four direction markers
		for i := 0; i < 4; i++ {
			a := float64(i)*math.Pi/2 - math.Pi/4
			cv.filledRect(120+45*math.Cos(a)-5, 180+45*math.Sin(a)-5, 10, 10, bloodRed)
		}

	case 20: // Judgement → Quetzalcoatl descending, feathered serpent
		// Sky background with glow
		cv.glow(120, 100, 50, sunGold)
		// Feathered serpent body (descending S-curve)
		cv.bezier(180, 60, 60, 120, 180, 200, 60, 280, 6, jade)
		// Scales/feathers along body
		for i := 0; i < 30; i++ {
			t := float64(i) / 30
			px, py := bezierPoint(t, 180, 60, 60, 120, 180, 200, 60, 280)
			if i%3 == 0 {
				cv.filledCircle(px, py, 4, sunGold)
			} else {
				cv.filledCircle(px, py, 3, turquoise)
			}
		}
		// Serpent head at bottom
		cv.serpentHead(60, 280, 35, jade, false)
		// Feathered crest on serpent head
		for i := 0; i < 5; i++ {
			a := math.Pi/2 + float64(i)*0.3 - 0.6
			fx := 60 + 30*math.Cos(a)
			fy := 270 - 25*math.Sin(a)
			cols := []color.RGBA{jade, turquoise, bloodRed, turquoise, jade}
			cv.line(60, 270, fx, fy, 2, cols[i])
		}
		// People below looking up
		for i := 0; i < 3; i++ {
			x := 100 + float64(i)*30
			cv.filledCircle(x, 340, 10, skin)
			cv.circle(x, 340, 10, 1.5, obsidian)
			cv.filledRect(x-6, 352, 12, 15, p)
		}
		// Wind/breath spirals
		cv.arc(80, 300, 12, 0, math.Pi*1.5, 1.5, sunGold)

	case 21: // World → dancer within great calendar circle
		// Large calendar circle
		cv.circle(120, 200, 85, 4, sunGold)
		cv.circle(120, 200, 80, 2, goldDim)
		// Day sign dots around rim
		for i := 0; i < 20; i++ {
			a := float64(i) * math.Pi / 10
			gx := 120 + 82*math.Cos(a)
			gy := 200 + 82*math.Sin(a)
			cv.filledCircle(gx, gy, 3, sunGold)
		}
		// Inner ring
		cv.circle(120, 200, 60, 2, sunGold)
		// Dancing figure inside
		cv.filledCircle(120, 175, 18, skin)
		cv.circle(120, 175, 18, 2, obsidian)
		// Headdress
		cv.line(120, 157, 108, 140, 2, jade)
		cv.line(120, 157, 120, 135, 2, bloodRed)
		cv.line(120, 157, 132, 140, 2, turquoise)
		// Body
		cv.filledRect(108, 195, 24, 40, lerpColor(p, s, 0.3))
		cv.boldOutline(108, 195, 24, 40, 2)
		// Arms extended
		cv.line(108, 200, 78, 180, 2.5, skin)
		cv.line(132, 200, 162, 180, 2.5, skin)
		// Legs in dance pose
		cv.line(113, 235, 100, 265, 2, skin)
		cv.line(127, 235, 140, 265, 2, skin)
		// Ribbons / cloth
		cv.bezier(78, 180, 55, 170, 45, 185, 50, 200, 2, jade)
		cv.bezier(162, 180, 185, 170, 195, 185, 190, 200, 2, bloodRed)
		// Four direction symbols in corners
		cv.glyphBlock(40, 70, 18, bloodRed)   // East
		cv.glyphBlock(200, 70, 18, jade)       // West
		cv.glyphBlock(40, 335, 18, sunGold)    // South
		cv.glyphBlock(200, 335, 18, turquoise) // North
	}

	return cv
}

// ========== main ==========

func main() {
	base := "decks/aztec"

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
