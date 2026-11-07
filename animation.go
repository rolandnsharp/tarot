package main

import (
	"math"
	"math/rand"
	"path"
	"time"
)

type Phase int

const (
	PhaseIdle Phase = iota
	PhaseQuestion
	PhaseShuffle
	PhaseDeal
	PhaseReveal
	PhaseDisplay
	PhaseReading
)

const (
	washBounceTime   = 3750 * time.Millisecond // free bounce phase
	washConvergeTime = 1200 * time.Millisecond // converge to center
	washSettleTime   = 600 * time.Millisecond  // rest at center (visible deck)
	washPauseTime    = 1500 * time.Millisecond // cards picked up, empty screen
	shuffleDuration  = washBounceTime + washConvergeTime + washSettleTime + washPauseTime
	washCardW        = 28
	washCardH        = 44
	washCount        = 22                     // the Major Arcana
	washZInterval    = 750 * time.Millisecond // how often the layering is shuffled
	dealInterval     = 400 * time.Millisecond
	revealInterval   = 500 * time.Millisecond
	frameInterval    = 50 * time.Millisecond
)

type WashCard struct {
	X, Y   float64
	VX, VY float64
	Z      int // draw order: lower Z drawn first (behind)
}

// Velocities and per-step pulls below are per frameInterval. Each tick scales them by
// the time since the last one, so a slow link that draws fewer frames still shuffles
// for the same time at the same speeds.

type AnimState struct {
	Phase      Phase
	StartTime  time.Time
	Frame      int
	CardsDealt int // 0-3: how many cards have been dealt
	CardsShown int // 0-3: how many cards have been revealed

	// Wash shuffle
	WashCards []WashCard
	WashRGB   [][]RGB3 // card back pixel grid (RGB mode)
	ScreenW   int      // terminal columns (= pixel width)
	ScreenH   int      // terminal rows
	lastTick  time.Time
	lastZ     time.Time
}

func NewAnimState() AnimState {
	as := AnimState{
		Phase:     PhaseIdle,
		StartTime: time.Now(),
	}
	dir := deckDir()
	if dir != "" {
		if pixels, err := LoadHexCard(path.Join(dir, "back.hex")); err == nil {
			as.WashRGB = BuildRGBCardFrame(pixels, deckBorderColor())
		}
	}
	return as
}

func (a *AnimState) Start() {
	a.Phase = PhaseShuffle
	a.StartTime = time.Now()
	a.Frame = 0
	a.WashCards = nil // initialized lazily when screen size is known
	a.lastTick = a.StartTime
	a.lastZ = a.StartTime
}

func (a *AnimState) initWashCards() {
	bufH := (a.ScreenH - 5) * 2
	cx := float64(a.ScreenW-washCardW) / 2
	cy := float64(bufH-washCardH) / 2
	a.WashCards = make([]WashCard, washCount)
	for i := range a.WashCards {
		angle := rand.Float64() * 2 * math.Pi
		speed := 3.75 + rand.Float64()*3.75
		a.WashCards[i] = WashCard{
			X:  cx,
			Y:  cy,
			VX: math.Cos(angle) * speed,
			VY: math.Sin(angle) * speed,
			Z:  rand.Intn(1000),
		}
	}
}

func (a *AnimState) Tick() {
	a.Frame++
	now := time.Now()
	elapsed := now.Sub(a.StartTime)
	// f: how many nominal frames this tick stands for, capped so a long stall
	// doesn't fling the cards across the screen.
	f := float64(now.Sub(a.lastTick)) / float64(frameInterval)
	if f > 5 {
		f = 5
	}
	a.lastTick = now

	switch a.Phase {
	case PhaseShuffle:
		if a.ScreenW < washCardW || a.ScreenH < 10 {
			break // wait for valid screen size
		}
		if a.WashCards == nil {
			a.initWashCards()
		}

		bufH := float64((a.ScreenH - 5) * 2)
		maxX := float64(a.ScreenW - washCardW)
		maxY := bufH - washCardH
		cx := maxX / 2
		cy := maxY / 2

		pausing := elapsed > washBounceTime+washConvergeTime+washSettleTime
		settling := !pausing && elapsed > washBounceTime+washConvergeTime
		converging := !settling && !pausing && elapsed > washBounceTime

		if pausing {
			a.WashCards = a.WashCards[:0] // cards picked up, nothing to render
		}

		// Re-randomize all Z values now and then for dynamic layering
		if !converging && !settling && !pausing && now.Sub(a.lastZ) >= washZInterval {
			a.lastZ = now
			for k := range a.WashCards {
				a.WashCards[k].Z = rand.Intn(1000)
			}
		}

		for i := range a.WashCards {
			c := &a.WashCards[i]

			if settling {
				// Snap to center stack
				c.X = cx
				c.Y = cy
				c.VX = 0
				c.VY = 0
			} else if converging {
				// Lerp toward center with increasing strength
				t := float64(elapsed-washBounceTime) / float64(washConvergeTime)
				ease := t * t * t // cubic ease-in
				pull := 1 - math.Pow(1-(0.1+ease*0.3), f)
				c.X += (cx - c.X) * pull
				c.Y += (cy - c.Y) * pull
				damp := math.Pow(0.85, f)
				c.VX *= damp
				c.VY *= damp
				c.X += c.VX * f
				c.Y += c.VY * f
			} else {
				// Free bounce with random perturbation
				jitter := 0.75 * math.Sqrt(f)
				c.VX += (rand.Float64() - 0.5) * jitter
				c.VY += (rand.Float64() - 0.5) * jitter
				speed := math.Sqrt(c.VX*c.VX + c.VY*c.VY)
				if speed > 6 {
					c.VX = c.VX / speed * 6
					c.VY = c.VY / speed * 6
				}
				c.X += c.VX * f
				c.Y += c.VY * f

				// Bounce off edges
				if c.X < 0 {
					c.X = 0
					c.VX = math.Abs(c.VX)
				} else if c.X > maxX {
					c.X = maxX
					c.VX = -math.Abs(c.VX)
				}
				if c.Y < 0 {
					c.Y = 0
					c.VY = math.Abs(c.VY)
				} else if c.Y > maxY {
					c.Y = maxY
					c.VY = -math.Abs(c.VY)
				}
			}
		}

		if elapsed > shuffleDuration {
			a.Phase = PhaseDeal
			a.StartTime = time.Now()
			a.CardsDealt = 0
			a.WashCards = nil
		}

	case PhaseDeal:
		// Deal cards one by one
		targetDealt := int(elapsed/dealInterval) + 1
		if targetDealt > 3 {
			targetDealt = 3
		}
		a.CardsDealt = targetDealt

		if a.CardsDealt >= 3 && elapsed > 3*dealInterval+500*time.Millisecond {
			a.Phase = PhaseReveal
			a.StartTime = time.Now()
			a.CardsShown = 0
		}

	case PhaseReveal:
		targetShown := int(elapsed/revealInterval) + 1
		if targetShown > 3 {
			targetShown = 3
		}
		a.CardsShown = targetShown

		if a.CardsShown >= 3 && elapsed > 3*revealInterval+300*time.Millisecond {
			a.Phase = PhaseDisplay
		}
	}
}

// CardVisible returns whether card at index i should be visible
func (a *AnimState) CardVisible(i int) bool {
	return a.Phase >= PhaseDeal && i < a.CardsDealt
}

// CardRevealed returns whether card at index i should show its face
func (a *AnimState) CardRevealed(i int) bool {
	return a.Phase >= PhaseReveal && i < a.CardsShown
}
