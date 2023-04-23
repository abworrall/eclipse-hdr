package estack

import(
	"fmt"
	"image/color"
	"math"
)

// ImgDiff compares two images, and returns an error metric; the less
// similar, the higher the value. It figures out the difference in
// XYZ luminance for each pixel in s1, and returns the average
// per-lotsof-pixels difference across the set of compared pixels.
//
// If the pixel in either image is too dim or too bright, it is
// ignored, so we only really compare the subset of corona pixels that
// both images have a reasonable exposure for.
//
// This is used for alignment finetuning, to see if tweaking the
// transform a few pixels in each direction gives a better match for
// those pixels. It works fairly well, given how dumb it is.
func ImgDiff(s *Stack, s1, s2 *StackedImage, passName string, xform AlignmentTransform) float64 {
	totErr   := 0.0
	nErr     := 0
	bounds   := s.InputArea

	tooLow  := uint32(0x0200) // 0x1000
	tooHigh := uint32(0x8000) // 0x8000

	diff     := NewFloatGrid(bounds.Dx(), bounds.Dy())
	s2ximage := xform.XFormImage(s2.OrigImage)

	nPix, nLow, nHigh := 0,0,0
	
	for x:= bounds.Min.X; x<bounds.Max.X; x++ {
		for y:= bounds.Min.Y; y<bounds.Max.Y; y++ {
			c1 := s1.XImage.At(x, y)
			c2 := s2ximage.At(x, y)
			r1, g1, b1,_ := c1.RGBA()
			r2, g2, b2,_ := c2.RGBA()

			nPix++
			if r1 < tooLow || g1 < tooLow || b1 < tooLow || r2 < tooLow || g2 < tooLow || b2 < tooLow {
				nLow++
				continue
			} else if r1 > tooHigh || g1 > tooHigh || b1 > tooHigh || r2 > tooHigh || g2 > tooHigh || b2 > tooHigh {
				nHigh++
				continue
			}

			// This is the illuminance at max over the two images (that have diff exposures)
			evMax := s1.ExposureValue
			if s2.IlluminanceAtMaxExposure > evMax.IlluminanceAtMaxExposure {
				evMax = s2.ExposureValue
			}
			Y1 := col2Y(s, c1, s1.ExposureValue, evMax)
			Y2 := col2Y(s, c2, s2.ExposureValue, evMax)

			pixErr := math.Abs(Y1 - Y2)

			diff.Set(x-bounds.Min.X, y-bounds.Min.Y, pixErr)
			totErr += pixErr
			nErr++
		}
	}

	// Scale to a number that tends to be in the range 10,000 - 100,000
	errMetric := totErr * 10000000.0 / float64(nErr)

	title := fmt.Sprintf("%s: %.1f%% comparable; err=% 7.0f; %s",
		passName, (100.0 * (float64(nErr) / float64(nPix))), errMetric, xform)
	diff.ToImg(title, fmt.Sprintf("diff-%s-%s.png", xform.Name, passName))

	return errMetric
}

// Returns the Y luminance value, after white-balancing the RGB input,
// scaling exposure across both images, and mapping into XYZ_D50. This
// dupes an annoyingly large amount of logic from pixelworkspace.go &
// generate.go
func col2Y(s *Stack, c color.Color, ev, evMax ExposureValue) float64 {
	r, g, b, _ := c.RGBA()

	// White balance
	wbRGB := MyVec3{
		float64(r) / float64(0xFFFF) * (1.0 / s.Configuration.AsShotNeutral[0]),
		float64(g) / float64(0xFFFF) * (1.0 / s.Configuration.AsShotNeutral[1]),
		float64(b) / float64(0xFFFF) * (1.0 / s.Configuration.AsShotNeutral[2]),
	}

	// Linear tonemap (allows pixels from diff exposures to be compared)
	fusedRGB := MyVec3{
		wbRGB[0] * ev.IlluminanceAtMaxExposure / evMax.IlluminanceAtMaxExposure,
		wbRGB[1] * ev.IlluminanceAtMaxExposure / evMax.IlluminanceAtMaxExposure,
		wbRGB[2] * ev.IlluminanceAtMaxExposure / evMax.IlluminanceAtMaxExposure,
	}

	// Use camera's matrix to convert from camera native RGB into XYZ(D50)
	XYZ := s.Configuration.ForwardMatrix.Apply(fusedRGB)
	XYZ.FloorAt(0.0) // See some tiny -ve values here in very dark noise, which rollover into bright R/G/B pixels

	// Now just take the Y value
	return XYZ[1]
}
