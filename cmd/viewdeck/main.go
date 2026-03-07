// viewdeck renders all .hex files in a directory to stdout.
// Usage: go run ./cmd/viewdeck decks/diffusion-sm/major/
package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type RGB3 = [3]uint8

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: viewdeck <dir> [dir...]")
		os.Exit(1)
	}

	for _, dir := range os.Args[1:] {
		entries, err := os.ReadDir(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", dir, err)
			continue
		}

		var files []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".hex") {
				files = append(files, e.Name())
			}
		}
		sort.Strings(files)

		for _, name := range files {
			renderCard(filepath.Join(dir, name), name)
			fmt.Println()
		}
	}
}

func renderCard(path, filename string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	pixels := make([][]RGB3, len(lines))
	for y, line := range lines {
		cols := strings.Split(strings.TrimSpace(line), ",")
		pixels[y] = make([]RGB3, len(cols))
		for x, h := range cols {
			b, _ := hex.DecodeString(strings.TrimSpace(h))
			if len(b) >= 3 {
				pixels[y][x] = RGB3{b[0], b[1], b[2]}
			}
		}
	}

	// Frame
	bc := RGB3{140, 110, 40}
	innerH := len(pixels)
	innerW := len(pixels[0])
	fH := innerH + 4
	fW := innerW + 4

	frame := make([][]RGB3, fH)
	for y := range frame {
		frame[y] = make([]RGB3, fW)
		for x := range frame[y] {
			frame[y][x] = bc
		}
	}
	for _, p := range [][2]int{{0, 0}, {0, 1}, {0, fW - 2}, {0, fW - 1},
		{fH - 1, 0}, {fH - 1, 1}, {fH - 1, fW - 2}, {fH - 1, fW - 1}} {
		frame[p[0]][p[1]] = RGB3{}
	}
	for y := 0; y < innerH; y++ {
		for x := 0; x < innerW; x++ {
			frame[y+2][x+2] = pixels[y][x]
		}
	}

	// Label from filename
	label := strings.TrimSuffix(filename, ".hex")
	// Strip leading number prefix like "08-"
	if i := strings.Index(label, "-"); i >= 0 && i <= 3 {
		label = label[i+1:]
	}
	label = strings.ReplaceAll(label, "-", " ")
	label = strings.Title(label)

	// Render half-block
	for row := 0; row < fH-2; row += 2 {
		for col := 0; col < fW; col++ {
			top := frame[row][col]
			bot := frame[row+1][col]
			topT := top == RGB3{}
			botT := bot == RGB3{}
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

	// Label row
	innerLW := fW - 4
	if len(label) > innerLW {
		label = label[:innerLW]
	}
	pad := innerLW - len(label)
	left := pad / 2
	right := pad - left
	centered := strings.Repeat(" ", left) + label + strings.Repeat(" ", right)
	corner := fmt.Sprintf("\033[38;2;%d;%d;%dm▀\033[0m", bc[0], bc[1], bc[2])
	fmt.Print(corner, corner)
	fmt.Printf("\033[1m\033[38;2;0;0;0m\033[48;2;%d;%d;%dm%s\033[0m", bc[0], bc[1], bc[2], centered)
	fmt.Print(corner, corner)
	fmt.Println()
}
