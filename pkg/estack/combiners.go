package estack

import(
	"image"
	"image/color"
	"math"

	"github.com/skypies/util/histogram"
)

// The output color should have 16 bits per color channel
type CombinerFunc func(Stack, int,int, []ExposureValue, []color.Color) (color.Color, error)

var(
	Hists = []histogram.Histogram{
		histogram.Histogram{NumBuckets:256, ValMin:0, ValMax:256},
		histogram.Histogram{NumBuckets:256, ValMin:0, ValMax:256},
		histogram.Histogram{NumBuckets:256, ValMin:0, ValMax:256},
		histogram.Histogram{NumBuckets:256, ValMin:0, ValMax:256},
		histogram.Histogram{NumBuckets:256, ValMin:0, ValMax:256},
	}
)

// {{{ s.CombineImagesAt

func (s Stack)CombineImagesAt(x,y int) color.Color {
	evs := []ExposureValue{}
	colors := []color.Color{}
	
	for _,si := range s.Images {
		evs = append(evs, si.ExposureValue)
		colors = append(colors, si.At(x - si.Offset.X, y - si.Offset.Y))
	}

	out,_ := s.Rendering.Combiner(s, x, y, evs, colors)
	return out
}

// }}}

// {{{ MergeAverage

func MergeAverage(s Stack, x,y int, evs []ExposureValue, colors []color.Color) (color.Color, error) {
	var r,g,b int64
	
	for _,c := range colors {
		rr,gg,bb,_ := c.RGBA()
		r += int64(rr)
		g += int64(gg)
		b += int64(bb)
	}
	
	r /= int64(len(colors))
	g /= int64(len(colors))
	b /= int64(len(colors))

	out := color.RGBA64{R:uint16(r), G:uint16(g), B:uint16(b), A:0xffff} // 16 bits per color channel

	// Build a histogram per image.
	// 
	for i,c := range colors {
		if lum,clipped := evs[i].Col2Lum(c); !clipped {
			avgLog2 := math.Log2(float64(lum.R + lum.G + lum.B) / 3.0)
			if avgLog2 > 0.2 {
				Hists[i].Add(histogram.ScalarVal(int(avgLog2 * 25.6)))
			}
		}
	}
	
	return out, nil
}

// }}}
// {{{ MergeDistinct

func MergeDistinct(s Stack, x,y int, evs []ExposureValue, colors []color.Color) (color.Color, error) {
	var r,g,b uint32
	r,_,_,_ = colors[0].RGBA()
	_,g,_,_ = colors[1].RGBA()

	if len(colors) > 2 {
		_,_,b,_ = colors[2].RGBA()
	}

	/*
	if r > 0x23ff { r = 0xffff }
	if g > 0x23ff { g = 0xffff }
	if b > 0x23ff { b = 0xffff }
*/
	
	return color.RGBA64{
		R:uint16(r),
		G:uint16(g),
		B:uint16(b),
		A:0xffff,
	}, nil
}

// }}}
// {{{ MergeHDR

func MergeHDR(s Stack, x,y int, evs []ExposureValue, colors []color.Color) (color.Color, error) {
	lums := []Luminance{}

	for i,c := range colors {
		if lum,clipped := evs[i].Col2Lum(c); clipped {
			// If a value has clipped, skip it, since it isn't an accurate physical measure any more
			continue
		} else {
			lums = append(lums, lum)
		}
	}

	// Average out the luminances
	avgLum := Luminance{}
	if len(lums) > 0 {
		for _,lum := range lums {
			avgLum.R += lum.R
			avgLum.G += lum.G
			avgLum.B += lum.B
		}
		avgLum.R /= float64(len(lums))
		avgLum.G /= float64(len(lums))
		avgLum.B /= float64(len(lums))
	}
	
	// Now ... how to tone map the output down to [0x0000,0xFFFF] per channel ?
	r,g,b := avgLum.TonemapGamma(s.Rendering.Gamma, s.Rendering.MaxCandles)

	return color.RGBA64{R:r, G:g, B:b, A:0xffff}, nil
}

// }}}
// {{{ MergeQuadrantsLuminance

func MergeQuadrantsLuminance(s Stack, x,y int, evs []ExposureValue, colors []color.Color) (color.Color, error) {
	center := image.Point{
		X: s.Rendering.Bounds.Min.X + s.Rendering.Bounds.Dx() / 2,
		Y: s.Rendering.Bounds.Min.Y + s.Rendering.Bounds.Dy() / 2,
	}

	fromCenter := image.Point{x-center.X, y-center.Y}
	quadrant := 0 // we want 0 to be NW, going clockwise. Recall that Y axis is inverted.
	if fromCenter.X < 0 {
		if fromCenter.Y < 0 {
			quadrant = 0
		} else {
			quadrant = 3
		}
	} else {
		if fromCenter.Y < 0 {
			quadrant = 1
		} else {
			quadrant = 2
		}
	}

	if quadrant >= len(colors) { quadrant = len(colors) - 1 }

	var r,g,b uint16
	if lum,clipped := evs[quadrant].Col2Lum(colors[quadrant]); !clipped {
		r,g,b = lum.TonemapGamma(s.Rendering.Gamma, s.Rendering.MaxCandles)
	}
/*
	if quadrant == 0 {
		g,b = 0,0
	} else if quadrant == 1 {
		r,b = 0,0
	} else if quadrant == 2 {
		r,g = 0,0
	}
*/	
	out := color.RGBA64{R:uint16(r), G:uint16(g), B:uint16(b), A:0xffff}
	return out, nil
}

// }}}
// {{{ MergeBestExposed

// Pick the image that is best exposed, and ignore the others.
func MergeBestExposed(s Stack, x,y int, evs []ExposureValue, colors []color.Color) (color.Color, error) {
	
	// We go from the most exposed (most light, least candles to expose, lowest EV value) downwards,
	// and pick the first image that is not over exposed
	// TODO: actually sort on this value, instead of assuming they're in order
	for i := len(s.Images)-1; i>=0; i-- {
		r,g,b,_ := colors[i].RGBA()
		if r > 0xf000 || g > 0xf000 || b > 0xf000 {
			continue
		}

		lum,_ := evs[i].Col2Lum(colors[i])

		if s.Rendering.RadialExponent > 0.0 {
			// Now, the hack; apply a scaling function, based on distance from center (in pixels) ...
			center := image.Point{
				X: s.Rendering.Bounds.Min.X + s.Rendering.Bounds.Dx() / 2,
				Y: s.Rendering.Bounds.Min.Y + s.Rendering.Bounds.Dy() / 2,
			}
			fromCenter := image.Point{x-center.X, y-center.Y}
			distPix := math.Sqrt(float64(fromCenter.X*fromCenter.X + fromCenter.Y*fromCenter.Y))
			distRad := distPix / float64(s.Rendering.SolarRadiusPixels)
			lum.RadialScale(distRad, s.Rendering.RadialExponent)
		}

		rr,gg,bb := lum.TonemapGamma(s.Rendering.Gamma, s.Rendering.MaxCandles)
		return color.RGBA64{R:rr, G:gg, B:bb, A:0xffff}, nil
	}

	return color.RGBA64{}, nil
}

// }}}
// {{{ MergeBullseye

// Dervied from bestexposed, but paints a bulleye
func MergeBullseye(s Stack, x,y int, evs []ExposureValue, colors []color.Color) (color.Color, error) {	
	// We go from the most exposed (most light, least candles to expose, lowest EV value) downwards,
	// and pick the first image that is not over exposed
	for i := len(s.Images)-1; i>=0; i-- {
		r,g,b,_ := colors[i].RGBA()
		if r > 0xf000 || g > 0xf000 || b > 0xf000 {
			continue
		}

		lum,_ := evs[i].Col2Lum(colors[i])

		// Now, the hack; apply a scaling function, based on distance from center (in pixels) ...
		center := image.Point{
			X: s.Rendering.Bounds.Min.X + s.Rendering.Bounds.Dx() / 2,
			Y: s.Rendering.Bounds.Min.Y + s.Rendering.Bounds.Dy() / 2,
		}
		fromCenter := image.Point{x-center.X, y-center.Y}
		distPix := math.Sqrt(float64(fromCenter.X*fromCenter.X + fromCenter.Y*fromCenter.Y))
		distRad := distPix / float64(s.Rendering.SolarRadiusPixels)
		lum.RadialBullseye(distRad)

		rr,gg,bb := lum.TonemapGamma(s.Rendering.Gamma, s.Rendering.MaxCandles)
		return color.RGBA64{R:rr, G:gg, B:bb, A:0xffff}, nil
	}

	return color.RGBA64{}, nil
}

// }}}

// {{{ SelectFromOneLayer

// This is for slideshow debugging; it does everything like HDR, but only takes a pixel
//  from the layer specified in the render options.
func SelectFromOneLayer(s Stack, x,y int, evs []ExposureValue, colors []color.Color) (color.Color, error) {
	layer := s.Rendering.SelectJustThisLayer
	lum,_ := evs[layer].Col2Lum(colors[layer])
	r,g,b := lum.TonemapGamma(s.Rendering.Gamma, s.Rendering.MaxCandles)
	return color.RGBA64{R:r, G:g, B:b, A:0xffff}, nil
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
