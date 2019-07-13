package estack

import(
	"image/color"
	"math"
)

// A Luminance is a value representing the physical luminance on the image sensor, across
// a very wide range of values (e.g. kinda HDR)
type Luminance struct {
	R,G,B float64
}

// Given the exposure info, map the color into a physical luminance. The bool indicates
//  whether the color value was clipped (i.e. whiteout)
func (ev ExposureValue)Col2Lum(c color.Color) (Luminance, bool) {
	r,g,b,_ := c.RGBA()

	if r > 0xf000 || g > 0xf000 || b > 0xf000 {
		// If a value has clipped, skip it, since it isn't an accurate physical measure any more
		return Luminance{}, true
//	} else if r < 0x000f || g < 0x000f || b < 0x000f {
//		return Luminance{}, true
	}

	return Luminance{
		R: float64(r) / float64(0xFFFF) * float64(ev.MaxLuminance),
		G: float64(g) / float64(0xFFFF) * float64(ev.MaxLuminance),
		B: float64(b) / float64(0xFFFF) * float64(ev.MaxLuminance),
	}, false
}

// A simple linear mapping from [0.0, maxCandles] -> [0x0000, 0xFFFF]
func (l Luminance)TonemapLinear(maxCandles float64) (uint16,uint16,uint16) {
	f := func(lum float64) uint16 {
		if lum > maxCandles { lum = maxCandles } // Clip
		return uint16(lum * 0xFFFF / maxCandles)
	}

	return f(l.R), f(l.G), f(l.B)
}

// Gamma: lum' = A.lum^gamma
func (l Luminance)TonemapGamma(gamma, maxCandles float64) (uint16,uint16,uint16) {
	f := func(lum float64) uint16 {
		lum /= maxCandles             // maps to [0,1]
		if lum > 1.0 { lum = 1.0 }    // clip
		lum = math.Pow(lum, gamma)    // V^gamma
		return uint16(lum * 0xFFFF)
	}

	return f(l.R), f(l.G), f(l.B)
}

// This is pretty sketchy. Given the distance of the pixel from the
// center of the sun (as a multiple of solar radii, R; e.g. a value of
// 1.0 means right on the circumference of the sun), apply the
// (hyper?)exponential scaling function: L' = L^(R^x)
func (l *Luminance)RadialScale(rad float64, x float64) {
	if rad < 1.0 { return }
	
	scale := math.Pow(rad, x)
	
	l.R = math.Pow(l.R, scale)
	l.G = math.Pow(l.G, scale)
	l.B = math.Pow(l.B, scale)
}

func (l *Luminance)RadialBullseye(rad float64) {
	if rad < 1.0 { return }

	if rad < 1.5 {
		l.R *= 4.0
	} else if rad < 2.0 {
		l.G *= 4.0
	} else if rad < 2.5 {
		l.B *= 4.0
	}
}
