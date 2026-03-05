package main

import (
	"math"
	"math/rand"
	"path/filepath"
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
	washCardW = 28
	washCardH        = 44
	dealInterval     = 400 * time.Millisecond
	revealInterval   = 500 * time.Millisecond
	frameInterval    = 50 * time.Millisecond
)

type WashCard struct {
	X, Y   float64
	VX, VY float64
	Z      int // draw order: lower Z drawn first (behind)
}

type AnimState struct {
	Phase      Phase
	StartTime  time.Time
	Frame      int
	CardsDealt int // 0-3: how many cards have been dealt
	CardsShown int // 0-3: how many cards have been revealed

	// Wash shuffle
	WashCards   []WashCard
	WashRGB     [44][28][3]uint8 // card back pixel grid (RGB mode)
	ScreenW     int         // terminal columns (= pixel width)
	ScreenH     int         // terminal rows
}

func NewAnimState() AnimState {
	as := AnimState{
		Phase:     PhaseIdle,
		StartTime: time.Now(),
	}
	dir := deckDir()
	if dir != "" {
		if pixels, err := LoadHexCard(filepath.Join(dir, "back.hex")); err == nil {
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
}

func (a *AnimState) initWashCards() {
	bufH := (a.ScreenH - 5) * 2
	cx := float64(a.ScreenW-washCardW) / 2
	cy := float64(bufH-washCardH) / 2
	a.WashCards = make([]WashCard, len(Deck))
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
	elapsed := time.Since(a.StartTime)

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

		// Re-randomize all Z values every ~15 frames for dynamic layering
		if !converging && !settling && !pausing && a.Frame%15 == 0 {
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
				pull := 0.1 + ease*0.3
				c.X += (cx - c.X) * pull
				c.Y += (cy - c.Y) * pull
				c.VX *= 0.85
				c.VY *= 0.85
				c.X += c.VX
				c.Y += c.VY
			} else {
				// Free bounce with random perturbation
				c.VX += (rand.Float64() - 0.5) * 0.75
				c.VY += (rand.Float64() - 0.5) * 0.75
				speed := math.Sqrt(c.VX*c.VX + c.VY*c.VY)
				if speed > 6 {
					c.VX = c.VX / speed * 6
					c.VY = c.VY / speed * 6
				}
				c.X += c.VX
				c.Y += c.VY

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
