// viewhex renders a .hex card file in the terminal.
// Usage: go run ./cmd/viewhex <file.hex>
package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

type RGB3 = [3]uint8

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: viewhex <file.hex>")
		os.Exit(1)
	}

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	pixels := make([][]RGB3, len(lines))
	for y, line := range lines {
		cols := strings.Split(strings.TrimSpace(line), ",")
		pixels[y] = make([]RGB3, len(cols))
		for x, h := range cols {
			h = strings.TrimSpace(h)
			b, _ := hex.DecodeString(h)
			if len(b) >= 3 {
				pixels[y][x] = RGB3{b[0], b[1], b[2]}
			}
		}
	}

	// Add a simple 2px border frame
	innerH := len(pixels)
	innerW := len(pixels[0])
	frameH := innerH + 4
	frameW := innerW + 4
	bc := RGB3{140, 110, 40} // gold border

	frame := make([][]RGB3, frameH)
	for y := range frame {
		frame[y] = make([]RGB3, frameW)
		for x := range frame[y] {
			frame[y][x] = bc
		}
	}
	// Rounded corners
	for _, p := range [][2]int{{0, 0}, {0, 1}, {0, frameW - 2}, {0, frameW - 1},
		{frameH - 1, 0}, {frameH - 1, 1}, {frameH - 1, frameW - 2}, {frameH - 1, frameW - 1}} {
		frame[p[0]][p[1]] = RGB3{0, 0, 0}
	}
	for y := 0; y < innerH; y++ {
		for x := 0; x < innerW; x++ {
			frame[y+2][x+2] = pixels[y][x]
		}
	}

	// Render half-block
	for row := 0; row < frameH-1; row += 2 {
		for col := 0; col < frameW; col++ {
			top := frame[row][col]
			bot := frame[row+1][col]
			topT := top[0] == 0 && top[1] == 0 && top[2] == 0
			botT := bot[0] == 0 && bot[1] == 0 && bot[2] == 0
			if topT && botT {
				fmt.Print(" ")
			} else if topT {
				fmt.Printf("\033[38;2;%d;%d;%dm▄\033[0m", bot[0], bot[1], bot[2])
			} else if botT {
				fmt.Printf("\033[38;2;%d;%d;%dm▀\033[0m", top[0], top[1], top[2])
			} else {
				fmt.Printf("\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm▀\033[0m",
					top[0], top[1], top[2], bot[0], bot[1], bot[2])
			}
		}
		fmt.Println()
	}
}
