package estack

import(
	"image/color"
)

// ColorTweakByLayer messes with the RGB channels, coloring the pixel
// depending on which source image was used.
func ColorTweakByLayer(ws *PixelWorkspace) {
	r, g, b, alpha := ws.Output.RGBA()

	switch ws.LayerMostUsed {
	case 0: g=0; b=0
	case 1: r=0; b=0
	case 2: r=0; g=0
	case 3: b=0
	case 4: g=0
	case 5: r=0
	case 6:
	}

	ws.Output = color.RGBA64{uint16(r), uint16(g), uint16(b), uint16(alpha)}
}
