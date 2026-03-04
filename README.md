# Terminal Tarot

A terminal tarot card reader with pixel art cards, shuffle animations, and AI-powered readings.

![demo](demo.gif)

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

## Controls

- **Enter** — begin the reading
- **r** — reshuffle and start over
- **q** / **Esc** — quit
