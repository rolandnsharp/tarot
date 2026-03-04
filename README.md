# Terminal Tarot

A terminal tarot card reader with pixel art cards, shuffle animations, and AI-powered readings.

![demo](https://raw.githubusercontent.com/rolandnsharp/tarot/main/demo.gif)

## Usage

```bash
go build -o tarot .
./tarot
```

Type a question for the cards (or press Enter to skip), then watch the shuffle and reading unfold.

## AI Readings

Create a `tarot.md` file in the same directory to enable AI-powered readings:

```markdown
## Interpreter
A mysterious oracle who speaks in poetic riddles.

## Querent
Your name, sign, interests — whatever context you want the reader to have.

## Connection
provider: ollama
model: qwen3:0.6b
base_url: http://localhost:11434/v1
api_key: unused
```

Works with any OpenAI-compatible API (Ollama, OpenAI, etc).

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
