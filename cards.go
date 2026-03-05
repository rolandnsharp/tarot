package main

import (
	"math/rand"
)

type Suit int

const (
	MajorArcana Suit = iota
	Wands
	Cups
	Swords
	Pentacles
)

type Card struct {
	Name     string
	Number   int
	Suit     Suit
	Upright  string
	Reversed string
}

func (c Card) IsMajor() bool {
	return c.Suit == MajorArcana
}

// PlayingCardSymbol returns the playing card suit symbol.
// Wands→♣, Cups→♥, Swords→♠, Pentacles→♦
func (s Suit) PlayingCardSymbol() string {
	switch s {
	case Wands:
		return "♣"
	case Cups:
		return "♥"
	case Swords:
		return "♠"
	case Pentacles:
		return "♦"
	default:
		return "★"
	}
}

func (c Card) SuitName() string {
	switch c.Suit {
	case Wands:
		return "Wands"
	case Cups:
		return "Cups"
	case Swords:
		return "Swords"
	case Pentacles:
		return "Pentacles"
	default:
		return ""
	}
}

var Deck = []Card{
	// Major Arcana
	{Name: "The Fool", Number: 0, Suit: MajorArcana, Upright: "New beginnings, innocence, spontaneity", Reversed: "Recklessness, risk-taking, negligence"},
	{Name: "The Magician", Number: 1, Suit: MajorArcana, Upright: "Willpower, desire, resourcefulness", Reversed: "Manipulation, poor planning, untapped talents"},
	{Name: "The High Priestess", Number: 2, Suit: MajorArcana, Upright: "Intuition, mystery, subconscious mind", Reversed: "Secrets, withdrawal, silence"},
	{Name: "The Empress", Number: 3, Suit: MajorArcana, Upright: "Fertility, nurturing, abundance", Reversed: "Dependence, smothering, emptiness"},
	{Name: "The Emperor", Number: 4, Suit: MajorArcana, Upright: "Authority, structure, control", Reversed: "Tyranny, rigidity, coldness"},
	{Name: "The Hierophant", Number: 5, Suit: MajorArcana, Upright: "Tradition, conformity, spiritual wisdom", Reversed: "Rebellion, subversiveness, new approaches"},
	{Name: "The Lovers", Number: 6, Suit: MajorArcana, Upright: "Love, harmony, relationships", Reversed: "Disharmony, imbalance, misalignment"},
	{Name: "The Chariot", Number: 7, Suit: MajorArcana, Upright: "Determination, willpower, triumph", Reversed: "Lack of control, aggression, obstacles"},
	{Name: "Strength", Number: 8, Suit: MajorArcana, Upright: "Courage, persuasion, inner strength", Reversed: "Self-doubt, weakness, insecurity"},
	{Name: "The Hermit", Number: 9, Suit: MajorArcana, Upright: "Soul-searching, introspection, solitude", Reversed: "Isolation, loneliness, withdrawal"},
	{Name: "Wheel of Fortune", Number: 10, Suit: MajorArcana, Upright: "Change, cycles, fate", Reversed: "Bad luck, resistance to change, breaking cycles"},
	{Name: "Justice", Number: 11, Suit: MajorArcana, Upright: "Fairness, truth, law", Reversed: "Unfairness, dishonesty, lack of accountability"},
	{Name: "The Hanged Man", Number: 12, Suit: MajorArcana, Upright: "Pause, surrender, new perspective", Reversed: "Delays, resistance, stalling"},
	{Name: "Death", Number: 13, Suit: MajorArcana, Upright: "Endings, transformation, transition", Reversed: "Resistance to change, fear, stagnation"},
	{Name: "Temperance", Number: 14, Suit: MajorArcana, Upright: "Balance, moderation, patience", Reversed: "Imbalance, excess, lack of vision"},
	{Name: "The Devil", Number: 15, Suit: MajorArcana, Upright: "Bondage, materialism, shadow self", Reversed: "Release, breaking free, power reclaimed"},
	{Name: "The Tower", Number: 16, Suit: MajorArcana, Upright: "Sudden change, upheaval, revelation", Reversed: "Avoidance, fear of change, delayed disaster"},
	{Name: "The Star", Number: 17, Suit: MajorArcana, Upright: "Hope, faith, renewal", Reversed: "Lack of faith, despair, disconnection"},
	{Name: "The Moon", Number: 18, Suit: MajorArcana, Upright: "Illusion, fear, subconscious", Reversed: "Release of fear, clarity, truth"},
	{Name: "The Sun", Number: 19, Suit: MajorArcana, Upright: "Joy, success, vitality", Reversed: "Negativity, sadness, temporary depression"},
	{Name: "Judgement", Number: 20, Suit: MajorArcana, Upright: "Rebirth, inner calling, absolution", Reversed: "Self-doubt, refusal of self-examination"},
	{Name: "The World", Number: 21, Suit: MajorArcana, Upright: "Completion, integration, accomplishment", Reversed: "Incompletion, shortcuts, emptiness"},

	// Wands
	{Name: "Ace of Wands", Number: 1, Suit: Wands, Upright: "Inspiration, new opportunities, growth", Reversed: "Lack of direction, delays, missed chances"},
	{Name: "Two of Wands", Number: 2, Suit: Wands, Upright: "Future planning, progress, decisions", Reversed: "Fear of change, indecision, playing safe"},
	{Name: "Three of Wands", Number: 3, Suit: Wands, Upright: "Expansion, foresight, overseas opportunities", Reversed: "Obstacles, delays, lack of foresight"},
	{Name: "Four of Wands", Number: 4, Suit: Wands, Upright: "Celebration, harmony, homecoming", Reversed: "Conflict, transition, lack of support"},
	{Name: "Five of Wands", Number: 5, Suit: Wands, Upright: "Competition, conflict, tension", Reversed: "Avoiding conflict, compromise, peace"},
	{Name: "Six of Wands", Number: 6, Suit: Wands, Upright: "Victory, success, recognition", Reversed: "Excess pride, lack of recognition, fall from grace"},
	{Name: "Seven of Wands", Number: 7, Suit: Wands, Upright: "Challenge, perseverance, defence", Reversed: "Giving up, overwhelmed, yielding"},
	{Name: "Eight of Wands", Number: 8, Suit: Wands, Upright: "Speed, action, movement", Reversed: "Delays, frustration, waiting"},
	{Name: "Nine of Wands", Number: 9, Suit: Wands, Upright: "Resilience, courage, persistence", Reversed: "Exhaustion, giving up, overwhelmed"},
	{Name: "Ten of Wands", Number: 10, Suit: Wands, Upright: "Burden, responsibility, hard work", Reversed: "Unable to delegate, overstressed, burnt out"},
	{Name: "Page of Wands", Number: 11, Suit: Wands, Upright: "Enthusiasm, exploration, discovery", Reversed: "Setbacks, lack of direction, procrastination"},
	{Name: "Knight of Wands", Number: 12, Suit: Wands, Upright: "Energy, passion, adventure", Reversed: "Haste, scattered energy, delays"},
	{Name: "Queen of Wands", Number: 13, Suit: Wands, Upright: "Courage, confidence, independence", Reversed: "Selfishness, jealousy, insecurities"},
	{Name: "King of Wands", Number: 14, Suit: Wands, Upright: "Leadership, vision, entrepreneur", Reversed: "Impulsiveness, haste, ruthless"},

	// Cups
	{Name: "Ace of Cups", Number: 1, Suit: Cups, Upright: "New feelings, intuition, intimacy", Reversed: "Emotional loss, blocked creativity, emptiness"},
	{Name: "Two of Cups", Number: 2, Suit: Cups, Upright: "Unity, partnership, connection", Reversed: "Imbalance, broken communication, tension"},
	{Name: "Three of Cups", Number: 3, Suit: Cups, Upright: "Celebration, friendship, community", Reversed: "Overindulgence, gossip, isolation"},
	{Name: "Four of Cups", Number: 4, Suit: Cups, Upright: "Meditation, contemplation, apathy", Reversed: "Retreat, withdrawal, checking in"},
	{Name: "Five of Cups", Number: 5, Suit: Cups, Upright: "Loss, regret, disappointment", Reversed: "Acceptance, moving on, finding peace"},
	{Name: "Six of Cups", Number: 6, Suit: Cups, Upright: "Nostalgia, childhood memories, innocence", Reversed: "Stuck in the past, naivety, unrealistic"},
	{Name: "Seven of Cups", Number: 7, Suit: Cups, Upright: "Fantasy, illusion, wishful thinking", Reversed: "Illusion, temptation, diversionary tactics"},
	{Name: "Eight of Cups", Number: 8, Suit: Cups, Upright: "Walking away, disillusionment, leaving behind", Reversed: "Avoidance, fear of change, staying"},
	{Name: "Nine of Cups", Number: 9, Suit: Cups, Upright: "Contentment, satisfaction, gratitude", Reversed: "Inner happiness, materialism, dissatisfaction"},
	{Name: "Ten of Cups", Number: 10, Suit: Cups, Upright: "Harmony, marriage, happiness", Reversed: "Broken family, domestic disharmony, misalignment"},
	{Name: "Page of Cups", Number: 11, Suit: Cups, Upright: "Creative opportunities, intuitive messages", Reversed: "Emotional immaturity, insecurity, disappointment"},
	{Name: "Knight of Cups", Number: 12, Suit: Cups, Upright: "Romance, charm, imagination", Reversed: "Unrealistic, jealousy, moodiness"},
	{Name: "Queen of Cups", Number: 13, Suit: Cups, Upright: "Compassion, calm, comfort", Reversed: "Inner feelings, self-care, self-love"},
	{Name: "King of Cups", Number: 14, Suit: Cups, Upright: "Emotional balance, generosity, diplomacy", Reversed: "Self-compassion, inner feelings, moodiness"},

	// Swords
	{Name: "Ace of Swords", Number: 1, Suit: Swords, Upright: "Clarity, breakthrough, new ideas", Reversed: "Confusion, chaos, lack of clarity"},
	{Name: "Two of Swords", Number: 2, Suit: Swords, Upright: "Difficult decisions, indecision, stalemate", Reversed: "Indecision, confusion, information overload"},
	{Name: "Three of Swords", Number: 3, Suit: Swords, Upright: "Heartbreak, suffering, grief", Reversed: "Recovery, forgiveness, moving on"},
	{Name: "Four of Swords", Number: 4, Suit: Swords, Upright: "Rest, recovery, contemplation", Reversed: "Exhaustion, burn out, stagnation"},
	{Name: "Five of Swords", Number: 5, Suit: Swords, Upright: "Conflict, disagreements, competition", Reversed: "Reconciliation, making amends, past resentment"},
	{Name: "Six of Swords", Number: 6, Suit: Swords, Upright: "Transition, leaving behind, moving on", Reversed: "Emotional baggage, unresolved issues, resistance"},
	{Name: "Seven of Swords", Number: 7, Suit: Swords, Upright: "Deception, trickery, tactics", Reversed: "Coming clean, rethinking approach, confession"},
	{Name: "Eight of Swords", Number: 8, Suit: Swords, Upright: "Imprisonment, entrapment, self-victimization", Reversed: "Self-acceptance, new perspective, freedom"},
	{Name: "Nine of Swords", Number: 9, Suit: Swords, Upright: "Anxiety, worry, fear", Reversed: "Inner turmoil, deep-seated fears, secrets"},
	{Name: "Ten of Swords", Number: 10, Suit: Swords, Upright: "Painful endings, deep wounds, betrayal", Reversed: "Recovery, regeneration, fear of ruin"},
	{Name: "Page of Swords", Number: 11, Suit: Swords, Upright: "New ideas, curiosity, thirst for knowledge", Reversed: "Deception, manipulation, all talk"},
	{Name: "Knight of Swords", Number: 12, Suit: Swords, Upright: "Ambitious, action-oriented, fast-thinking", Reversed: "Restless, unfocused, burnout"},
	{Name: "Queen of Swords", Number: 13, Suit: Swords, Upright: "Independent, unbiased judgement, clear boundaries", Reversed: "Overly emotional, easily influenced, bitchy"},
	{Name: "King of Swords", Number: 14, Suit: Swords, Upright: "Intellectual power, authority, truth", Reversed: "Manipulative, tyrannical, abusive"},

	// Pentacles
	{Name: "Ace of Pentacles", Number: 1, Suit: Pentacles, Upright: "New financial opportunity, prosperity", Reversed: "Lost opportunity, lack of planning, scarcity"},
	{Name: "Two of Pentacles", Number: 2, Suit: Pentacles, Upright: "Balance, adaptability, time management", Reversed: "Overwhelmed, disorganized, financial loss"},
	{Name: "Three of Pentacles", Number: 3, Suit: Pentacles, Upright: "Teamwork, collaboration, learning", Reversed: "Disharmony, misalignment, working alone"},
	{Name: "Four of Pentacles", Number: 4, Suit: Pentacles, Upright: "Conservation, security, frugality", Reversed: "Greediness, materialism, self-protection"},
	{Name: "Five of Pentacles", Number: 5, Suit: Pentacles, Upright: "Financial loss, poverty, isolation", Reversed: "Recovery, charity, improvement"},
	{Name: "Six of Pentacles", Number: 6, Suit: Pentacles, Upright: "Generosity, charity, giving", Reversed: "Strings attached, stinginess, power dynamics"},
	{Name: "Seven of Pentacles", Number: 7, Suit: Pentacles, Upright: "Long-term view, perseverance, investment", Reversed: "Lack of vision, limited success, impatience"},
	{Name: "Eight of Pentacles", Number: 8, Suit: Pentacles, Upright: "Apprenticeship, skill development, diligence", Reversed: "Self-development, perfectionism, misdirected"},
	{Name: "Nine of Pentacles", Number: 9, Suit: Pentacles, Upright: "Abundance, luxury, self-sufficiency", Reversed: "Financial setbacks, reckless spending, superficiality"},
	{Name: "Ten of Pentacles", Number: 10, Suit: Pentacles, Upright: "Wealth, inheritance, family", Reversed: "Financial failure, loneliness, loss"},
	{Name: "Page of Pentacles", Number: 11, Suit: Pentacles, Upright: "Ambition, desire, diligence", Reversed: "Lack of progress, procrastination, learn from failure"},
	{Name: "Knight of Pentacles", Number: 12, Suit: Pentacles, Upright: "Efficiency, routine, conservatism", Reversed: "Laziness, obsessiveness, work without reward"},
	{Name: "Queen of Pentacles", Number: 13, Suit: Pentacles, Upright: "Nurturing, practical, providing financially", Reversed: "Financial independence, self-care, work-home conflict"},
	{Name: "King of Pentacles", Number: 14, Suit: Pentacles, Upright: "Wealth, business, leadership, security", Reversed: "Financially inept, obsessed with wealth, stubborn"},
}

func ShuffleDeck() []Card {
	shuffled := make([]Card, len(Deck))
	copy(shuffled, Deck)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	return shuffled
}

func DrawThree() []Card {
	shuffled := ShuffleDeck()
	return shuffled[:3]
}
