package main

import (
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
	shuffleDuration = 2 * time.Second
	dealInterval    = 400 * time.Millisecond
	revealInterval  = 500 * time.Millisecond
	frameInterval   = 50 * time.Millisecond
)

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

	// Shuffle animation offset
	ShuffleOffset float64
	ShuffleSpring harmonica.Spring
	ShuffleVel    float64
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
		ShuffleSpring: harmonica.NewSpring(harmonica.FPS(60), 8.0, 0.3),
		SpringY:       [3]float64{-20, -20, -20}, // start above screen
	}
}

func (a *AnimState) Start() {
	a.Phase = PhaseShuffle
	a.StartTime = time.Now()
	a.Frame = 0
	a.ShuffleOffset = 5
}

func (a *AnimState) Tick() {
	a.Frame++
	elapsed := time.Since(a.StartTime)

	switch a.Phase {
	case PhaseShuffle:
		// Animate shuffle jitter
		a.ShuffleOffset, a.ShuffleVel = a.ShuffleSpring.Update(
			a.ShuffleOffset, a.ShuffleVel, 0,
		)
		if elapsed > shuffleDuration {
			a.Phase = PhaseDeal
			a.StartTime = time.Now()
			a.CardsDealt = 0
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

// ShuffleFrame returns a frame index for the shuffle animation (0-3)
func (a *AnimState) ShuffleFrame() int {
	return a.Frame % 4
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
