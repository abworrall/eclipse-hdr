package estack

import "log"

// A GlobalTonemapper operates over `s.Pixels`, populating `TonemappedRGB` for each `s.Pixel`.
type GlobalTonemapper func(s *Stack)

func TonemapFattal02(s *Stack) {
	f02 := Fattal02{
		// The PFSTMO Parameters - see https://www.mankier.com/1/pfstmo_fattal02
		// These are the default values when using the FFT solver
		DetailLevel: 3,
		Noise:       0.002,
		Alpha:       1.0,
		Beta:        0.9,
		Gamma:       0.8,
		BlackPoint:  0.1,
		WhitePoint:  0.5,
		Saturation:  0.8,
	}

	// We override some of the default parameters
	f02.WhitePoint = 0.0 // We don't want _any_ overexposed pixels	
	f02.Gamma      = 1.0 // Don't gamma expand during tonemapping; we do it later when we output to sRGB
	f02.Saturation = 0.4 // Try and prevent solar prominences blowing out the red channel

	// If DetailLevel<3, the attentuation grid starts to see the
	// near-black noise as low-gradient features to retain and amplify,
	// which means the disc of the moon becomes full of bright colorful
	// noise
	f02.DetailLevel = 4

	log.Printf("ToneMapping: fattal02 HDR\n")

	f02.Run(s)
}

func runLocalTonemapper(s *Stack, tmo PixelFunc) {
	for x:=0; x<len(s.Pixels); x++ {
		for y:=0; y<len(s.Pixels[x]); y++ {
			tmo(s.Pixels[x][y])
		}
	}
}

func TonemapLinear(s *Stack) {
	log.Printf("ToneMapping: linear tonemap\n")
	runLocalTonemapper(s, func(ws *PixelWorkspace){
		ws.TonemappedRGB = ws.FusedRGB
	})
}
