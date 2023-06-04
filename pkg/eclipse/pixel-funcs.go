package eclipse

import(
	"math"

	"github.com/mdouchement/hdr/hdrcolor"

	"github.com/abworrall/eclipse-hdr/pkg/ecolor"
)

// A PixelFunc mutates a pixel. There are two families of these functions:
// - FuseBy: examine LDR pixels from all layers to generate a single HDR pixel
// - DevelopBy: perform color correction to the HDR pixel prior to tonemapping
type PixelFunc func(Config, *Pixel)

// FuseByPickMostExposed is the default algorithm for image fusion:
// look for the image that is most-exposed (i.e. has received the most
// photons and will thus have lowest noise), but not over-exposed at
// this pixel (e.g. no channel more than ~80%).
func FuseByPickMostExposed(cfg Config, p *Pixel) {
	// pixel is too exposed if luminosity greater than this.
	// Good values in range [0.6, 0.8].
	maxY := cfg.FuserLuminance

	// The images are pre-sorted in asc EV with the largest exposures
	// (most photons, least noise) first, so stop as soon as we can
	for i:=0; i<len(p.In); i++ {
		// If this looks too exposed, and we can move on to another layer, move on.
		if i < len(p.In)-1 {
			_, Y, _, _ := p.In[i].HDRXYZA()
			if Y > maxY {
				continue
			}
		}

		p.LayerNumber = i
		p.Fused = p.In[i]

		return
	}
}

// FuseBySector cuts up the image into pie slices, and simply picks
// a source layer based on which pie segment the pixel lies inside.
// It's useful for comparing the source images to see how well
// they've been aligned.
func FuseBySector(cfg Config, p *Pixel) {
	center              := RectCenter(cfg.OutputArea)
	pos                 := p.OutputPos
	thetaRadians        := math.Atan2(float64(pos.Y-center.Y), float64(pos.X-center.X))
	thetaDegrees        := 180 + thetaRadians * 180.0 / math.Pi
	numSegmentsPerLayer := 5
	numSegments         := len(p.In) * numSegmentsPerLayer
	segmentWidth        := 360.0 / float64(numSegments)
	thisSegment         := int(thetaDegrees / segmentWidth)

	p.LayerNumber = thisSegment % len(p.In)
	p.Fused = p.In[p.LayerNumber]
}

// FuseByAverage averages the non-overexposed layers together. It produces poor results, with notable
// color fringes forming near the boundary of each layer's area.
func FuseByAverage(cfg Config, p *Pixel) {
	max := 0.8 // pixel is too exposed if any channel recorded more than this (range [0.0, 1.0])

	toAvg := []ecolor.CameraNative{}

	// The images are pre-sorted in asc EV; slowest exposures first, most likely to over-expose.
	for i:=0; i<len(p.In); i++ {
		// If this looks too exposed, and we have less-exposed layers left, move on.
		if i < len(p.In)-1 {
			r, g, b, _ := p.In[i].HDRRGBA()
			if r > max || g > max || b > max {
				continue
			}
		}

		toAvg = append(toAvg, p.In[i])
	}

	p.Fused = ecolor.AverageBalancedCameraNativeRGBs(toAvg)
	p.LayerNumber = len(toAvg)
}

// DevelopDNG follows the DNG spec's algorithm for mapping a
// CameraNative sensor reading into a camera-neutral XYZ(D50) color,
// and then into a standard sRGB(D65) output color. This requires
// data from the camera, that is written into the DNG files
// - AsShotNeutral (the white balance correction)
// - ForwardMatrix (the camera's color correction matrix)
func DevelopByDNG(cfg Config, p *Pixel) {
	
	xyzD50 := p.Fused.ToPCS(cfg.CameraToPCS)
	sRgb   := ecolor.XYZToSRGB(xyzD50)

	// In eclipse shots, there are lots of near-black pixels. The above
	// transforms leave those pixels with slightly -ve values, which
	// underflow into really bright pixels, so we clip them.
	// This is one of a few places in the pipeline where clipping happens.
	sRgb = ecolor.HDRRGBFloorAt(sRgb, 0.0)

	// [If we were developing for final output, we would gamma expand to get final sRGB]

	p.DevelopedRGB = sRgb
}

func DevelopByWhiteBalanceOnly(cfg Config, p *Pixel) {
	wbRgb  := ecolor.ApplyCameraWhite(p.Fused, cfg.CameraWhite)
	p.DevelopedRGB = wbRgb
}

func DevelopByNone(cfg Config, p *Pixel) {
	p.DevelopedRGB = p.Fused.RGB
}

// DevelopAsLayer is for debugging - it colors the pixel based on
// which layer it came from. (White balances it too)
func DevelopByLayer(cfg Config, p *Pixel) {
	wbRgb  := ecolor.ApplyCameraWhite(p.Fused, cfg.CameraWhite)
	r, g, b, _ := wbRgb.HDRRGBA()

	switch p.LayerNumber {
	case 0: g=0; b=0
	case 1: r=0; b=0
	case 2: r=0; g=0
	case 3: b=0
	case 4: g=0
	case 5: r=0
	case 6:
	}

	p.DevelopedRGB = hdrcolor.RGB{r, g, b}
}
