package main

import (
	"math"
	"math/rand"
	"time"

	"github.com/charmbracelet/harmonica"
)

type Phase int

const (
	PhaseIdle Phase = iota
	PhaseShuffle
	PhaseDeal
	PhaseReveal
	PhaseDisplay
)

const (
	washBounceTime   = 2500 * time.Millisecond // free bounce phase
	washConvergeTime = 800 * time.Millisecond  // converge to center
	washSettleTime   = 400 * time.Millisecond  // rest at center
	shuffleDuration  = washBounceTime + washConvergeTime + washSettleTime
	washCardCount    = 20
	washCardW        = 28
	washCardH        = 44
	dealInterval     = 400 * time.Millisecond
	revealInterval   = 500 * time.Millisecond
	frameInterval    = 50 * time.Millisecond
)

type WashCard struct {
	X, Y   float64
	VX, VY float64
}

type AnimState struct {
	Phase      Phase
	StartTime  time.Time
	Frame      int
	CardsDealt int // 0-3: how many cards have been dealt
	CardsShown int // 0-3: how many cards have been revealed

	// Harmonica springs for each card position (vertical bounce)
	Springs  [3]harmonica.Spring
	SpringY  [3]float64 // current Y offset
	SpringVY [3]float64 // current velocity

	// Wash shuffle
	WashCards    []WashCard
	WashGrid     [][]byte // parsed card back pixel grid
	ScreenW      int      // terminal columns (= pixel width)
	ScreenH      int      // terminal rows
}

func NewAnimState() AnimState {
	freq := 6.0
	damping := 0.4

	return AnimState{
		Phase:     PhaseIdle,
		StartTime: time.Now(),
		Springs: [3]harmonica.Spring{
			harmonica.NewSpring(harmonica.FPS(60), freq, damping),
			harmonica.NewSpring(harmonica.FPS(60), freq, damping),
			harmonica.NewSpring(harmonica.FPS(60), freq, damping),
		},
		SpringY:  [3]float64{-20, -20, -20},
		WashGrid: ParsePixelGrid(cardBackPixels),
	}
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
	a.WashCards = make([]WashCard, washCardCount)
	for i := range a.WashCards {
		angle := rand.Float64() * 2 * math.Pi
		speed := 5.0 + rand.Float64()*5.0
		a.WashCards[i] = WashCard{
			X:  cx,
			Y:  cy,
			VX: math.Cos(angle) * speed,
			VY: math.Sin(angle) * speed,
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
		if len(a.WashCards) == 0 {
			a.initWashCards()
		}

		bufH := float64((a.ScreenH - 5) * 2)
		maxX := float64(a.ScreenW - washCardW)
		maxY := bufH - washCardH
		cx := maxX / 2
		cy := maxY / 2

		settling := elapsed > washBounceTime+washConvergeTime
		converging := !settling && elapsed > washBounceTime

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
				c.VX += (rand.Float64() - 0.5) * 1.0
				c.VY += (rand.Float64() - 0.5) * 1.0
				speed := math.Sqrt(c.VX*c.VX + c.VY*c.VY)
				if speed > 8 {
					c.VX = c.VX / speed * 8
					c.VY = c.VY / speed * 8
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
		// Deal cards one by one with spring bounce
		targetDealt := int(elapsed/dealInterval) + 1
		if targetDealt > 3 {
			targetDealt = 3
		}
		if targetDealt > a.CardsDealt {
			a.CardsDealt = targetDealt
			// Reset spring for newly dealt card
			idx := a.CardsDealt - 1
			a.SpringY[idx] = -15
			a.SpringVY[idx] = 0
		}

		// Update springs for dealt cards
		for i := 0; i < a.CardsDealt; i++ {
			a.SpringY[i], a.SpringVY[i] = a.Springs[i].Update(
				a.SpringY[i], a.SpringVY[i], 0,
			)
		}

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

		// Keep updating springs
		for i := 0; i < 3; i++ {
			a.SpringY[i], a.SpringVY[i] = a.Springs[i].Update(
				a.SpringY[i], a.SpringVY[i], 0,
			)
		}

		if a.CardsShown >= 3 && elapsed > 3*revealInterval+300*time.Millisecond {
			a.Phase = PhaseDisplay
		}

	case PhaseDisplay:
		// Settle springs
		for i := 0; i < 3; i++ {
			a.SpringY[i], a.SpringVY[i] = a.Springs[i].Update(
				a.SpringY[i], a.SpringVY[i], 0,
			)
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

// CardYOffset returns the vertical offset for card animation (in lines)
func (a *AnimState) CardYOffset(i int) int {
	if !a.CardVisible(i) {
		return -20
	}
	y := int(a.SpringY[i])
	if y < -10 {
		return -10
	}
	if y > 3 {
		return 3
	}
	return y
}
