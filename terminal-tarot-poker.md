# Terminal Tarot Poker

## Concept
Deal tarot cards like Texas Hold'em poker.

## Deal Sequence
1. **Hole cards** — deal the querent two face-down cards, then reveal them
2. **The Flop** — deal three community cards face-down, then reveal
3. **The Turn** — deal one community card face-down, then reveal
4. **The River** — deal one community card face-down, then reveal

## Roles
- **Querent's hole cards** — the querent's personal cards (like the player's hand)
- **Community cards (flop/turn/river)** — shared cards the interpreter reads from

## Interpreter
The AI reads the spread using the poker-style layout, interpreting the querent's hole cards in combination with the community cards as they're revealed.

---

# Themed Decks

## Concept
Support multiple art decks that anyone can contribute via pull request.

## Deck Selection
- Run `tarot deck` to get an interactive selection menu of all available decks
- Decks live in a `decks/` folder, one subfolder per deck
- Selected deck is remembered (stored in config or a dotfile)

## Art Style
Each deck is pixel art using a limited character set for shading and edges:
- `.` — light fill / empty space
- `-` — horizontal edges
- `\` `/` — diagonal edges
- `|` — vertical edges
- `~` — curves / waves
- `#` — heavy fill / solid areas
- `█` — full block (solid fill)
- `▀` — upper half block
- `▄` — lower half block
- `▌` — left half block
- `▐` — right half block
- `▖` — lower left quarter
- `▗` — lower right quarter
- `▘` — upper left quarter
- `▝` — upper right quarter

Each character can have its own foreground color per-cell, giving definition within the half-block pixel grid.

## Deck Structure
```
decks/
  classic/
    back.txt       # card back art
    major/         # major arcana (00-fool.txt, 01-magician.txt, ...)
    cups/          # minor arcana suits
    wands/
    swords/
    pentacles/
  cosmic/
    back.txt
    major/
    ...
```

## Contributing a Deck
Anyone can submit a new deck via pull request by adding a new folder under `decks/` with the required art files.
