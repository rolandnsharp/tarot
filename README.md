# Terminal Tarot

A terminal tarot card reader with pixel art cards, shuffle animations, and AI-powered readings.

![demo](https://raw.githubusercontent.com/rolandnsharp/tarot/main/demo.gif)

## Install

Download the latest release for your platform from [GitHub Releases](https://github.com/rolandnsharp/tarot/releases), then:

```bash
tar xzf tarot-*.tar.gz
cd tarot-*/
./tarot
```

The archive includes the binary, card decks, sounds, and music — no Go installation needed.

### Build from Source

If you prefer to build from source (requires Go 1.24+):

```bash
go build -o tarot .
./tarot
```

Type a question for the cards (or press Enter to skip), then watch the shuffle and reading unfold.

## AI Readings

Readings are powered by a local language model via [Ollama](https://ollama.com). Small models are ideal here — tarot is poetry, not precision. A 0.6B parameter model runs on almost anything and gives beautifully loose, creative interpretations.

### Quick Setup

1. **Install Ollama:**

```bash
curl -fsSL https://ollama.com/install.sh | sh
```

2. **Pull the model:**

```bash
ollama pull qwen3:0.6b
```

3. **Start the server** (runs on port 11434 by default):

```bash
ollama serve
```

That's it. The included `tarot.md` is already configured to connect to Ollama locally. Run `./tarot`, type a question, and the reading begins.

### Configuration

The `tarot.md` file controls the reading experience:

```markdown
## Interpreter

## Querent
The person sitting at this terminal, whoever they may be.

## Deck
gothic

## Connection
provider: ollama
model: qwen3:0.6b
base_url: http://localhost:11434/v1
api_key: unused
```

- **Interpreter** — custom personality for the reader. When left blank, the deck itself sets the tone (each deck has a built-in voice — the cyberpunk deck reads like a neon-lit street oracle, the gothic deck speaks like light through cathedral glass, etc). Add text here to override with your own personality.
- **Querent** — context about who's being read (make it personal or keep it generic)
- **Deck** — `cyberpunk`, `nouveau`, or `gothic`
- **Connection** — works with any OpenAI-compatible API (Ollama, OpenAI, OpenRouter, etc)

### Using Other Providers

For OpenAI:
```
provider: openai
model: gpt-4o-mini
api_key: sk-...
```

For any OpenAI-compatible endpoint:
```
provider: openai
model: your-model
base_url: https://your-endpoint/v1
api_key: your-key
```

## Entropy

The shuffle draws from real-world entropy. Go's runtime seeds its PRNG from the operating system's cryptographic entropy pool at startup — on Linux this is `getrandom`, fed by hardware interrupt timing, thermal noise, and disk jitter. Every launch produces a unique, unreproducible shuffle without any manual seeding. The cards are sorted by the chaos of the physical world.

## Music

Place `.wav` files in a `music/` directory next to the binary. A random track will loop during each reading. Requires `aplay` (part of `alsa-utils`) for audio playback. If `aplay` is not available or the directory is missing, the app runs silently.

```bash
sudo apt install alsa-utils  # Debian/Ubuntu
```

### Included Tracks

- [Luminis](https://freesound.org/s/722399/) by Vrymaa — CC0
- [Mystical Guitar Atmosphere](https://freesound.org/s/719441/) by MichiJung — [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/)
- [Somnium](https://freesound.org/s/722400/) by Vrymaa — [CC BY-NC 4.0](https://creativecommons.org/licenses/by-nc/4.0/)

## Sound Effects

- [Card Flip](https://freesound.org/s/240776/) by f4ngy — [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/)
- [Sliding Paper on Table](https://freesound.org/s/46631/) by 123jorre456 — CC0

## Controls

- **Enter** — begin the reading
- **r** — reshuffle and start over
- **q** / **Esc** — quit
