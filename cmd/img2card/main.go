package main

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	"golang.org/x/image/draw"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: img2card <image.png|jpg> [--frame]\n")
		fmt.Fprintf(os.Stderr, "\nConverts an image to hex RGB card format (24x40 inner art).\n")
		fmt.Fprintf(os.Stderr, "With --frame, outputs a 28x44 framed card with rounded border.\n")
		fmt.Fprintf(os.Stderr, "Output goes to stdout.\n")
		os.Exit(1)
	}

	imgPath := os.Args[1]
	framed := false
	for _, arg := range os.Args[2:] {
		if arg == "--frame" {
			framed = true
		}
	}

	f, err := os.Open(imgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening %s: %v\n", imgPath, err)
		os.Exit(1)
	}
	defer f.Close()

	src, _, err := image.Decode(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding image: %v\n", err)
		os.Exit(1)
	}

	// Resize to 24x40 using Catmull-Rom (high quality)
	dst := image.NewRGBA(image.Rect(0, 0, 24, 40))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	if framed {
		outputFramed(dst)
	} else {
		outputInner(dst)
	}
}

func outputInner(img *image.RGBA) {
	for y := 0; y < 40; y++ {
		var vals []string
		for x := 0; x < 24; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			vals = append(vals, fmt.Sprintf("%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8)))
		}
		fmt.Println(strings.Join(vals, ","))
	}
}

func outputFramed(img *image.RGBA) {
	// Border color: sample average of edge pixels, or use a moody purple default
	borderR, borderG, borderB := avgEdgeColor(img)
	bc := fmt.Sprintf("%02x%02x%02x", borderR, borderG, borderB)
	transparent := "000000"

	for y := 0; y < 44; y++ {
		var vals []string
		for x := 0; x < 28; x++ {
			switch {
			// Row 0 & 43: corners are transparent
			case (y == 0 || y == 43) && (x < 2 || x >= 26):
				vals = append(vals, transparent)
			// Row 0 & 43: border fill
			case y == 0 || y == 43:
				vals = append(vals, bc)
			// Row 1 & 42: full border
			case y == 1 || y == 42:
				vals = append(vals, bc)
			// Inner rows: border | padding | 24px inner | padding | border
			case x == 0 || x == 27:
				vals = append(vals, bc)
			case x == 1 || x == 26:
				vals = append(vals, bc)
			default:
				// Inner pixel at (x-2, y-2)
				ix, iy := x-2, y-2
				r, g, b, _ := img.At(ix, iy).RGBA()
				vals = append(vals, fmt.Sprintf("%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8)))
			}
		}
		fmt.Println(strings.Join(vals, ","))
	}
}

func avgEdgeColor(img *image.RGBA) (uint8, uint8, uint8) {
	var rSum, gSum, bSum, count uint64
	addPixel := func(c color.Color) {
		r, g, b, _ := c.RGBA()
		rSum += uint64(r >> 8)
		gSum += uint64(g >> 8)
		bSum += uint64(b >> 8)
		count++
	}
	// Top and bottom rows
	for x := 0; x < 24; x++ {
		addPixel(img.At(x, 0))
		addPixel(img.At(x, 39))
	}
	// Left and right columns
	for y := 0; y < 40; y++ {
		addPixel(img.At(0, y))
		addPixel(img.At(23, y))
	}
	if count == 0 {
		return 139, 47, 201 // fallback purple
	}
	return uint8(rSum / count), uint8(gSum / count), uint8(bSum / count)
}
