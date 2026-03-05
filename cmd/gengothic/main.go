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

// ========== Gothic / Stained Glass elements ==========

// leadLine draws a thick dark line simulating lead came between glass panes
func (c *HiResCanvas) leadLine(x1, y1, x2, y2, thickness float64) {
	c.line(x1, y1, x2, y2, thickness, lead)
}

// glassPane fills a rectangular region with a subtle light-through-glass gradient
func (c *HiResCanvas) glassPane(x, y, w, h float64, col color.RGBA) {
	// Light comes from upper-left, creating a subtle brightness gradient
	for py := int(y); py < int(y+h); py++ {
		for px := int(x); px < int(x+w); px++ {
			ty := (float64(py) - y) / h
			tx := (float64(px) - x) / w
			// Simulate light passing through glass — brighter center, dimmer edges
			bright := 0.7 + 0.3*math.Sin(ty*math.Pi)*math.Sin(tx*math.Pi)
			gc := color.RGBA{
				clampU8(float64(col.R) * bright),
				clampU8(float64(col.G) * bright),
				clampU8(float64(col.B) * bright),
				255,
			}
			c.set(px, py, gc)
		}
	}
}

// roseWindow draws a circular mandala with radial jewel-tone segments
func (c *HiResCanvas) roseWindow(cx, cy, radius float64) {
	petals := 12
	colors := []color.RGBA{ruby, sapphire, emerald, amberGold, amethyst, ruby,
		sapphire, emerald, amberGold, amethyst, ruby, sapphire}

	// Outer ring
	c.circle(cx, cy, radius, 4, lead)

	// Fill wedge segments
	for i := 0; i < petals; i++ {
		a1 := float64(i) * 2 * math.Pi / float64(petals)
		a2 := float64(i+1) * 2 * math.Pi / float64(petals)
		aMid := (a1 + a2) / 2
		col := colors[i%len(colors)]
		// Fill the wedge with glass
		for r := 8.0; r < radius-3; r += 1 {
			for a := a1 + 0.02; a < a2-0.02; a += 0.01 {
				px := cx + r*math.Cos(a)
				py := cy + r*math.Sin(a)
				// Brightness varies with distance from center
				bright := 0.6 + 0.4*math.Sin((r/radius)*math.Pi)
				gc := color.RGBA{
					clampU8(float64(col.R) * bright),
					clampU8(float64(col.G) * bright),
					clampU8(float64(col.B) * bright),
					255,
				}
				c.blend(int(px), int(py), gc, 0.9)
			}
		}
		// Lead line spokes
		c.line(cx+8*math.Cos(a1), cy+8*math.Sin(a1),
			cx+(radius-2)*math.Cos(a1), cy+(radius-2)*math.Sin(a1), 2, lead)
		// Small circle at mid-radius on each spoke midpoint
		mr := radius * 0.6
		c.filledCircle(cx+mr*math.Cos(aMid), cy+mr*math.Sin(aMid), 4, ivory)
		c.circle(cx+mr*math.Cos(aMid), cy+mr*math.Sin(aMid), 4, 1.5, lead)
	}

	// Inner circle
	c.circle(cx, cy, radius*0.35, 3, lead)
	c.filledCircle(cx, cy, radius*0.33, amberGold)
	c.glow(cx, cy, radius*0.25, ivory)
	// Center cross
	c.leadLine(cx-radius*0.3, cy, cx+radius*0.3, cy, 2)
	c.leadLine(cx, cy-radius*0.3, cx, cy+radius*0.3, 2)
}

// gothicArch draws a pointed arch frame
func (c *HiResCanvas) gothicArch(x1, y1, x2, y2, thickness float64, col color.RGBA) {
	// x1,y1 = bottom-left; x2,y2 = top-center point; width = 2*(x2-x1)
	w := x2 - x1
	right := x2 + w
	bottom := y1

	// Left pillar
	c.line(x1, bottom, x1, y2+w*0.6, thickness, col)
	// Right pillar
	c.line(right, bottom, right, y2+w*0.6, thickness, col)
	// Pointed arch top — two arcs meeting at the peak
	// Left arc: from left pillar top, curving to peak
	steps := 100
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		// Quadratic bezier: left top → peak
		ax := x1 + (x2-x1)*t
		// Arch shape: parabolic
		ay := (y2+w*0.6)*(1-t) + y2*t - w*0.3*math.Sin(t*math.Pi)
		c.filledCircle(ax, ay, thickness/2, col)
	}
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		ax := x2 + w*t
		ay := y2*(1-t) + (y2+w*0.6)*t - w*0.3*math.Sin(t*math.Pi)
		c.filledCircle(ax, ay, thickness/2, col)
	}
}

// gothicArchFrame draws a full pointed arch frame around the card
func (c *HiResCanvas) gothicArchFrame(col color.RGBA) {
	m := 12.0
	w := float64(hiW)
	h := float64(hiH)
	th := 3.0

	// Side pillars
	c.line(m, h*0.25, m, h-m, th, col)
	c.line(w-m, h*0.25, w-m, h-m, th, col)
	// Bottom
	c.line(m, h-m, w-m, h-m, th, col)
	// Pointed arch top
	peak := h * 0.06
	cx := w / 2
	steps := 150
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		// Left half
		if t <= 0.5 {
			lt := t * 2
			px := m + (cx-m)*lt
			py := h*0.25 - (h*0.25-peak)*math.Pow(lt, 1.3)
			c.filledCircle(px, py, th/2, col)
		} else {
			rt := (t - 0.5) * 2
			px := cx + (w-m-cx)*rt
			py := peak + (h*0.25-peak)*math.Pow(rt, 1.3)
			c.filledCircle(px, py, th/2, col)
		}
	}
}

// trefoil draws a three-lobed ornament
func (c *HiResCanvas) trefoil(cx, cy, size float64, col color.RGBA) {
	for i := 0; i < 3; i++ {
		a := float64(i)*2*math.Pi/3 - math.Pi/2
		lx := cx + size*0.4*math.Cos(a)
		ly := cy + size*0.4*math.Sin(a)
		c.filledCircle(lx, ly, size*0.35, col)
		c.circle(lx, ly, size*0.35, 2, lead)
	}
	c.filledCircle(cx, cy, size*0.2, col)
}

// quatrefoil draws a four-lobed ornament
func (c *HiResCanvas) quatrefoil(cx, cy, size float64, col color.RGBA) {
	for i := 0; i < 4; i++ {
		a := float64(i)*math.Pi/2 + math.Pi/4
		lx := cx + size*0.35*math.Cos(a)
		ly := cy + size*0.35*math.Sin(a)
		c.filledCircle(lx, ly, size*0.3, col)
		c.circle(lx, ly, size*0.3, 1.5, lead)
	}
	c.filledCircle(cx, cy, size*0.15, col)
}

// gothicBorder draws a pointed arch + trefoil repeating border
func (c *HiResCanvas) gothicBorder(margin, thickness float64, col color.RGBA) {
	w := float64(hiW)
	h := float64(hiH)
	m := margin

	// Outer frame
	c.line(m, m, w-m, m, thickness, col)
	c.line(w-m, m, w-m, h-m, thickness, col)
	c.line(w-m, h-m, m, h-m, thickness, col)
	c.line(m, h-m, m, m, thickness, col)

	// Inner frame
	m2 := margin + 7
	c.line(m2, m2, w-m2, m2, thickness*0.6, col)
	c.line(w-m2, m2, w-m2, h-m2, thickness*0.6, col)
	c.line(w-m2, h-m2, m2, h-m2, thickness*0.6, col)
	c.line(m2, h-m2, m2, m2, thickness*0.6, col)

	// Small pointed arches along top border
	archW := (w - 2*m2) / 6
	for i := 0; i < 6; i++ {
		ax := m2 + float64(i)*archW
		// Mini pointed arch
		c.line(ax, m2, ax+archW/2, m2-6, thickness*0.4, col)
		c.line(ax+archW/2, m2-6, ax+archW, m2, thickness*0.4, col)
	}

	// Small pointed arches along bottom border
	for i := 0; i < 6; i++ {
		ax := m2 + float64(i)*archW
		c.line(ax, h-m2, ax+archW/2, h-m2+6, thickness*0.4, col)
		c.line(ax+archW/2, h-m2+6, ax+archW, h-m2, thickness*0.4, col)
	}

	// Trefoil corners
	cornerSize := 10.0
	c.trefoil(m+cornerSize, m+cornerSize, cornerSize, col)
	c.trefoil(w-m-cornerSize, m+cornerSize, cornerSize, col)
	c.trefoil(m+cornerSize, h-m-cornerSize, cornerSize, col)
	c.trefoil(w-m-cornerSize, h-m-cornerSize, cornerSize, col)
}

// leadGrid draws a background lattice of lead lines
func (c *HiResCanvas) leadGrid(spacing float64) {
	// Vertical lead lines
	for x := spacing; x < float64(hiW); x += spacing {
		c.line(x, 0, x, float64(hiH), 1, leadDim)
	}
	// Horizontal lead lines
	for y := spacing; y < float64(hiH); y += spacing {
		c.line(0, y, float64(hiW), y, 1, leadDim)
	}
	// Diamond lattice overlay
	for x := 0.0; x < float64(hiW); x += spacing * 2 {
		for y := 0.0; y < float64(hiH); y += spacing * 2 {
			c.line(x, y+spacing, x+spacing, y, 0.5, leadDim)
			c.line(x+spacing, y, x+spacing*2, y+spacing, 0.5, leadDim)
			c.line(x, y+spacing, x+spacing, y+spacing*2, 0.5, leadDim)
			c.line(x+spacing, y+spacing*2, x+spacing*2, y+spacing, 0.5, leadDim)
		}
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

// ========== color palette ==========

var (
	lead     = color.RGBA{20, 20, 25, 255}    // near-black lead came
	leadDim  = color.RGBA{30, 28, 35, 255}    // subtle lead for grid
	ruby     = color.RGBA{180, 20, 40, 255}   // ruby red glass
	rubyDim  = color.RGBA{100, 12, 25, 255}
	sapphire = color.RGBA{30, 50, 160, 255}   // sapphire blue glass
	saphDim  = color.RGBA{18, 30, 90, 255}
	emerald  = color.RGBA{20, 120, 60, 255}   // emerald green glass
	emerDim  = color.RGBA{12, 70, 35, 255}
	amberGold = color.RGBA{220, 180, 40, 255} // amber / gold highlights
	amberDim = color.RGBA{130, 105, 25, 255}
	amethyst = color.RGBA{100, 30, 140, 255}  // amethyst purple glass
	amethDim = color.RGBA{60, 18, 85, 255}
	ivory    = color.RGBA{240, 235, 220, 255} // ivory highlight
	stoneDk  = color.RGBA{35, 30, 40, 255}    // dark stone background
	stoneBot = color.RGBA{25, 20, 30, 255}    // darker stone at bottom
	skin     = color.RGBA{210, 175, 140, 255}
)

// suit colors — mapped to glass tones
var (
	cupsRuby     = ruby
	cupsRubyDim  = rubyDim
	wandsAmber   = amberGold
	wandsAmberD  = amberDim
	swordsBlue   = sapphire
	swordsBluD   = saphDim
	pentEmerald  = emerald
	pentEmeraldD = emerDim
)

type suitColors struct {
	primary, dim color.RGBA
}

var suitPalette = []suitColors{
	{cupsRuby, cupsRubyDim},         // cups (index 0)
	{wandsAmber, wandsAmberD},       // wands (index 1)
	{swordsBlue, swordsBluD},        // swords (index 2)
	{pentEmerald, pentEmeraldD},     // pentacles (index 3)
}

var suitFileNames = []string{"cups", "wands", "swords", "pentacles"}

// ========== gothic background ==========

func gothicBG() *HiResCanvas {
	cv := newCanvas()
	cv.fillGradient(stoneDk, stoneBot)
	return cv
}

// ========== suit pip shapes (hi-res: ~30px) ==========

func (c *HiResCanvas) drawCup(cx, cy, size float64, col color.RGBA) {
	// Chalice / goblet in stained glass style
	c.arc(cx, cy-size*0.1, size*0.4, 0, math.Pi, 2.5, col)
	c.filledCircle(cx, cy-size*0.1, size*0.3, col)
	c.line(cx, cy+size*0.2, cx, cy+size*0.5, 2.5, col)
	c.line(cx-size*0.25, cy+size*0.5, cx+size*0.25, cy+size*0.5, 2.5, col)
	// Lead outline
	c.arc(cx, cy-size*0.1, size*0.42, 0, math.Pi, 1.5, lead)
	c.circle(cx, cy-size*0.1, size*0.32, 1, lead)
}

func (c *HiResCanvas) drawWand(cx, cy, size float64, col color.RGBA) {
	c.line(cx, cy-size*0.5, cx, cy+size*0.5, 2.5, col)
	// Flame top
	c.filledCircle(cx, cy-size*0.55, size*0.18, amberGold)
	c.glow(cx, cy-size*0.55, size*0.25, amberGold)
	// Cross bars (gothic style)
	c.line(cx-size*0.15, cy-size*0.2, cx+size*0.15, cy-size*0.2, 2, col)
	c.line(cx-size*0.1, cy+size*0.15, cx+size*0.1, cy+size*0.15, 1.5, col)
}

func (c *HiResCanvas) drawSword(cx, cy, size float64, col color.RGBA) {
	c.line(cx, cy-size*0.55, cx, cy+size*0.4, 2.5, col)
	// Cross-guard (gothic cross shape)
	c.line(cx-size*0.3, cy-size*0.1, cx+size*0.3, cy-size*0.1, 2.5, col)
	c.filledCircle(cx, cy+size*0.45, size*0.08, col)
	// Blade glow
	c.glow(cx, cy-size*0.55, size*0.12, ivory)
}

func (c *HiResCanvas) drawPentacle(cx, cy, size float64, col color.RGBA) {
	c.circle(cx, cy, size*0.4, 2.5, col)
	for i := 0; i < 5; i++ {
		a := float64(i)*2*math.Pi/5 - math.Pi/2
		a2 := float64(i+2)*2*math.Pi/5 - math.Pi/2
		x1 := cx + size*0.38*math.Cos(a)
		y1 := cy + size*0.38*math.Sin(a)
		x2 := cx + size*0.38*math.Cos(a2)
		y2 := cy + size*0.38*math.Sin(a2)
		c.line(x1, y1, x2, y2, 1.5, col)
	}
	c.circle(cx, cy, size*0.42, 1.5, lead)
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
	cv.fillGradient(stoneDk, stoneBot)

	// Lead line grid background
	cv.leadGrid(30)

	// Gothic border
	cv.gothicBorder(8, 3.0, lead)

	// Central rose window
	cv.roseWindow(120, 200, 65)

	// Gothic arch frame around rose window
	cv.gothicArchFrame(lead)

	// Small quatrefoils in corners
	cv.quatrefoil(35, 35, 14, amberGold)
	cv.quatrefoil(205, 35, 14, amberGold)
	cv.quatrefoil(35, 365, 14, amberGold)
	cv.quatrefoil(205, 365, 14, amberGold)

	// Trefoils mid-border
	cv.trefoil(120, 22, 10, amberDim)
	cv.trefoil(120, 378, 10, amberDim)
	cv.trefoil(22, 200, 10, amberDim)
	cv.trefoil(218, 200, 10, amberDim)

	return cv
}

func genPip(si, num int) *HiResCanvas {
	cv := gothicBG()
	sc := suitPalette[si]

	// Subtle lattice background
	cv.leadGrid(40)

	// Gothic border
	cv.gothicBorder(6, 2.0, lead)

	// Corner numbers
	cv.drawNum(15, 15, num, sc.primary, 5)
	cv.drawNum(185, 350, num, sc.primary, 5)

	// Small gothic arch at top
	cv.line(60, 45, 120, 25, 1.5, sc.dim)
	cv.line(120, 25, 180, 45, 1.5, sc.dim)

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
	cv := gothicBG()
	sc := suitPalette[si]

	// Subtle lattice
	cv.leadGrid(50)

	// Gothic arch frame
	cv.gothicArchFrame(lead)
	cv.gothicBorder(5, 2.0, lead)

	p := sc.primary

	switch rank {
	case 11: // Page — young figure in stained glass panel
		// Glass panel background
		cv.glassPane(30, 50, 180, 290, suitGlassColor(si))
		// Lead frame around panel
		cv.line(30, 50, 210, 50, 2, lead)
		cv.line(30, 340, 210, 340, 2, lead)
		cv.line(30, 50, 30, 340, 2, lead)
		cv.line(210, 50, 210, 340, 2, lead)
		// Head (silhouette style)
		cv.filledCircle(120, 110, 25, skin)
		cv.glow(120, 110, 12, ivory)
		// Body
		cv.filledRect(100, 140, 40, 100, p)
		// Cloak edges
		cv.line(100, 140, 88, 240, 2.5, sc.dim)
		cv.line(140, 140, 152, 240, 2.5, sc.dim)
		// Legs
		cv.filledRect(105, 240, 14, 55, sc.dim)
		cv.filledRect(125, 240, 14, 55, sc.dim)
		// Suit symbol in hand
		cv.drawSuitPip(si, 120, 190, 28)
		// Lead line details
		cv.leadLine(30, 195, 210, 195, 1.5)
		// Trefoil accents
		cv.trefoil(50, 70, 10, amberDim)
		cv.trefoil(190, 70, 10, amberDim)

	case 12: // Knight — armored figure
		cv.glassPane(30, 50, 180, 290, suitGlassColor(si))
		cv.line(30, 50, 210, 50, 2, lead)
		cv.line(30, 340, 210, 340, 2, lead)
		cv.line(30, 50, 30, 340, 2, lead)
		cv.line(210, 50, 210, 340, 2, lead)
		// Head with helm
		cv.filledCircle(105, 100, 25, skin)
		cv.glow(105, 100, 12, ivory)
		cv.arc(105, 90, 26, math.Pi, 2*math.Pi, 3, p)
		// Body/armor
		cv.filledRect(85, 130, 40, 100, p)
		// Extended arm with weapon
		cv.line(125, 160, 195, 140, 3, p)
		cv.line(195, 110, 195, 170, 3, sc.primary)
		cv.glow(195, 120, 8, ivory)
		// Legs
		cv.filledRect(90, 230, 14, 65, sc.dim)
		cv.filledRect(110, 230, 14, 65, sc.dim)
		// Suit symbol
		cv.drawSuitPip(si, 170, 260, 28)
		// Shield
		cv.arc(65, 210, 25, 0.5*math.Pi, 1.5*math.Pi, 2, p)

	case 13: // Queen — crowned figure with flowing robes
		cv.glassPane(30, 50, 180, 290, suitGlassColor(si))
		cv.line(30, 50, 210, 50, 2, lead)
		cv.line(30, 340, 210, 340, 2, lead)
		cv.line(30, 50, 30, 340, 2, lead)
		cv.line(210, 50, 210, 340, 2, lead)
		// Crown
		for i := 0; i < 5; i++ {
			x := 100 + float64(i)*10
			cv.line(x, 55, x, 68, 2, amberGold)
			cv.filledCircle(x, 52, 4, amberGold)
		}
		cv.filledRect(98, 68, 44, 6, amberGold)
		// Head
		cv.filledCircle(120, 88, 22, skin)
		cv.glow(120, 88, 12, ivory)
		// Flowing robes
		for y := 115.0; y < 310; y += 1 {
			t := (y - 115) / 195
			w := 28 + t*55
			col := lerpColor(p, sc.dim, t*0.5)
			cv.filledRect(120-w, y, w*2, 1, col)
		}
		// Robe trim
		cv.line(65, 310, 175, 310, 2, amberGold)
		// Suit symbol
		cv.drawSuitPip(si, 120, 210, 30)
		// Quatrefoil accents
		cv.quatrefoil(50, 180, 8, amberDim)
		cv.quatrefoil(190, 180, 8, amberDim)

	case 14: // King — broad crowned figure with scepter
		cv.glassPane(30, 50, 180, 290, suitGlassColor(si))
		cv.line(30, 50, 210, 50, 2, lead)
		cv.line(30, 340, 210, 340, 2, lead)
		cv.line(30, 50, 30, 340, 2, lead)
		cv.line(210, 50, 210, 340, 2, lead)
		// Crown (larger)
		for i := 0; i < 7; i++ {
			x := 92 + float64(i)*8
			h := 12.0
			if i%2 == 0 {
				h = 18
			}
			cv.line(x, 55-h, x, 58, 2.5, amberGold)
			cv.filledCircle(x, 55-h-3, 4, amberGold)
		}
		cv.filledRect(88, 58, 64, 8, amberGold)
		// Head
		cv.filledCircle(120, 80, 24, skin)
		cv.glow(120, 80, 14, ivory)
		// Beard
		cv.filledCircle(120, 98, 15, color.RGBA{100, 60, 30, 255})
		// Broad robes
		cv.filledRect(80, 115, 80, 120, p)
		cv.filledRect(75, 115, 90, 8, amberGold)
		// Scepter
		cv.line(175, 100, 175, 280, 3, amberGold)
		cv.trefoil(175, 95, 10, amberGold)
		// Legs
		cv.filledRect(90, 235, 20, 60, sc.dim)
		cv.filledRect(130, 235, 20, 60, sc.dim)
		// Suit symbol on chest
		cv.drawSuitPip(si, 120, 170, 30)
		// Throne hints — gothic arches
		cv.line(60, 110, 60, 280, 2.5, lead)
		cv.line(180, 110, 180, 280, 2.5, lead)
		// Pointed arch above throne
		cv.line(60, 110, 120, 80, 2, lead)
		cv.line(120, 80, 180, 110, 2, lead)
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

// suitGlassColor returns a dim translucent glass color for the suit's background panel
func suitGlassColor(si int) color.RGBA {
	switch si {
	case 0:
		return color.RGBA{60, 15, 22, 255} // dim ruby
	case 1:
		return color.RGBA{55, 45, 15, 255} // dim amber
	case 2:
		return color.RGBA{15, 22, 55, 255} // dim sapphire
	case 3:
		return color.RGBA{12, 40, 25, 255} // dim emerald
	}
	return stoneDk
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
	glass              color.RGBA // stained glass background tint
}

var majorSchemes = []majorScheme{
	{ivory, amberGold, color.RGBA{50, 45, 35, 255}},              // 0  Fool
	{amberGold, amethyst, color.RGBA{55, 45, 15, 255}},            // 1  Magician
	{sapphire, ivory, color.RGBA{15, 22, 55, 255}},               // 2  High Priestess
	{emerald, amberGold, color.RGBA{12, 40, 25, 255}},            // 3  Empress
	{ruby, amberGold, color.RGBA{60, 15, 22, 255}},               // 4  Emperor
	{amberGold, ivory, color.RGBA{55, 45, 15, 255}},              // 5  Hierophant
	{ruby, ivory, color.RGBA{55, 15, 25, 255}},                   // 6  Lovers
	{sapphire, amberGold, color.RGBA{15, 22, 55, 255}},           // 7  Chariot
	{amberGold, ruby, color.RGBA{55, 45, 15, 255}},               // 8  Strength
	{ivory, amberDim, color.RGBA{40, 38, 32, 255}},               // 9  Hermit
	{amberGold, amethyst, color.RGBA{50, 30, 55, 255}},           // 10 Wheel
	{amberGold, sapphire, color.RGBA{30, 35, 55, 255}},           // 11 Justice
	{emerald, amethyst, color.RGBA{20, 35, 40, 255}},             // 12 Hanged Man
	{ivory, ruby, color.RGBA{45, 20, 25, 255}},                   // 13 Death
	{sapphire, amberGold, color.RGBA{18, 25, 55, 255}},           // 14 Temperance
	{ruby, amberGold, color.RGBA{55, 18, 18, 255}},               // 15 Devil
	{amberGold, ruby, color.RGBA{55, 30, 15, 255}},               // 16 Tower
	{ivory, sapphire, color.RGBA{25, 30, 50, 255}},               // 17 Star
	{amethyst, sapphire, color.RGBA{35, 22, 55, 255}},            // 18 Moon
	{amberGold, ruby, color.RGBA{55, 45, 15, 255}},               // 19 Sun
	{ivory, amberGold, color.RGBA{45, 40, 30, 255}},              // 20 Judgement
	{emerald, amberGold, color.RGBA{20, 40, 30, 255}},            // 21 World
}

func genMajor(num int) *HiResCanvas {
	cv := gothicBG()
	sc := majorSchemes[num]
	p, s := sc.primary, sc.secondary

	// Subtle lattice
	cv.leadGrid(50)

	// Number in corner
	cv.drawNum(15, 15, num, amberGold, 5)

	// Gothic arch frame
	cv.gothicArchFrame(lead)

	// Glass panel background for the scene
	cv.glassPane(25, 40, 190, 320, sc.glass)
	// Lead frame around panel
	cv.line(25, 40, 215, 40, 2, lead)
	cv.line(25, 360, 215, 360, 2, lead)
	cv.line(25, 40, 25, 360, 2, lead)
	cv.line(215, 40, 215, 360, 2, lead)

	switch num {
	case 0: // Fool — figure at cliff edge, stained glass sky
		// Cliff/ground in glass panels
		cv.glassPane(25, 270, 150, 90, color.RGBA{40, 30, 22, 255})
		cv.leadLine(25, 270, 175, 270, 2)
		// Figure
		cv.filledCircle(100, 180, 22, skin)
		cv.glow(100, 180, 12, ivory)
		cv.filledRect(85, 205, 30, 70, p)
		// Cloak
		cv.bezier(85, 210, 60, 240, 50, 280, 70, 300, 2.5, s)
		cv.bezier(115, 210, 130, 240, 140, 260, 130, 280, 2.5, s)
		// Legs
		cv.filledRect(90, 275, 12, 40, color.RGBA{p.R / 2, p.G / 2, p.B / 2, 255})
		cv.filledRect(105, 275, 12, 40, color.RGBA{p.R / 2, p.G / 2, p.B / 2, 255})
		// Rose in hand
		cv.trefoil(130, 220, 8, ruby)
		// Star above (rose window miniature)
		cv.circle(180, 65, 18, 2, amberGold)
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			cv.line(180+8*math.Cos(a), 65+8*math.Sin(a),
				180+16*math.Cos(a), 65+16*math.Sin(a), 1.5, amberGold)
		}
		cv.filledCircle(180, 65, 5, ivory)
		// Void below cliff
		cv.glow(200, 330, 20, amethyst)

	case 1: // Magician — table with elements, arch above
		// Pointed arch above
		cv.line(40, 120, 120, 55, 3, amberGold)
		cv.line(120, 55, 200, 120, 3, amberGold)
		// Infinity symbol
		cv.arc(105, 70, 15, 0, 2*math.Pi, 2.5, p)
		cv.arc(135, 70, 15, 0, 2*math.Pi, 2.5, p)
		cv.glow(120, 70, 10, ivory)
		// Figure
		cv.filledCircle(120, 130, 22, skin)
		cv.glow(120, 130, 12, ivory)
		cv.filledRect(105, 155, 30, 80, s)
		// Arms
		cv.line(105, 175, 55, 160, 3, skin)
		cv.line(135, 175, 185, 160, 3, skin)
		cv.glow(55, 160, 8, p)
		cv.glow(185, 160, 8, s)
		// Table
		cv.filledRect(40, 260, 160, 8, amberDim)
		cv.leadLine(40, 260, 200, 260, 2)
		// Four elements
		cv.drawSuitPip(0, 60, 295, 25)
		cv.drawSuitPip(1, 100, 295, 25)
		cv.drawSuitPip(2, 140, 295, 25)
		cv.drawSuitPip(3, 180, 295, 25)
		// Lead dividers between elements
		cv.leadLine(80, 270, 80, 320, 1.5)
		cv.leadLine(120, 270, 120, 320, 1.5)
		cv.leadLine(160, 270, 160, 320, 1.5)

	case 2: // High Priestess — crescent moon, pillars, veil
		// Moon crescent
		cv.filledCircle(120, 60, 25, sapphire)
		cv.filledCircle(132, 55, 25, sc.glass)
		cv.glow(110, 60, 20, sapphire)
		// Two pillars
		cv.glassPane(30, 90, 22, 260, color.RGBA{25, 25, 35, 255})
		cv.glassPane(188, 90, 22, 260, color.RGBA{55, 50, 42, 255})
		cv.leadLine(30, 90, 52, 90, 2)
		cv.leadLine(30, 350, 52, 350, 2)
		cv.leadLine(188, 90, 210, 90, 2)
		cv.leadLine(188, 350, 210, 350, 2)
		// Pillar capitals
		cv.filledRect(28, 85, 26, 8, amberGold)
		cv.filledRect(186, 85, 26, 8, amberGold)
		// Veil (translucent glass)
		for y := 100; y < 340; y++ {
			a := 0.15 + 0.08*math.Sin(float64(y)*0.05)
			for x := 55; x < 185; x++ {
				cv.blend(x, y, sapphire, a)
			}
		}
		// Figure
		cv.filledCircle(120, 145, 20, skin)
		cv.glow(120, 145, 10, ivory)
		// Robes
		for y := 170.0; y < 340; y++ {
			t := (y - 170) / 170
			w := 20 + t*40
			cv.filledRect(120-w, y, w*2, 1, lerpColor(sapphire, saphDim, t))
		}
		// Scroll
		cv.filledRect(105, 300, 30, 20, ivory)
		cv.leadLine(105, 300, 135, 300, 1.5)
		cv.leadLine(105, 320, 135, 320, 1.5)

	case 3: // Empress — crown, garden, abundance
		// Crown with gems
		for i := 0; i < 7; i++ {
			x := 92 + float64(i)*8
			cv.line(x, 50, x, 62, 2, amberGold)
			cv.filledCircle(x, 47, 4, emerald)
		}
		cv.filledRect(88, 62, 64, 6, amberGold)
		// Head
		cv.filledCircle(120, 78, 22, skin)
		cv.glow(120, 78, 12, ivory)
		// Flowing robes
		for y := 105.0; y < 300; y++ {
			t := (y - 105) / 195
			w := 25 + t*75
			col := lerpColor(p, s, t*0.3)
			cv.filledRect(120-w, y, w*2, 1, col)
		}
		// Garden below — glass panels of flowers
		for x := 30.0; x < 210; x += 30 {
			cv.glassPane(x, 310, 28, 45, color.RGBA{15, 45, 25, 255})
			cv.leadLine(x, 310, x, 355, 1)
			cv.trefoil(x+14, 330, 8, ruby)
		}
		cv.leadLine(30, 310, 210, 310, 2)
		// Stars
		cv.glow(50, 55, 8, amberGold)
		cv.glow(190, 55, 8, amberGold)

	case 4: // Emperor — throne, angular, commanding
		// Crown
		for i := 0; i < 5; i++ {
			x := 100 + float64(i)*10
			h := 20.0
			if i%2 == 1 {
				h = 15
			}
			cv.line(x, 50-h, x, 55, 3, amberGold)
			cv.filledCircle(x, 50-h-3, 5, ruby)
		}
		cv.filledRect(96, 55, 48, 8, amberGold)
		// Head
		cv.filledCircle(120, 78, 24, skin)
		cv.glow(120, 78, 12, ivory)
		// Beard
		cv.filledCircle(120, 98, 14, color.RGBA{130, 80, 40, 255})
		// Throne — gothic arch pillars
		cv.filledRect(30, 55, 18, 280, color.RGBA{50, 35, 30, 255})
		cv.filledRect(192, 55, 18, 280, color.RGBA{50, 35, 30, 255})
		cv.filledRect(30, 50, 180, 8, amberGold)
		// Body
		cv.filledRect(80, 108, 80, 130, p)
		cv.leadLine(80, 108, 160, 108, 2)
		// Orb
		cv.filledCircle(175, 180, 15, amberGold)
		cv.glow(175, 180, 12, ivory)
		// Scepter with cross
		cv.line(65, 130, 65, 260, 3, amberGold)
		cv.line(55, 140, 75, 140, 2, amberGold)
		cv.filledCircle(65, 125, 6, ruby)
		// Legs
		cv.filledRect(90, 238, 20, 60, color.RGBA{p.R / 2, p.G / 2, p.B / 2, 255})
		cv.filledRect(130, 238, 20, 60, color.RGBA{p.R / 2, p.G / 2, p.B / 2, 255})

	case 5: // Hierophant — triple cross, columns
		// Gothic columns
		cv.glassPane(25, 80, 25, 270, color.RGBA{35, 28, 22, 255})
		cv.glassPane(190, 80, 25, 270, color.RGBA{35, 28, 22, 255})
		cv.filledRect(23, 75, 29, 8, amberGold)
		cv.filledRect(188, 75, 29, 8, amberGold)
		// Triple cross
		cv.line(120, 45, 120, 135, 3, amberGold)
		cv.line(110, 60, 130, 60, 3, amberGold)
		cv.line(105, 85, 135, 85, 3, amberGold)
		cv.line(108, 115, 132, 115, 3, amberGold)
		cv.glow(120, 85, 15, amberGold)
		// Figure
		cv.filledCircle(120, 170, 22, skin)
		cv.glow(120, 170, 12, ivory)
		// Papal tiara
		cv.filledRect(105, 143, 30, 12, amberGold)
		cv.filledRect(110, 135, 20, 10, amberGold)
		cv.trefoil(120, 133, 8, amberGold)
		// Robes
		cv.filledRect(90, 195, 60, 100, p)
		cv.leadLine(90, 195, 150, 195, 2)
		// Blessing hand
		cv.line(140, 205, 170, 185, 3, skin)
		cv.glow(170, 183, 8, ivory)
		// Two supplicants
		cv.filledCircle(70, 325, 15, skin)
		cv.filledRect(60, 343, 20, 40, color.RGBA{60, 40, 35, 255})
		cv.filledCircle(170, 325, 15, skin)
		cv.filledRect(160, 343, 20, 40, color.RGBA{60, 40, 35, 255})

	case 6: // Lovers — two figures, angel, heart/rose window
		// Mini rose window above
		cv.roseWindow(120, 68, 30)
		// Two figures in separate glass panels
		cv.glassPane(30, 140, 80, 200, color.RGBA{55, 15, 25, 255})
		cv.glassPane(130, 140, 80, 200, color.RGBA{15, 22, 55, 255})
		cv.leadLine(30, 140, 210, 140, 2)
		cv.leadLine(110, 140, 110, 340, 2)
		cv.leadLine(130, 140, 130, 340, 2)
		// Left figure
		cv.filledCircle(70, 180, 18, skin)
		cv.glow(70, 180, 10, ivory)
		cv.filledRect(55, 202, 30, 80, color.RGBA{200, 160, 120, 255})
		cv.filledRect(58, 282, 12, 40, color.RGBA{180, 140, 100, 255})
		cv.filledRect(78, 282, 12, 40, color.RGBA{180, 140, 100, 255})
		// Right figure
		cv.filledCircle(170, 180, 18, skin)
		cv.glow(170, 180, 10, ivory)
		cv.filledRect(155, 202, 30, 80, color.RGBA{180, 120, 140, 255})
		cv.filledRect(158, 282, 12, 40, color.RGBA{160, 100, 120, 255})
		cv.filledRect(178, 282, 12, 40, color.RGBA{160, 100, 120, 255})
		// Hands reaching through divider
		cv.line(85, 225, 110, 225, 2.5, skin)
		cv.line(155, 225, 130, 225, 2.5, skin)
		cv.glow(120, 225, 10, ivory)
		// Garden at bottom
		cv.leadLine(30, 340, 210, 340, 2)
		for x := 40.0; x < 200; x += 25 {
			cv.trefoil(x, 350, 6, emerald)
		}

	case 7: // Chariot — vehicle, stars, sphinxes
		// Starry sky glass panel
		for i := 0; i < 6; i++ {
			x := 45 + float64(i)*30
			y := 55 + float64(i%3)*12
			cv.glow(x, y, 8, amberGold)
			cv.filledCircle(x, y, 3, ivory)
		}
		// Figure
		cv.filledCircle(120, 110, 22, skin)
		cv.glow(120, 110, 12, ivory)
		cv.filledRect(100, 135, 40, 55, sapphire)
		cv.leadLine(100, 135, 140, 135, 2)
		// Chariot body (glass panel)
		cv.glassPane(40, 195, 160, 75, color.RGBA{35, 25, 22, 255})
		cv.leadLine(40, 195, 200, 195, 2)
		cv.leadLine(40, 270, 200, 270, 2)
		cv.leadLine(40, 195, 40, 270, 2)
		cv.leadLine(200, 195, 200, 270, 2)
		// Pentacle emblem
		cv.drawPentacle(120, 232, 28, amberGold)
		cv.glow(120, 232, 12, amberGold)
		// Wheels
		cv.circle(55, 300, 22, 3, amberGold)
		cv.filledCircle(55, 300, 7, amberDim)
		cv.circle(185, 300, 22, 3, amberGold)
		cv.filledCircle(185, 300, 7, amberDim)
		for i := 0; i < 6; i++ {
			a := float64(i) * math.Pi / 3
			cv.line(55+22*math.Cos(a), 300+22*math.Sin(a), 55, 300, 1.5, amberDim)
			cv.line(185+22*math.Cos(a), 300+22*math.Sin(a), 185, 300, 1.5, amberDim)
		}
		// Sphinxes
		cv.filledCircle(60, 345, 13, lead)
		cv.filledCircle(180, 345, 13, ivory)

	case 8: // Strength — figure with lion, infinity, arch
		// Pointed arch at top
		cv.line(40, 100, 120, 45, 2.5, amberGold)
		cv.line(120, 45, 200, 100, 2.5, amberGold)
		// Infinity symbol
		cv.arc(105, 60, 18, 0, 2*math.Pi, 2.5, s)
		cv.arc(135, 60, 18, 0, 2*math.Pi, 2.5, s)
		cv.glow(120, 60, 10, ivory)
		// Figure
		cv.filledCircle(100, 135, 20, skin)
		cv.glow(100, 135, 10, ivory)
		// Robes
		for y := 160.0; y < 280; y++ {
			t := (y - 160) / 120
			w := 15 + t*30
			cv.filledRect(100-w, y, w*2, 1, lerpColor(ivory, color.RGBA{ivory.R / 2, ivory.G / 2, ivory.B / 2, 255}, t))
		}
		// Lion (amber glass)
		cv.filledCircle(155, 225, 30, amberGold)
		cv.glow(155, 225, 18, amberGold)
		cv.arc(155, 225, 35, 0, 2*math.Pi, 2.5, amberDim)
		cv.filledRect(135, 255, 45, 35, amberDim)
		cv.filledRect(138, 290, 12, 25, amberDim)
		cv.filledRect(160, 290, 12, 25, amberDim)
		// Hand on lion
		cv.line(100, 160, 140, 215, 3, skin)
		// Lead line dividers
		cv.leadLine(25, 330, 215, 330, 2)

	case 9: // Hermit — lantern, mountain, star
		// Mountain
		for y := 190; y < 360; y++ {
			t := float64(y-190) / 170
			w := t * 100
			col := lerpColor(color.RGBA{50, 40, 35, 255}, stoneBot, t*0.5)
			cv.filledRect(120-w, float64(y), w*2, 1, col)
		}
		// Path
		cv.bezier(120, 260, 100, 300, 140, 340, 120, 380, 2, amberDim)
		// Figure
		cv.filledCircle(120, 155, 18, skin)
		cv.glow(120, 155, 8, ivory)
		// Hooded cloak
		cv.arc(120, 145, 22, math.Pi, 2*math.Pi, 3, color.RGBA{60, 50, 45, 255})
		cv.filledRect(100, 175, 40, 80, color.RGBA{60, 50, 45, 255})
		// Staff
		cv.line(88, 155, 88, 300, 3, amberDim)
		// Lantern (glowing glass)
		cv.filledRect(155, 140, 15, 20, amberGold)
		cv.glow(162, 150, 25, amberGold)
		cv.glow(162, 150, 15, ivory)
		// Star above (six-pointed)
		cv.glow(120, 50, 18, amberGold)
		cv.filledCircle(120, 50, 6, ivory)
		for i := 0; i < 6; i++ {
			a := float64(i) * math.Pi / 3
			cv.line(120+6*math.Cos(a), 50+6*math.Sin(a),
				120+16*math.Cos(a), 50+16*math.Sin(a), 1.5, amberGold)
		}

	case 10: // Wheel of Fortune — ornate rose window as wheel
		cv.roseWindow(120, 200, 75)
		// Four corner symbols (evangelists in glass)
		cv.glassPane(28, 45, 35, 35, color.RGBA{55, 45, 15, 255})
		cv.glassPane(177, 45, 35, 35, color.RGBA{55, 45, 15, 255})
		cv.glassPane(28, 320, 35, 35, color.RGBA{55, 45, 15, 255})
		cv.glassPane(177, 320, 35, 35, color.RGBA{55, 45, 15, 255})
		cv.filledCircle(45, 62, 12, amberGold)
		cv.filledCircle(195, 62, 12, amberGold)
		cv.filledCircle(45, 338, 12, amberGold)
		cv.filledCircle(195, 338, 12, amberGold)
		cv.glow(45, 62, 8, ivory)
		cv.glow(195, 62, 8, ivory)
		cv.glow(45, 338, 8, ivory)
		cv.glow(195, 338, 8, ivory)

	case 11: // Justice — scales, sword, columns
		// Gothic columns
		cv.glassPane(28, 80, 20, 270, color.RGBA{30, 28, 22, 255})
		cv.glassPane(192, 80, 20, 270, color.RGBA{30, 28, 22, 255})
		cv.filledRect(26, 75, 24, 8, amberGold)
		cv.filledRect(190, 75, 24, 8, amberGold)
		// Sword
		cv.line(120, 48, 120, 230, 3, sapphire)
		cv.line(105, 85, 135, 85, 3, sapphire)
		cv.glow(120, 55, 10, ivory)
		// Figure
		cv.filledCircle(120, 145, 22, skin)
		cv.glow(120, 145, 12, ivory)
		cv.filledRect(102, 135, 36, 6, amberGold)
		// Robes
		for y := 170.0; y < 300; y++ {
			t := (y - 170) / 130
			w := 20 + t*35
			cv.filledRect(120-w, y, w*2, 1, lerpColor(p, amberDim, t))
		}
		// Scales
		cv.leadLine(55, 195, 185, 195, 3)
		cv.arc(70, 225, 18, 0, math.Pi, 2, amberGold)
		cv.arc(170, 225, 18, 0, math.Pi, 2, amberGold)
		cv.leadLine(70, 195, 70, 225, 1.5)
		cv.leadLine(170, 195, 170, 225, 1.5)
		// Pedestal
		cv.filledRect(85, 310, 70, 12, amberDim)
		cv.leadLine(85, 310, 155, 310, 2)

	case 12: // Hanged Man — tree, inverted figure, halo
		// Tree (gothic pillars as trunk)
		cv.filledRect(110, 45, 20, 100, color.RGBA{70, 50, 30, 255})
		cv.bezier(110, 55, 60, 45, 30, 60, 25, 50, 3, color.RGBA{70, 50, 30, 255})
		cv.bezier(130, 55, 180, 45, 210, 60, 215, 50, 3, color.RGBA{70, 50, 30, 255})
		// Leaves as trefoils
		for x := 30.0; x < 220; x += 30 {
			cv.trefoil(x, 48, 8, emerald)
		}
		// Rope
		cv.leadLine(120, 135, 120, 175, 2)
		// Inverted figure
		cv.filledCircle(120, 145, 6, skin)
		cv.line(120, 150, 105, 200, 3, sapphire)
		cv.line(120, 150, 135, 200, 3, sapphire)
		cv.line(105, 200, 135, 200, 2, sapphire)
		cv.filledRect(108, 200, 24, 55, emerald)
		cv.line(108, 225, 88, 255, 2, skin)
		cv.line(132, 225, 155, 255, 2, skin)
		// Head at bottom with golden halo
		cv.filledCircle(120, 280, 20, skin)
		cv.circle(120, 280, 30, 3, amberGold)
		cv.glow(120, 280, 35, amberDim)
		cv.glow(120, 280, 20, ivory)
		// Eyes
		cv.filledCircle(114, 278, 2, color.RGBA{50, 35, 25, 255})
		cv.filledCircle(126, 278, 2, color.RGBA{50, 35, 25, 255})

	case 13: // Death — skeleton, scythe, rose
		// Skull
		cv.filledCircle(120, 105, 35, ivory)
		cv.filledCircle(108, 100, 10, stoneBot)
		cv.filledCircle(132, 100, 10, stoneBot)
		cv.filledCircle(120, 112, 5, stoneBot)
		cv.filledRect(108, 122, 24, 4, stoneBot)
		for x := 108.0; x < 132; x += 6 {
			cv.line(x, 122, x, 126, 1, ivory)
		}
		cv.glow(120, 105, 22, ivory)
		// Scythe
		cv.line(180, 85, 80, 340, 3, amberDim)
		cv.arc(140, 95, 48, math.Pi+0.3, 2*math.Pi-0.3, 3, sapphire)
		cv.glow(140, 65, 12, ivory)
		// Skeletal body
		cv.leadLine(120, 140, 120, 245, 3)
		for i := 0; i < 4; i++ {
			y := 165 + float64(i)*18
			cv.arc(120, y, 18, 0, math.Pi, 1.5, ivory)
		}
		cv.line(120, 245, 100, 310, 2, ivory)
		cv.line(120, 245, 140, 310, 2, ivory)
		// Rose of rebirth (trefoil rose window)
		cv.trefoil(120, 348, 16, ruby)
		cv.glow(120, 348, 12, ruby)

	case 14: // Temperance — angel, two vessels, water
		// Wings
		cv.bezier(90, 105, 35, 65, 15, 55, 25, 95, 3, amberGold)
		cv.bezier(150, 105, 205, 65, 225, 55, 215, 95, 3, amberGold)
		for i := 0; i < 5; i++ {
			t := float64(i) / 5
			cv.bezier(90, 105, 35+t*30, 65+t*15, 15+t*20, 55+t*20, 25+t*15, 95, 1.5, amberDim)
			cv.bezier(150, 105, 205-t*30, 65+t*15, 225-t*20, 55+t*20, 215-t*15, 95, 1.5, amberDim)
		}
		// Halo
		cv.circle(120, 72, 18, 2, amberGold)
		cv.glow(120, 72, 12, amberGold)
		// Head
		cv.filledCircle(120, 95, 22, skin)
		cv.glow(120, 95, 12, ivory)
		// Robes
		for y := 120.0; y < 300; y++ {
			t := (y - 120) / 180
			w := 20 + t*45
			cv.filledRect(120-w, y, w*2, 1, lerpColor(sapphire, saphDim, t*0.3))
		}
		// Two vessels (glass chalices)
		cv.filledRect(55, 180, 25, 35, amberGold)
		cv.leadLine(55, 180, 80, 180, 2)
		cv.filledRect(160, 215, 25, 35, amberGold)
		cv.leadLine(160, 215, 185, 215, 2)
		// Flowing water
		cv.bezier(75, 195, 100, 190, 140, 210, 165, 220, 3, sapphire)
		cv.glow(120, 205, 15, sapphire)
		// Pool at bottom
		cv.glassPane(30, 330, 180, 25, color.RGBA{18, 30, 60, 255})
		cv.leadLine(30, 330, 210, 330, 2)

	case 15: // Devil — horned figure, chains, captives
		// Horns
		cv.bezier(100, 85, 85, 45, 70, 35, 60, 50, 3, ruby)
		cv.bezier(140, 85, 155, 45, 170, 35, 180, 50, 3, ruby)
		// Head
		cv.filledCircle(120, 100, 28, color.RGBA{ruby.R / 2, ruby.G / 2, ruby.B / 2, 255})
		cv.glow(120, 100, 18, ruby)
		// Glowing eyes
		cv.glow(110, 95, 8, amberGold)
		cv.glow(130, 95, 8, amberGold)
		cv.filledCircle(110, 95, 3, ivory)
		cv.filledCircle(130, 95, 3, ivory)
		// Body
		cv.filledRect(90, 130, 60, 100, color.RGBA{ruby.R / 3, ruby.G / 3, ruby.B / 3, 255})
		// Inverted pentagram
		cv.drawPentacle(120, 175, 30, amberGold)
		cv.glow(120, 175, 12, amberGold)
		// Bat wings
		cv.bezier(90, 135, 50, 115, 30, 135, 35, 165, 2, color.RGBA{ruby.R / 3, ruby.G / 3, ruby.B / 3, 255})
		cv.bezier(150, 135, 190, 115, 210, 135, 205, 165, 2, color.RGBA{ruby.R / 3, ruby.G / 3, ruby.B / 3, 255})
		// Chains (lead lines)
		for y := 235.0; y < 310; y += 8 {
			cv.filledCircle(80, y, 3, amberDim)
			cv.filledCircle(160, y, 3, amberDim)
		}
		// Captives
		cv.filledCircle(60, 330, 14, skin)
		cv.filledRect(50, 348, 20, 35, color.RGBA{100, 60, 50, 255})
		cv.filledCircle(180, 330, 14, skin)
		cv.filledRect(170, 348, 20, 35, color.RGBA{100, 60, 50, 255})

	case 16: // Tower — crumbling tower, lightning
		// Tower (stone blocks in glass)
		cv.glassPane(85, 85, 70, 250, color.RGBA{50, 35, 28, 255})
		cv.leadLine(85, 85, 155, 85, 2)
		cv.leadLine(85, 85, 85, 335, 2)
		cv.leadLine(155, 85, 155, 335, 2)
		// Crown fragments
		cv.filledRect(60, 55, 15, 10, color.RGBA{80, 55, 40, 255})
		cv.filledRect(170, 65, 12, 8, color.RGBA{80, 55, 40, 255})
		// Windows (pointed arch)
		for y := 125.0; y < 300; y += 55 {
			cv.line(105, y+20, 112, y, 1.5, amberDim)
			cv.line(112, y, 119, y+20, 1.5, amberDim)
			cv.glow(112, y+10, 5, amberGold)
			cv.line(125, y+35, 132, y+15, 1.5, amberDim)
			cv.line(132, y+15, 139, y+35, 1.5, amberDim)
			cv.glow(132, y+25, 5, amberGold)
		}
		// Lightning
		cv.line(150, 45, 135, 55, 4, amberGold)
		cv.line(135, 55, 155, 70, 4, amberGold)
		cv.line(155, 70, 125, 85, 5, amberGold)
		cv.glow(140, 60, 20, amberGold)
		cv.glow(130, 80, 25, ivory)
		// Explosion at top
		cv.glow(120, 85, 30, ruby)
		// Falling figures
		cv.filledCircle(50, 205, 12, skin)
		cv.filledRect(43, 220, 14, 25, sapphire)
		cv.filledCircle(195, 235, 12, skin)
		cv.filledRect(188, 250, 14, 25, ruby)
		// Flames (glass-tinted)
		for x := 85.0; x < 155; x += 3 {
			h := 18 + 12*math.Sin(x*0.2)
			for y := 335 - h; y < 335; y++ {
				t := (335 - y) / h
				col := lerpColor(ruby, amberGold, t)
				cv.blend(int(x), int(y), col, 0.7)
			}
		}

	case 17: // Star — large star, water bearer
		// Large 8-pointed star
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			for r := 0.0; r < 45; r += 1 {
				px := 120 + r*math.Cos(a)
				py := 95 + r*math.Sin(a)
				cv.blend(int(px), int(py), amberGold, 0.8-r/70)
			}
		}
		cv.glow(120, 95, 30, amberGold)
		cv.filledCircle(120, 95, 12, ivory)
		// Seven smaller stars
		starPos := [][2]float64{{50, 55}, {190, 48}, {35, 125}, {205, 108}, {60, 175}, {185, 165}, {45, 65}}
		for _, sp := range starPos {
			cv.glow(sp[0], sp[1], 8, sapphire)
			cv.filledCircle(sp[0], sp[1], 3, ivory)
		}
		// Water bearer
		cv.filledCircle(120, 235, 18, skin)
		cv.glow(120, 235, 10, ivory)
		cv.filledRect(105, 255, 30, 55, ivory)
		// Water streams (blue glass)
		cv.bezier(95, 270, 70, 295, 60, 330, 50, 355, 3, sapphire)
		cv.bezier(145, 270, 170, 295, 180, 330, 190, 355, 3, sapphire)
		cv.glow(65, 340, 12, sapphire)
		cv.glow(175, 340, 12, sapphire)
		// Pool (glass panel)
		cv.glassPane(30, 348, 180, 12, color.RGBA{18, 30, 60, 255})
		cv.leadLine(30, 348, 210, 348, 1.5)

	case 18: // Moon — crescent, towers, wolves
		// Crescent moon (glass panel)
		cv.filledCircle(120, 85, 40, amethyst)
		cv.filledCircle(138, 75, 40, sc.glass)
		cv.glow(105, 90, 35, amethyst)
		// Moon rays
		for i := 0; i < 8; i++ {
			x := 70 + float64(i)*20
			cv.line(x, 135, x+5, 155, 1.5, amberDim)
		}
		// Two towers (gothic)
		cv.glassPane(28, 185, 32, 145, color.RGBA{35, 28, 25, 255})
		cv.glassPane(180, 185, 32, 145, color.RGBA{35, 28, 25, 255})
		// Crenellations
		for x := 28.0; x < 60; x += 9 {
			cv.filledRect(x, 175, 7, 12, color.RGBA{45, 35, 30, 255})
		}
		for x := 180.0; x < 212; x += 9 {
			cv.filledRect(x, 175, 7, 12, color.RGBA{45, 35, 30, 255})
		}
		// Window glows
		cv.glow(44, 225, 8, amberGold)
		cv.glow(196, 225, 8, amberGold)
		// Winding path
		cv.bezier(120, 355, 100, 330, 140, 300, 120, 270, 3, amberDim)
		cv.bezier(120, 270, 100, 250, 110, 225, 120, 190, 2.5, amberDim)
		// Wolves
		cv.filledCircle(68, 335, 12, color.RGBA{80, 65, 50, 255})
		cv.filledRect(58, 343, 22, 12, color.RGBA{70, 55, 45, 255})
		cv.filledCircle(172, 335, 12, color.RGBA{80, 65, 50, 255})
		cv.filledRect(162, 343, 22, 12, color.RGBA{70, 55, 45, 255})

	case 19: // Sun — radiant sun, child
		// Large sun (rose window style)
		cv.filledCircle(120, 105, 48, amberGold)
		cv.glow(120, 105, 42, amberGold)
		cv.glow(120, 105, 22, ivory)
		// Sun face
		cv.filledCircle(108, 100, 5, color.RGBA{amberDim.R / 2, amberDim.G / 2, amberDim.B / 2, 255})
		cv.filledCircle(132, 100, 5, color.RGBA{amberDim.R / 2, amberDim.G / 2, amberDim.B / 2, 255})
		cv.arc(120, 112, 15, 0.2, math.Pi-0.2, 2, color.RGBA{amberDim.R / 2, amberDim.G / 2, amberDim.B / 2, 255})
		// Rays
		for i := 0; i < 16; i++ {
			a := float64(i) * math.Pi / 8
			r1 := 52.0
			r2 := 78.0
			if i%2 == 0 {
				r2 = 88
			}
			cv.line(120+r1*math.Cos(a), 105+r1*math.Sin(a),
				120+r2*math.Cos(a), 105+r2*math.Sin(a), 2.5, amberGold)
		}
		// Child figure
		cv.filledCircle(120, 265, 16, skin)
		cv.glow(120, 265, 8, ivory)
		cv.filledRect(110, 283, 20, 38, ivory)
		cv.filledRect(112, 321, 10, 22, skin)
		cv.filledRect(128, 321, 10, 22, skin)
		// Arms up
		cv.line(110, 290, 92, 268, 2, skin)
		cv.line(130, 290, 148, 268, 2, skin)
		// Garden (glass panels)
		for x := 35.0; x < 210; x += 45 {
			cv.trefoil(x, 348, 10, emerald)
		}

	case 20: // Judgement — angel, trumpet, rising figures
		// Angel
		cv.filledCircle(120, 65, 20, skin)
		cv.glow(120, 65, 12, ivory)
		cv.circle(120, 50, 22, 2, amberGold)
		cv.glow(120, 50, 15, amberGold)
		// Wings
		cv.bezier(100, 75, 50, 38, 22, 48, 28, 85, 3, amberGold)
		cv.bezier(140, 75, 190, 38, 218, 48, 212, 85, 3, amberGold)
		for i := 0; i < 4; i++ {
			t := float64(i) * 0.15
			cv.bezier(100, 75, 50+t*40, 38+t*20, 22+t*30, 48+t*20, 28+t*20, 85, 1.5, amberDim)
			cv.bezier(140, 75, 190-t*40, 38+t*20, 218-t*30, 48+t*20, 212-t*20, 85, 1.5, amberDim)
		}
		// Trumpet
		cv.line(120, 90, 185, 110, 3, amberGold)
		cv.filledCircle(190, 112, 8, amberGold)
		cv.glow(190, 112, 10, ivory)
		cv.arc(195, 112, 15, -0.5, 0.5, 1.5, amberGold)
		// Clouds
		for x := 25.0; x < 220; x += 40 {
			cy := 135 + 10*math.Sin(x*0.05)
			cv.filledCircle(x, cy, 16, color.RGBA{45, 38, 35, 255})
		}
		// Rising figures (glass panels)
		cv.glassPane(35, 230, 50, 120, color.RGBA{35, 30, 22, 255})
		cv.glassPane(95, 230, 50, 120, color.RGBA{35, 30, 22, 255})
		cv.glassPane(155, 230, 50, 120, color.RGBA{35, 30, 22, 255})
		cv.leadLine(35, 230, 205, 230, 2)
		cv.leadLine(85, 230, 85, 350, 1.5)
		cv.leadLine(95, 230, 95, 350, 1.5)
		cv.leadLine(145, 230, 145, 350, 1.5)
		cv.leadLine(155, 230, 155, 350, 1.5)
		figs := []float64{60, 120, 180}
		for _, fx := range figs {
			cv.filledCircle(fx, 260, 14, skin)
			cv.glow(fx, 260, 8, ivory)
			cv.filledRect(fx-10, 278, 20, 35, ivory)
			cv.line(fx-10, 285, fx-18, 268, 2, skin)
			cv.line(fx+10, 285, fx+18, 268, 2, skin)
		}
		// Glow around rising
		cv.glow(120, 290, 40, amberDim)

	case 21: // World — wreath as rose window, dancing figure
		// Large wreath / rose-window circle
		cv.circle(120, 200, 85, 4, lead)
		cv.circle(120, 200, 80, 2, amberGold)
		// Fill segments between like rose window
		for i := 0; i < 12; i++ {
			a1 := float64(i) * 2 * math.Pi / 12
			a2 := float64(i+1) * 2 * math.Pi / 12
			aMid := (a1 + a2) / 2
			cols := []color.RGBA{emerald, ruby, sapphire, amberGold, amethyst, emerald,
				ruby, sapphire, amberGold, amethyst, emerald, ruby}
			col := cols[i]
			for r := 65.0; r < 80; r += 1 {
				for a := a1 + 0.02; a < a2-0.02; a += 0.02 {
					px := 120 + r*math.Cos(a)
					py := 200 + r*math.Sin(a)
					bright := 0.5 + 0.3*math.Sin((r-65)/15*math.Pi)
					gc := color.RGBA{
						clampU8(float64(col.R) * bright),
						clampU8(float64(col.G) * bright),
						clampU8(float64(col.B) * bright),
						255,
					}
					cv.blend(int(px), int(py), gc, 0.8)
				}
			}
			// Lead spoke
			cv.line(120+65*math.Cos(a1), 200+65*math.Sin(a1),
				120+80*math.Cos(a1), 200+80*math.Sin(a1), 1.5, lead)
			// Trefoil at junctions
			cv.trefoil(120+82*math.Cos(aMid), 200+82*math.Sin(aMid), 5, amberGold)
		}
		// Dancing figure inside
		cv.filledCircle(120, 168, 18, skin)
		cv.glow(120, 168, 10, ivory)
		cv.bezier(105, 183, 95, 220, 100, 255, 110, 270, 2.5, emerald)
		cv.bezier(135, 183, 145, 220, 140, 255, 130, 270, 2.5, emerald)
		cv.filledRect(108, 188, 24, 45, color.RGBA{emerald.R / 2, emerald.G / 2, emerald.B / 2, 255})
		// Arms
		cv.line(105, 198, 78, 178, 2.5, skin)
		cv.line(135, 198, 162, 178, 2.5, skin)
		// Legs
		cv.line(113, 233, 98, 268, 2.5, skin)
		cv.line(127, 233, 142, 268, 2.5, skin)
		// Four corner symbols (evangelist windows)
		corners := [][2]float64{{30, 48}, {210, 48}, {30, 358}, {210, 358}}
		cornerCols := []color.RGBA{amberGold, ruby, sapphire, emerald}
		for i, pos := range corners {
			cv.filledCircle(pos[0], pos[1], 12, cornerCols[i])
			cv.glow(pos[0], pos[1], 10, ivory)
			cv.circle(pos[0], pos[1], 13, 1.5, lead)
		}
	}

	return cv
}

// ========== main ==========

func main() {
	base := "decks/gothic"

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
