// rendercard renders a PNG image in the terminal using pixterm's ansimage package.
// Usage: go run ./cmd/rendercard [image.png] [mode: classic|blocks|chars]
package main

import (
	"fmt"
	"image/color"
	"os"

	"github.com/eliukblau/pixterm/pkg/ansimage"
)

func main() {
	path := "fool.png"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	mode := ansimage.NoDithering
	modeName := "classic (half-block)"
	if len(os.Args) > 2 {
		switch os.Args[2] {
		case "blocks":
			mode = ansimage.DitheringWithBlocks
			modeName = "block dithering"
		case "chars":
			mode = ansimage.DitheringWithChars
			modeName = "character dithering"
		}
	}

	fmt.Fprintf(os.Stderr, "Rendering %s with %s mode...\n", path, modeName)

	// Render at 44 rows tall, width proportional to 512x768 source (2:3 ratio)
	// Half-block = 2 pixels per row, so 44 rows = 88 pixels tall
	// Width: 88 * (512/768) ≈ 59, but ansimage height must be even
	img, err := ansimage.NewScaledFromFile(
		path,
		44,                   // height in terminal rows
		30,                   // width in terminal cols (proportional)
		color.Black,          // background for transparency
		ansimage.ScaleModeFit,
		mode,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(img.Render())

	// Also render the other modes for comparison
	if len(os.Args) <= 2 {
		for _, m := range []struct {
			mode ansimage.DitheringMode
			name string
		}{
			{ansimage.DitheringWithBlocks, "Block dithering"},
			{ansimage.DitheringWithChars, "Character dithering"},
		} {
			img2, err := ansimage.NewScaledFromFile(
				path, 44, 30,
				color.Black,
				ansimage.ScaleModeFit,
				m.mode,
			)
			if err != nil {
				continue
			}
			fmt.Fprintf(os.Stderr, "\n--- %s ---\n", m.name)
			fmt.Println(img2.Render())
		}
	}
}
