package eclipse

import(
	"fmt"
	"image/color"
	"math"

	"github.com/abworrall/eclipse-hdr/pkg/ecolor"
	"github.com/abworrall/eclipse-hdr/pkg/emath"
)

// ImgDiff compares two images, and returns an error metric; the less
// similar, the higher the value. It figures out the difference in XYZ
// luminance for each pixel (after normalizing for EV differences),
// and returns the average per-lotsof-pixels difference across the set
// of compared pixels.
//
// If the pixel in either image is too dim or too bright on any channel, it is
// ignored, so we only really compare the subset of corona pixels that
// both images have a reasonable exposure for.
func ImgDiff(cfg Config, l1, l2 *Layer, passName string, xform AlignmentTransform) float64 {
	totErr   := 0.0
	nErr     := 0
	bounds   := cfg.InputArea

	tooLow  := uint32(0x0200)
	tooHigh := uint32(0x8000)

	diff     := emath.NewFloatGrid(bounds.Dx(), bounds.Dy())
	l2image  := xform.XFormImage(l2.LoadedImage)

	nPix, nLow, nHigh := 0,0,0

	for x:= bounds.Min.X; x<bounds.Max.X; x++ {
		for y:= bounds.Min.Y; y<bounds.Max.Y; y++ {
			c1 := l1.Image.At(x, y)
			c2 := l2image.At(x, y)
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
			evMax := l1.ExposureValue
			if l2.IlluminanceAtMaxExposure > evMax.IlluminanceAtMaxExposure {
				evMax = l2.ExposureValue
			}
			Y1 := col2Y(cfg, c1, l1.ExposureValue, evMax)
			Y2 := col2Y(cfg, c2, l2.ExposureValue, evMax)

			pixErr := math.Abs(Y1 - Y2)

			diff.Set(x-bounds.Min.X, y-bounds.Min.Y, pixErr)
			totErr += pixErr
			nErr++
		}
	}

	// Scale to a number that tends to be in the range 10,000 - 100,000
	errMetric := totErr * 10000000.0 / float64(nErr)

	if cfg.Verbosity > 0 {
		title := fmt.Sprintf("%s: %.1f%% comparable; err=% 7.0f; %s",
			passName, (100.0 * (float64(nErr) / float64(nPix))), errMetric, xform)
		diff.ToImg(title, fmt.Sprintf("diff-%s-%s.png", xform.Name, passName))
	}

	return errMetric
}

// Does a full DNG development pass on the pixel, to get into XYZ_D50
// color space; then returns the Y (luminance). Accounts for differing EVs.
func col2Y(cfg Config, c color.Color, ev, evMax ExposureValue) float64 {
	cn := ecolor.NewCameraNative(c, ev.IlluminanceAtMaxExposure)
	cn.AdjustIllumAtMax(evMax.IlluminanceAtMaxExposure)
	xyz := cn.ToPCS(cfg.CameraToPCS)

	return xyz.Y
}
