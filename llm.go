package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"
)

// Messages for bubbletea
type readingChunkMsg struct{ Text string }
type readingDoneMsg struct{}
type readingErrMsg struct{ Err error }

// streamMsg is the union sent over the channel
type streamMsg interface{}

func buildPrompt(cfg Config, cards []Card, question string) (system, user string) {
	var sys strings.Builder
	sys.WriteString("You are a tarot card reader. ")
	if cfg.Interpreter != "" {
		sys.WriteString(cfg.Interpreter)
		sys.WriteString("\n\n")
	}
	sys.WriteString("You are giving a three-card tarot reading (Past, Present, Future spread). ")
	sys.WriteString("Give a rich, intuitive reading that weaves the three cards into a cohesive narrative. ")
	if question != "" {
		sys.WriteString("The querent has asked a specific question — make sure your reading directly addresses it. ")
	}
	sys.WriteString("Keep the reading to about 3-4 paragraphs.")
	if cfg.Querent != "" {
		sys.WriteString("\n\nThe querent is: ")
		sys.WriteString(cfg.Querent)
	}

	var usr strings.Builder
	positions := []string{"Past", "Present", "Future"}
	usr.WriteString("The three cards drawn are:\n\n")
	for i, card := range cards {
		if i >= 3 {
			break
		}
		usr.WriteString(fmt.Sprintf("%d. %s (%s position)", i+1, card.Name, positions[i]))
		if card.IsMajor() {
			usr.WriteString(" [Major Arcana]")
		} else {
			usr.WriteString(fmt.Sprintf(" [%s]", card.SuitName()))
		}
		usr.WriteString(fmt.Sprintf("\n   Upright: %s\n   Reversed: %s\n\n", card.Upright, card.Reversed))
	}
	if question != "" {
		usr.WriteString("The querent's question is: ")
		usr.WriteString(question)
		usr.WriteString("\n\n")
	}
	usr.WriteString("Please give the reading now.")

	return sys.String(), usr.String()
}

func startReading(cfg Config, cards []Card, question string) <-chan streamMsg {
	ch := make(chan streamMsg, 64)

	go func() {
		defer close(ch)

		clientCfg := openai.DefaultConfig(cfg.APIKey)
		if cfg.BaseURL != "" {
			clientCfg.BaseURL = cfg.BaseURL
		}
		clientCfg.HTTPClient = &http.Client{
			Transport: &http.Transport{
				DialContext:         (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
				TLSHandshakeTimeout: 5 * time.Second,
			},
		}
		client := openai.NewClientWithConfig(clientCfg)

		systemPrompt, userPrompt := buildPrompt(cfg, cards, question)

		stream, err := client.CreateChatCompletionStream(
			context.Background(),
			openai.ChatCompletionRequest{
				Model: cfg.Model,
				Messages: []openai.ChatCompletionMessage{
					{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
					{Role: openai.ChatMessageRoleUser, Content: userPrompt},
				},
			},
		)
		if err != nil {
			ch <- readingErrMsg{Err: err}
			return
		}
		defer stream.Close()

		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				ch <- readingDoneMsg{}
				return
			}
			if err != nil {
				ch <- readingErrMsg{Err: err}
				return
			}
			if len(resp.Choices) > 0 {
				delta := resp.Choices[0].Delta.Content
				if delta != "" {
					ch <- readingChunkMsg{Text: delta}
				}
			}
		}
	}()

	return ch
}

// streamResult wraps a stream message with an epoch to detect stale messages.
type streamResult struct {
	epoch int
	msg   tea.Msg
}

func waitForStream(ch <-chan streamMsg, epoch int) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return streamResult{epoch, readingDoneMsg{}}
		}
		return streamResult{epoch, msg.(tea.Msg)}
	}
}
